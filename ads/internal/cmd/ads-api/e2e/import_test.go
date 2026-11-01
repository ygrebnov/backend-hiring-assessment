//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportAds(t *testing.T) {
	grace := userByNickname(t, "grace")

	t.Run("imports multiple ads", func(t *testing.T) {
		title1 := uniqueTitle("import-first")
		title2 := uniqueTitle("import-second")

		body := map[string]any{
			"ads": []map[string]any{
				validAd(title1),
				validAd(title2),
			},
		}

		resp := api.post(t, "/ads/import", grace.UserID, body)
		resp.requireStatus(t, http.StatusCreated)

		var result importAdsResponse
		resp.decode(t, &result)

		require.Len(t, result.IDs, 2)
		require.Positive(t, result.IDs[0])
		require.Positive(t, result.IDs[1])
		require.NotEqual(t, result.IDs[0], result.IDs[1])

		cleanupImportedAds(t, grace.UserID, result.IDs)

		first := getAd(t, result.IDs[0])
		second := getAd(t, result.IDs[1])

		assert.Equal(t, title1, first.Title)
		assert.Equal(t, title2, second.Title)
		assert.Equal(t, grace.UserID, first.UserID)
		assert.Equal(t, grace.UserID, second.UserID)
		assert.Equal(t, grace.Nickname, first.UserName)
		assert.Equal(t, grace.Nickname, second.UserName)
		assert.Equal(t, 150.50, first.Price)
		assert.Equal(t, 150.50, second.Price)
	})

	t.Run("accepts empty import", func(t *testing.T) {
		resp := api.post(t, "/ads/import", grace.UserID, map[string]any{
			"ads": []map[string]any{},
		})
		resp.requireStatus(t, http.StatusCreated)

		var result importAdsResponse
		resp.decode(t, &result)

		assert.Empty(t, result.IDs)
	})

	t.Run("rejects invalid ad without creating any ads", func(t *testing.T) {
		title1 := uniqueTitle("import-valid")
		title2 := uniqueTitle("import-invalid")

		body := map[string]any{
			"ads": []map[string]any{
				validAd(title1),
				{
					"title":     title2,
					"price":     12.345,
					"photo_url": "https://example.com/invalid.jpg",
				},
			},
		}

		resp := api.post(t, "/ads/import", grace.UserID, body)
		resp.requireStatus(t, http.StatusBadRequest)

		assertNoAdsWithTitle(t, title1)
		assertNoAdsWithTitle(t, title2)
	})

	t.Run("rejects missing authentication", func(t *testing.T) {
		resp := api.post(t, "/ads/import", "", map[string]any{
			"ads": []map[string]any{
				validAd(uniqueTitle("unauthorized-import")),
			},
		})

		resp.requireStatus(t, http.StatusUnauthorized)
	})
}

