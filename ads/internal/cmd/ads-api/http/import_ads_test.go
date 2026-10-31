package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(fn)))
	return httptest.NewServer(mux)
}

func TestHandlerImportAds(t *testing.T) {
	t.Run("returns 201 with all generated IDs on success", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++

			require.Len(t, req.Ads, 3)

			return []domain.AdID{10, 11, 12}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: ptr(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
			{
				Title:      "scooter",
				PriceCents: ptr(int64(8000)),
				PhotoURL:   "https://example.com/scooter.jpg",
			},
			{
				Title:      "car",
				PriceCents: ptr(int64(200000)),
				PhotoURL:   "https://example.com/car.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

		assert.Equal(t, []int64{10, 11, 12}, got.IDs)
		assert.Equal(t, 1, calls, "bulk import must invoke the use case exactly once")
	})

	t.Run("passes authenticated user ID as owner for every ad", func(t *testing.T) {
		var received usecase.ImportAdsRequest

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{1, 2}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: ptr(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
			{
				Title:      "scooter",
				PriceCents: ptr(int64(8000)),
				PhotoURL:   "https://example.com/scooter.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-42")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 2)

		assert.Equal(t, domain.UserID("user-42"), received.OwnerID)
		assert.Equal(t, domain.UserID("user-42"), received.Ads[0].OwnerID)
		assert.Equal(t, domain.UserID("user-42"), received.Ads[1].OwnerID)

		assert.Equal(t, "bike", received.Ads[0].Title)
		assert.Equal(t, "scooter", received.Ads[1].Title)
	})

	t.Run("forwards idempotency key", func(t *testing.T) {
		var received usecase.ImportAdsRequest

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{42}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: ptr(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
		}}

		payload, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest(
			http.MethodPost,
			srv.URL+"/ads/import",
			bytes.NewReader(payload),
		)
		require.NoError(t, err)

		req.Header.Set("Authorization", "user-1")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "import-123")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "import-123", received.IdempotencyKey)
	})

	t.Run("returns 201 with empty IDs when body has no ads", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Empty(t, req.Ads)

			return []domain.AdID{}, nil
		})
		defer srv.Close()

		resp := postJSON(
			t,
			srv.URL+"/ads/import",
			importAdsBody{Ads: []createAdBody{}},
			"user-1",
		)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

		assert.Empty(t, got.IDs)
		assert.NotNil(t, got.IDs)
		assert.Equal(t, 1, calls)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{}, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		defer srv.Close()

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads/import", nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 500 when import fails", func(t *testing.T) {
		calls := 0
		dbErr := errors.New("db down")

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Len(t, req.Ads, 3)

			return nil, dbErr
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: ptr(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
			{
				Title:      "scooter",
				PriceCents: ptr(int64(8000)),
				PhotoURL:   "https://example.com/scooter.jpg",
			},
			{
				Title:      "car",
				PriceCents: ptr(int64(200000)),
				PhotoURL:   "https://example.com/car.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, 1, calls)
	})

	t.Run("returns 409 on idempotency conflict", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Equal(t, "import-123", req.IdempotencyKey)

			return nil, adsdao.ErrIdempotencyConflict
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: ptr(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
		}}

		payload, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest(
			http.MethodPost,
			srv.URL+"/ads/import",
			bytes.NewReader(payload),
		)
		require.NoError(t, err)

		req.Header.Set("Authorization", "user-1")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "import-123")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, 1, calls)
	})

	t.Run("accepts new price format", func(t *testing.T) {
		var received usecase.ImportAdsRequest

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{1, 2}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:    "bike",
				Price:    ptr(json.Number("150.50")),
				PhotoURL: "https://example.com/bike.jpg",
			},
			{
				Title:    "scooter",
				Price:    ptr(json.Number("80")),
				PhotoURL: "https://example.com/scooter.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 2)

		assert.Equal(t, int64(15050), received.Ads[0].PriceCents)
		assert.Equal(t, int64(8000), received.Ads[1].PriceCents)
	})

	t.Run("accepts mixed price formats", func(t *testing.T) {
		var received usecase.ImportAdsRequest

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{1, 2}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:    "bike",
				Price:    ptr(json.Number("150.50")),
				PhotoURL: "https://example.com/bike.jpg",
			},
			{
				Title:      "scooter",
				PriceCents: ptr(int64(8000)),
				PhotoURL:   "https://example.com/scooter.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 2)

		assert.Equal(t, int64(15050), received.Ads[0].PriceCents)
		assert.Equal(t, int64(8000), received.Ads[1].PriceCents)
	})

	t.Run("returns 400 without importing when a price is invalid", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return []domain.AdID{1, 2, 3}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:    "bike",
				Price:    ptr(json.Number("150.50")),
				PhotoURL: "https://example.com/bike.jpg",
			},
			{
				Title:    "scooter",
				Price:    ptr(json.Number("80.555")),
				PhotoURL: "https://example.com/scooter.jpg",
			},
			{
				Title:      "car",
				PriceCents: ptr(int64(200000)),
				PhotoURL:   "https://example.com/car.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls, "use case must not be called when batch validation fails")
	})
}
