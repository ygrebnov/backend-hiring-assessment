package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newCreateAdServer(fn usecase.CreateAdFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads", middleware.RequireAuth(handlerCreateAd(fn)))
	return httptest.NewServer(mux)
}

func postJSON(t *testing.T, url string, body any, authHeader string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestHandlerCreateAd(t *testing.T) {
	validBody := createAdBody{
		Title:      "mountain bike",
		PriceCents: ptr(int64(15000)),
		PhotoURL:   "https://example.com/bike.jpg",
	}

	t.Run("returns 201 with ID on success", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			return domain.AdID(7), nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got createAdResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, int64(7), got.ID)
	})

	t.Run("passes authenticated user ID as owner to the usecase", func(t *testing.T) {
		var received usecase.CreateAdRequest
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			received = req
			return domain.AdID(1), nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-42")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, domain.UserID("user-42"), received.OwnerID)
		assert.Equal(t, "mountain bike", received.Title)
		assert.Equal(t, int64(15000), received.PriceCents)
		assert.Equal(t, "https://example.com/bike.jpg", received.PhotoURL)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 when body is invalid JSON", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads", bytes.NewBufferString("not json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, errors.New("db down")
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("accepts new price format", func(t *testing.T) {
		var received usecase.CreateAdRequest

		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			received = req
			return domain.AdID(1), nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", map[string]any{
			"title":     "mountain bike",
			"price":     150.50,
			"photo_url": "https://example.com/bike.jpg",
		}, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, int64(15050), received.PriceCents)
	})

	t.Run("returns 400 when both price formats are provided", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", map[string]any{
			"title":       "mountain bike",
			"price":       150.50,
			"price_cents": 15050,
			"photo_url":   "https://example.com/bike.jpg",
		}, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 429 when publication limit is exceeded", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, adsdao.ErrPublicationLimitExceeded
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	})

	t.Run("returns 429 when publication limit error is wrapped", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, fmt.Errorf("unable to create ad: %w", adsdao.ErrPublicationLimitExceeded)
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	})
}

func TestCreateAdBodyPriceCents(t *testing.T) {
	tests := []struct {
		name    string
		body    createAdBody
		want    int64
		wantErr bool
	}{
		{
			name: "new price format",
			body: createAdBody{
				Price: ptr(json.Number("150.50")),
			},
			want: 15050,
		},
		{
			name: "legacy price format",
			body: createAdBody{
				PriceCents: ptr(int64(15050)),
			},
			want: 15050,
		},
		{
			name: "zero price",
			body: createAdBody{
				Price: ptr(json.Number("0")),
			},
			want: 0,
		},
		{
			name: "zero price in legacy format",
			body: createAdBody{
				PriceCents: ptr(int64(0)),
			},
			want: 0,
		},
		{
			name:    "missing price",
			body:    createAdBody{},
			wantErr: true,
		},
		{
			name: "both formats provided",
			body: createAdBody{
				Price:      ptr(json.Number("150.50")),
				PriceCents: ptr(int64(15050)),
			},
			wantErr: true,
		},
		{
			name: "negative legacy price",
			body: createAdBody{
				PriceCents: ptr(int64(-100)),
			},
			wantErr: true,
		},
		{
			name: "invalid new price",
			body: createAdBody{
				Price: ptr(json.Number("150.555")),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.body.priceCents()

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