func TestImportAdsIdempotency(t *testing.T) {
	heidi := userByNickname(t, "heidi")
	ivan := userByNickname(t, "ivan")
	judy := userByNickname(t, "judy")
	alice := userByNickname(t, "alice")
	bob := userByNickname(t, "bob")
	carol := userByNickname(t, "carol")

	t.Run("retry returns original IDs", func(t *testing.T) {
		title1 := uniqueTitle("idempotent-first")
		title2 := uniqueTitle("idempotent-second")
		key := uniqueTitle("import-key")

		body := map[string]any{
			"ads": []map[string]any{
				validAd(title1),
				validAd(title2),
			},
		}

		first := importWithKey(t, heidi.UserID, key, body)
		first.requireStatus(t, http.StatusCreated)

		var firstResult importAdsResponse
		first.decode(t, &firstResult)

		require.Len(t, firstResult.IDs, 2)

		cleanupImportedAds(t, heidi.UserID, firstResult.IDs)

		second := importWithKey(t, heidi.UserID, key, body)
		second.requireStatus(t, http.StatusCreated)

		var secondResult importAdsResponse
		second.decode(t, &secondResult)

		assert.Equal(t, firstResult.IDs, secondResult.IDs)

		assertAdsWithTitle(t, title1, 1)
		assertAdsWithTitle(t, title2, 1)
	})

	t.Run("same key with different payload returns conflict", func(t *testing.T) {
		key := uniqueTitle("conflict-key")
		originalTitle := uniqueTitle("original")
		conflictingTitle := uniqueTitle("conflicting")

		originalBody := map[string]any{
			"ads": []map[string]any{
				validAd(originalTitle),
			},
		}

		first := importWithKey(t, ivan.UserID, key, originalBody)
		first.requireStatus(t, http.StatusCreated)

		var result importAdsResponse
		first.decode(t, &result)

		require.Len(t, result.IDs, 1)
		cleanupImportedAds(t, ivan.UserID, result.IDs)

		conflictingBody := map[string]any{
			"ads": []map[string]any{
				validAd(conflictingTitle),
			},
		}

		second := importWithKey(t, ivan.UserID, key, conflictingBody)
		second.requireStatus(t, http.StatusConflict)

		assertAdsWithTitle(t, originalTitle, 1)
		assertNoAdsWithTitle(t, conflictingTitle)
	})

	t.Run("equivalent price representations are idempotent", func(t *testing.T) {
		key := uniqueTitle("normalized-key")
		title := uniqueTitle("normalized-price")

		firstBody := map[string]any{
			"ads": []map[string]any{
				{
					"title":     title,
					"price":     150.50,
					"photo_url": "https://example.com/photo.jpg",
				},
			},
		}

		first := importWithKey(t, judy.UserID, key, firstBody)
		first.requireStatus(t, http.StatusCreated)

		var firstResult importAdsResponse
		first.decode(t, &firstResult)

		require.Len(t, firstResult.IDs, 1)
		cleanupImportedAds(t, judy.UserID, firstResult.IDs)

		secondBody := map[string]any{
			"ads": []map[string]any{
				{
					"title":       title,
					"price_cents": 15050,
					"photo_url":   "https://example.com/photo.jpg",
				},
			},
		}

		second := importWithKey(t, judy.UserID, key, secondBody)
		second.requireStatus(t, http.StatusCreated)

		var secondResult importAdsResponse
		second.decode(t, &secondResult)

		assert.Equal(t, firstResult.IDs, secondResult.IDs)
		assertAdsWithTitle(t, title, 1)
	})

	t.Run("idempotency keys are scoped to users", func(t *testing.T) {
		key := uniqueTitle("shared-key")
		aliceTitle := uniqueTitle("alice-import")
		bobTitle := uniqueTitle("bob-import")

		aliceResp := importWithKey(t, alice.UserID, key, map[string]any{
			"ads": []map[string]any{validAd(aliceTitle)},
		})
		aliceResp.requireStatus(t, http.StatusCreated)

		var aliceResult importAdsResponse
		aliceResp.decode(t, &aliceResult)

		require.Len(t, aliceResult.IDs, 1)
		cleanupImportedAds(t, alice.UserID, aliceResult.IDs)

		bobResp := importWithKey(t, bob.UserID, key, map[string]any{
			"ads": []map[string]any{validAd(bobTitle)},
		})
		bobResp.requireStatus(t, http.StatusCreated)

		var bobResult importAdsResponse
		bobResp.decode(t, &bobResult)

		require.Len(t, bobResult.IDs, 1)
		cleanupImportedAds(t, bob.UserID, bobResult.IDs)

		assert.NotEqual(t, aliceResult.IDs[0], bobResult.IDs[0])

		assertAdsWithTitle(t, aliceTitle, 1)
		assertAdsWithTitle(t, bobTitle, 1)
	})

	t.Run("without key repeated imports create new ads", func(t *testing.T) {
		title := uniqueTitle("non-idempotent")

		body := map[string]any{
			"ads": []map[string]any{
				validAd(title),
			},
		}

		first := api.post(t, "/ads/import", carol.UserID, body)
		first.requireStatus(t, http.StatusCreated)

		var firstResult importAdsResponse
		first.decode(t, &firstResult)

		require.Len(t, firstResult.IDs, 1)
		cleanupImportedAds(t, carol.UserID, firstResult.IDs)

		second := api.post(t, "/ads/import", carol.UserID, body)
		second.requireStatus(t, http.StatusCreated)

		var secondResult importAdsResponse
		second.decode(t, &secondResult)

		require.Len(t, secondResult.IDs, 1)
		cleanupImportedAds(t, carol.UserID, secondResult.IDs)

		assert.NotEqual(t, firstResult.IDs[0], secondResult.IDs[0])
		assertAdsWithTitle(t, title, 2)
	})
}

// Import-specific helpers.

func importWithKey(
	t *testing.T,
	userID, key string,
	body any,
) apiResponse {
	t.Helper()

	headers := make(http.Header)
	headers.Set("Idempotency-Key", key)

	return api.postWithHeaders(t, "/ads/import", userID, body, headers)
}

func cleanupImportedAds(t *testing.T, userID string, ids []int64) {
	t.Helper()

	t.Cleanup(func() {
		for _, id := range ids {
			cleanupAd(t, id, userID)
		}
	})
}

func assertNoAdsWithTitle(t *testing.T, title string) {
	t.Helper()
	assertAdsWithTitle(t, title, 0)
}

