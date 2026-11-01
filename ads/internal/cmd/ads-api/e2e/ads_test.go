//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	resp := api.get(t, "/health")

	resp.requireStatus(t, http.StatusOK)
	assert.Empty(t, resp.Body)
}

func TestCreateAd(t *testing.T) {
	alice := userByNickname(t, "alice")
	bob := userByNickname(t, "bob")

	t.Run("creates ad with decimal price", func(t *testing.T) {
		title := uniqueTitle("decimal-price")

		resp := api.post(t, "/ads", alice.UserID, validAd(title))
		resp.requireStatus(t, http.StatusCreated)

		var created createAdResponse
		resp.decode(t, &created)

		require.Positive(t, created.ID)

		t.Cleanup(func() {
			cleanupAd(t, created.ID, alice.UserID)
		})

		ad := getAd(t, created.ID)

		assert.Equal(t, title, ad.Title)
		assert.Equal(t, 150.50, ad.Price)
		assert.Equal(t, int64(15050), ad.PriceCents)
		assert.Equal(t, alice.UserID, ad.UserID)
		assert.Equal(t, alice.Nickname, ad.UserName)
		assert.Equal(t, "https://example.com/photo.jpg", ad.PhotoURL)

		_, err := time.Parse(time.RFC3339Nano, ad.CreatedAt)
		require.NoError(t, err)
	})

	t.Run("creates ad with legacy price_cents", func(t *testing.T) {
		title := uniqueTitle("legacy-price")

		resp := api.post(t, "/ads", bob.UserID, map[string]any{
			"title":       title,
			"price_cents": 12345,
			"photo_url":   "https://example.com/legacy.jpg",
		})
		resp.requireStatus(t, http.StatusCreated)

		var created createAdResponse
		resp.decode(t, &created)

		require.Positive(t, created.ID)

		t.Cleanup(func() {
			cleanupAd(t, created.ID, bob.UserID)
		})

		ad := getAd(t, created.ID)

		assert.Equal(t, 123.45, ad.Price)
		assert.Equal(t, int64(12345), ad.PriceCents)
		assert.Equal(t, bob.UserID, ad.UserID)
		assert.Equal(t, bob.Nickname, ad.UserName)
	})

	t.Run("rejects conflicting price fields", func(t *testing.T) {
		resp := api.post(t, "/ads", alice.UserID, map[string]any{
			"title":       uniqueTitle("conflicting-price"),
			"price":       100.00,
			"price_cents": 10000,
			"photo_url":   "https://example.com/photo.jpg",
		})

		resp.requireStatus(t, http.StatusBadRequest)
	})

	t.Run("rejects invalid decimal precision", func(t *testing.T) {
		resp := api.post(t, "/ads", alice.UserID, map[string]any{
			"title":     uniqueTitle("invalid-price"),
			"price":     12.345,
			"photo_url": "https://example.com/photo.jpg",
		})

		resp.requireStatus(t, http.StatusBadRequest)
	})

	t.Run("rejects missing price", func(t *testing.T) {
		resp := api.post(t, "/ads", alice.UserID, map[string]any{
			"title":     uniqueTitle("missing-price"),
			"photo_url": "https://example.com/photo.jpg",
		})

		resp.requireStatus(t, http.StatusBadRequest)
	})

	t.Run("requires authentication", func(t *testing.T) {
		resp := api.post(t, "/ads", "", validAd(uniqueTitle("unauthorized")))

		resp.requireStatus(t, http.StatusUnauthorized)
	})
}

func TestGetAd(t *testing.T) {
	carol := userByNickname(t, "carol")
	bob := userByNickname(t, "bob")

	id := createAd(t, carol.UserID, uniqueTitle("get-ad"))

	t.Cleanup(func() {
		cleanupAd(t, id, carol.UserID)
	})

	t.Run("returns ad with owner username", func(t *testing.T) {
		ad := getAd(t, id)

		assert.Equal(t, id, ad.ID)
		assert.Equal(t, carol.UserID, ad.UserID)
		assert.Equal(t, carol.Nickname, ad.UserName)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		resp := api.get(t, "/ads/invalid")

		resp.requireStatus(t, http.StatusBadRequest)
	})

	t.Run("returns 404 for nonexistent ID", func(t *testing.T) {
		resp := api.get(t, "/ads/9223372036854775807")

		resp.requireStatus(t, http.StatusNotFound)
	})

	t.Run("returns seeded ad", func(t *testing.T) {
		ad := getAd(t, 1)

		assert.Equal(t, int64(1), ad.ID)
		assert.Equal(t, "Leather sofa #1", ad.Title)
		assert.Equal(t, 250.00, ad.Price)
		assert.Equal(t, int64(25000), ad.PriceCents)
		assert.Equal(t, bob.UserID, ad.UserID)
		assert.Equal(t, bob.Nickname, ad.UserName)
	})
}

