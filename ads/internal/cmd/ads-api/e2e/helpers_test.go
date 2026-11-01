//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiClient struct {
	baseURL    string
	httpClient *http.Client
}

type apiResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (c *apiClient) request(
	ctx context.Context,
	method string,
	path string,
	userID string,
	body any,
	headers http.Header,
) (apiResponse, error) {
	var reader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return apiResponse{}, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return apiResponse{}, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("Authorization", userID)
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse{}, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, fmt.Errorf("read response body: %w", err)
	}

	return apiResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       data,
	}, nil
}

func (c *apiClient) get(t *testing.T, path string) apiResponse {
	t.Helper()

	resp, err := c.request(t.Context(), http.MethodGet, path, "", nil, nil)
	require.NoError(t, err)

	return resp
}

func (c *apiClient) post(t *testing.T, path, userID string, body any) apiResponse {
	t.Helper()

	resp, err := c.request(t.Context(), http.MethodPost, path, userID, body, nil)
	require.NoError(t, err)

	return resp
}

func (c *apiClient) postWithHeaders(
	t *testing.T,
	path, userID string,
	body any,
	headers http.Header,
) apiResponse {
	t.Helper()

	resp, err := c.request(t.Context(), http.MethodPost, path, userID, body, headers)
	require.NoError(t, err)

	return resp
}

func (c *apiClient) delete(t *testing.T, path, userID string) apiResponse {
	t.Helper()

	resp, err := c.request(t.Context(), http.MethodDelete, path, userID, nil, nil)
	require.NoError(t, err)

	return resp
}

func (r apiResponse) decode(t *testing.T, dst any) {
	t.Helper()

	require.NoError(t, json.Unmarshal(r.Body, dst),
		"response body: %s", string(r.Body))
}

func (r apiResponse) requireStatus(t *testing.T, expected int) {
	t.Helper()

	require.Equal(t, expected, r.StatusCode,
		"unexpected response body: %s", string(r.Body))
}

type adResponse struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Price      float64 `json:"price"`
	PriceCents int64   `json:"price_cents"`
	PhotoURL   string  `json:"photo_url"`
	UserID     string  `json:"user_id"`
	UserName   string  `json:"user_name"`
	CreatedAt  string  `json:"created_at"`
}

type createAdResponse struct {
	ID int64 `json:"id"`
}

type listAdsResponse struct {
	Ads []adResponse `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

func validAd(title string) map[string]any {
	return map[string]any{
		"title":     title,
		"price":     150.50,
		"photo_url": "https://example.com/photo.jpg",
	}
}