func assertAdsWithTitle(t *testing.T, title string, expected int) {
	t.Helper()

	resp := api.get(t, "/ads?title="+url.QueryEscape(title))
	resp.requireStatus(t, http.StatusOK)

	var result listAdsResponse
	resp.decode(t, &result)

	require.Len(t, result.Ads, expected,
		"unexpected number of ads with title %q", title)

	for _, ad := range result.Ads {
		assert.Equal(t, title, ad.Title,
			fmt.Sprintf("unexpected ad ID %d", ad.ID))
	}
}

func TestImportAdsAtomicity(t *testing.T) {
	grace := userByNickname(t, "grace")

	t.Run("database failure rolls back all ads", func(t *testing.T) {
		installImportFailureTrigger(t)

		firstTitle := uniqueTitle("atomicity-first")
		rejectedTitle := uniqueTitle("atomicity-reject")
		thirdTitle := uniqueTitle("atomicity-third")

		body := map[string]any{
			"ads": []map[string]any{
				validAd(firstTitle),
				validAd(rejectedTitle),
				validAd(thirdTitle),
			},
		}

		resp := api.post(t, "/ads/import", grace.UserID, body)
		resp.requireStatus(t, http.StatusInternalServerError)

		// The first ad was inserted before the database error.
		// The transaction must roll it back.
		assertNoAdsWithTitle(t, firstTitle)
		assertNoAdsWithTitle(t, rejectedTitle)
		assertNoAdsWithTitle(t, thirdTitle)
	})

	t.Run("failed import does not consume idempotency key", func(t *testing.T) {
		removeTrigger := installImportFailureTrigger(t)

		key := uniqueTitle("atomicity-key")
		firstTitle := uniqueTitle("atomicity-first")
		rejectedTitle := uniqueTitle("atomicity-reject")
		thirdTitle := uniqueTitle("atomicity-third")

		body := map[string]any{
			"ads": []map[string]any{
				validAd(firstTitle),
				validAd(rejectedTitle),
				validAd(thirdTitle),
			},
		}

		// The first attempt must fail during the second insertion.
		first := importWithKey(t, grace.UserID, key, body)
		first.requireStatus(t, http.StatusInternalServerError)

		assertNoAdsWithTitle(t, firstTitle)
		assertNoAdsWithTitle(t, rejectedTitle)
		assertNoAdsWithTitle(t, thirdTitle)

		// The failed transaction must not leave an idempotency record.
		var count int

		err := db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*)
			 FROM ad_imports
			 WHERE user_id = $1::uuid
			   AND idempotency_key = $2`,
			grace.UserID,
			key,
		).Scan(&count)

		require.NoError(t, err)
		assert.Zero(t, count)

		// Remove the simulated database failure before retrying.
		removeTrigger()

		// The same request and idempotency key must now succeed.
		second := importWithKey(t, grace.UserID, key, body)
		second.requireStatus(t, http.StatusCreated)

		var result importAdsResponse
		second.decode(t, &result)

		require.Len(t, result.IDs, 3)
		cleanupImportedAds(t, grace.UserID, result.IDs)

		assertAdsWithTitle(t, firstTitle, 1)
		assertAdsWithTitle(t, rejectedTitle, 1)
		assertAdsWithTitle(t, thirdTitle, 1)

		// A subsequent retry must return the original IDs.
		third := importWithKey(t, grace.UserID, key, body)
		third.requireStatus(t, http.StatusCreated)

		var replay importAdsResponse
		third.decode(t, &replay)

		assert.Equal(t, result.IDs, replay.IDs)
	})
}

func installImportFailureTrigger(t *testing.T) func() {
	t.Helper()

	const createFunction = `
		CREATE OR REPLACE FUNCTION e2e_reject_import_ad()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF NEW.title LIKE 'e2e-atomicity-reject-%' THEN
				RAISE EXCEPTION 'E2E simulated database insertion failure';
			END IF;

			RETURN NEW;
		END;
		$$`

	const createTrigger = `
		CREATE TRIGGER e2e_import_failure
		BEFORE INSERT ON ads
		FOR EACH ROW
		EXECUTE FUNCTION e2e_reject_import_ad()`

	_, err := db.ExecContext(t.Context(), createFunction)
	require.NoError(t, err)

	_, err = db.ExecContext(t.Context(), createTrigger)
	require.NoError(t, err)

	var once sync.Once

	remove := func() {
		once.Do(func() {
			// t.Context() is canceled before t.Cleanup() runs.
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			_, err := db.ExecContext(
				ctx,
				"DROP TRIGGER IF EXISTS e2e_import_failure ON ads",
			)
			require.NoError(t, err)

			_, err = db.ExecContext(
				ctx,
				"DROP FUNCTION IF EXISTS e2e_reject_import_ad()",
			)
			require.NoError(t, err)
		})
	}

	t.Cleanup(remove)

	return remove
}