func TestListAds(t *testing.T) {
	dave := userByNickname(t, "dave")

	title := uniqueTitle("searchable-bike")
	id := createAd(t, dave.UserID, title)

	t.Cleanup(func() {
		cleanupAd(t, id, dave.UserID)
	})

	t.Run("lists ads", func(t *testing.T) {
		resp := api.get(t, "/ads")
		resp.requireStatus(t, http.StatusOK)

		var result listAdsResponse
		resp.decode(t, &result)

		assert.NotEmpty(t, result.Ads)
		assert.True(t, containsAd(result.Ads, id))
	})

	t.Run("filters by title", func(t *testing.T) {
		resp := api.get(t, "/ads?title="+url.QueryEscape(title))
		resp.requireStatus(t, http.StatusOK)

		var result listAdsResponse
		resp.decode(t, &result)

		require.Len(t, result.Ads, 1)
		assert.Equal(t, id, result.Ads[0].ID)
	})

	t.Run("filters by user", func(t *testing.T) {
		resp := api.get(t, "/ads?user_id="+url.QueryEscape(dave.UserID))
		resp.requireStatus(t, http.StatusOK)

		var result listAdsResponse
		resp.decode(t, &result)

		assert.True(t, containsAd(result.Ads, id))
	})

	t.Run("filters by price range", func(t *testing.T) {
		path := "/ads?title=" + url.QueryEscape(title) +
			"&min_price_cents=15050&max_price_cents=15050"

		resp := api.get(t, path)
		resp.requireStatus(t, http.StatusOK)

		var result listAdsResponse
		resp.decode(t, &result)

		require.Len(t, result.Ads, 1)
		assert.Equal(t, id, result.Ads[0].ID)
	})

	t.Run("returns empty list when no ads match", func(t *testing.T) {
		resp := api.get(t, "/ads?title="+url.QueryEscape(uniqueTitle("nonexistent")))
		resp.requireStatus(t, http.StatusOK)

		var result listAdsResponse
		resp.decode(t, &result)

		assert.Empty(t, result.Ads)
	})

	t.Run("rejects invalid price filter", func(t *testing.T) {
		resp := api.get(t, "/ads?min_price_cents=invalid")

		resp.requireStatus(t, http.StatusBadRequest)
	})
}

func TestDeleteAd(t *testing.T) {
	erin := userByNickname(t, "erin")
	frank := userByNickname(t, "frank")
	alice := userByNickname(t, "alice")

	t.Run("deletes own ad", func(t *testing.T) {
		id := createAd(t, erin.UserID, uniqueTitle("delete-own"))

		resp := api.delete(t, adPath(id), erin.UserID)
		resp.requireStatus(t, http.StatusNoContent)

		assert.Empty(t, resp.Body)

		getResp := api.get(t, adPath(id))
		getResp.requireStatus(t, http.StatusNotFound)
	})

	t.Run("rejects deletion by another user", func(t *testing.T) {
		id := createAd(t, frank.UserID, uniqueTitle("delete-forbidden"))

		t.Cleanup(func() {
			cleanupAd(t, id, frank.UserID)
		})

		resp := api.delete(t, adPath(id), alice.UserID)
		resp.requireStatus(t, http.StatusForbidden)

		// The ad must still exist after the rejected deletion.
		ad := getAd(t, id)
		assert.Equal(t, frank.UserID, ad.UserID)
	})

	t.Run("returns 404 for nonexistent ad", func(t *testing.T) {
		resp := api.delete(t, "/ads/9223372036854775807", alice.UserID)

		resp.requireStatus(t, http.StatusNotFound)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		resp := api.delete(t, "/ads/invalid", alice.UserID)

		resp.requireStatus(t, http.StatusBadRequest)
	})

	t.Run("requires authentication", func(t *testing.T) {
		resp := api.delete(t, "/ads/1", "")

		resp.requireStatus(t, http.StatusUnauthorized)
	})
}

// Helpers specific to ad E2E tests.

func uniqueTitle(prefix string) string {
	return fmt.Sprintf("e2e-%s-%d", prefix, time.Now().UnixNano())
}

func adPath(id int64) string {
	return fmt.Sprintf("/ads/%d", id)
}

func createAd(t *testing.T, userID, title string) int64 {
	t.Helper()

	resp := api.post(t, "/ads", userID, validAd(title))
	resp.requireStatus(t, http.StatusCreated)

	var created createAdResponse
	resp.decode(t, &created)

	require.Positive(t, created.ID)

	return created.ID
}

func getAd(t *testing.T, id int64) adResponse {
	t.Helper()

	resp := api.get(t, adPath(id))
	resp.requireStatus(t, http.StatusOK)

	var ad adResponse
	resp.decode(t, &ad)

	return ad
}

func cleanupAd(t *testing.T, id int64, userID string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := api.request(
		ctx,
		http.MethodDelete,
		adPath(id),
		userID,
		nil,
		nil,
	)
	require.NoError(t, err)
	resp.requireStatus(t, http.StatusNoContent)
}

func containsAd(ads []adResponse, id int64) bool {
	for _, ad := range ads {
		if ad.ID == id {
			return true
		}
	}

	return false
}
