//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	e2ePublicationLimit  = 5
	e2ePublicationWindow = 10 * time.Second
)

// resetPublicationLedger clears quota state to keep E2E tests repeatable
// against an already-running environment. It must only run against
// the dedicated E2E database.
func resetPublicationLedger(t *testing.T) {
	t.Helper()

	_, err := db.ExecContext(t.Context(), "TRUNCATE TABLE ad_publications")
	require.NoError(t, err)
}

func TestPublicationQuota(t *testing.T) {
	t.Run("allows five publications and rejects the sixth", func(t *testing.T) {
		resetPublicationLedger(t)

		userID := userByNickname(t, "alice").UserID

		for i := range e2ePublicationLimit {
			id := createAd(t, userID, uniqueTitle(fmt.Sprintf("quota-%d", i)))

			t.Cleanup(func() {
				cleanupAd(t, id, userID)
			})
		}

		resp := api.post(t, "/ads", userID, validAd(uniqueTitle("quota-exceeded")))
		resp.requireStatus(t, http.StatusTooManyRequests)
	})

	t.Run("quota is independent per user", func(t *testing.T) {
		resetPublicationLedger(t)

		userA := userByNickname(t, "alice").UserID
		userB := userByNickname(t, "bob").UserID

		for i := range e2ePublicationLimit {
			id := createAd(t, userA, uniqueTitle(fmt.Sprintf("user-a-%d", i)))

			t.Cleanup(func() {
				cleanupAd(t, id, userA)
			})
		}

		exceeded := api.post(t, "/ads", userA, validAd(uniqueTitle("user-a-exceeded")))
		exceeded.requireStatus(t, http.StatusTooManyRequests)

		id := createAd(t, userB, uniqueTitle("user-b-allowed"))

		t.Cleanup(func() {
			cleanupAd(t, id, userB)
		})
	})

	t.Run("deleting an ad does not restore quota", func(t *testing.T) {
		resetPublicationLedger(t)

		userID := userByNickname(t, "alice").UserID

		var ids []int64

		for i := range e2ePublicationLimit {
			id := createAd(t, userID, uniqueTitle(fmt.Sprintf("delete-quota-%d", i)))
			ids = append(ids, id)
		}

		// Delete one published ad.
		resp := api.delete(t, adPath(ids[0]), userID)
		resp.requireStatus(t, http.StatusNoContent)

		// Clean up the remaining ads.
		for _, id := range ids[1:] {
			t.Cleanup(func() {
				cleanupAd(t, id, userID)
			})
		}

		// The deleted publication must still count toward the quota.
		exceeded := api.post(t, "/ads", userID, validAd(uniqueTitle("quota-after-delete")))
		exceeded.requireStatus(t, http.StatusTooManyRequests)
	})

	t.Run("bulk imports do not consume quota", func(t *testing.T) {
		resetPublicationLedger(t)

		userID := userByNickname(t, "alice").UserID

		ads := make([]map[string]any, e2ePublicationLimit+1)

		for i := range ads {
			ads[i] = validAd(uniqueTitle(fmt.Sprintf("bulk-quota-%d", i)))
		}

		importResp := api.post(t, "/ads/import", userID, map[string]any{
			"ads": ads,
		})
		importResp.requireStatus(t, http.StatusCreated)

		var imported importAdsResponse
		importResp.decode(t, &imported)

		require.Len(t, imported.IDs, len(ads))

		cleanupImportedAds(t, userID, imported.IDs)

		// Importing more than five ads must not affect publication quota.
		for i := range e2ePublicationLimit {
			id := createAd(t, userID, uniqueTitle(fmt.Sprintf("after-import-%d", i)))

			t.Cleanup(func() {
				cleanupAd(t, id, userID)
			})
		}

		exceeded := api.post(t, "/ads", userID, validAd(uniqueTitle("after-import-exceeded")))
		exceeded.requireStatus(t, http.StatusTooManyRequests)
	})
}

func TestPublicationQuotaConcurrent(t *testing.T) {
	const requests = 20

	resetPublicationLedger(t)

	userID := userByNickname(t, "alice").UserID

	type result struct {
		statusCode int
		id         int64
		err        error
	}

	results := make(chan result, requests)

	var wg sync.WaitGroup

	for i := range requests {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			resp, err := api.request(
				ctx,
				http.MethodPost,
				"/ads",
				userID,
				validAd(uniqueTitle(fmt.Sprintf("concurrent-%d", i))),
				nil,
			)

			if err != nil {
				results <- result{err: err}
				return
			}

			r := result{statusCode: resp.StatusCode}

			if resp.StatusCode == http.StatusCreated {
				var created createAdResponse

				if err := json.Unmarshal(resp.Body, &created); err != nil {
					r.err = err
				} else {
					r.id = created.ID
				}
			}

			results <- r
		}(i)
	}

	wg.Wait()
	close(results)

	var createdIDs []int64
	var accepted, rejected int

	for result := range results {
		if result.id > 0 {
			createdIDs = append(createdIDs, result.id)
		}

		if !assert.NoError(t, result.err) {
			continue
		}

		switch result.statusCode {
		case http.StatusCreated:
			accepted++
			assert.Positive(t, result.id)

		case http.StatusTooManyRequests:
			rejected++

		default:
			t.Errorf("unexpected HTTP status: %d", result.statusCode)
		}
	}

	// Register cleanup before checking the final assertions.
	cleanupImportedAds(t, userID, createdIDs)

	assert.Equal(t, e2ePublicationLimit, accepted)
	assert.Equal(t, requests-e2ePublicationLimit, rejected)
}

func TestPublicationQuotaRollingWindow(t *testing.T) {
	resetPublicationLedger(t)

	userID := userByNickname(t, "alice").UserID

	// Fill the quota.
	for i := range e2ePublicationLimit {
		id := createAd(t, userID, uniqueTitle(fmt.Sprintf("rolling-%d", i)))

		t.Cleanup(func() {
			cleanupAd(t, id, userID)
		})
	}

	// Verify the quota is exhausted.
	exceeded := api.post(t, "/ads", userID, validAd(uniqueTitle("rolling-exceeded")))
	exceeded.requireStatus(t, http.StatusTooManyRequests)

	// Wait until the oldest publication expires from the rolling window.
	deadline := time.Now().Add(e2ePublicationWindow + 5*time.Second)

	for time.Now().Before(deadline) {
		resp := api.post(t, "/ads", userID, validAd(uniqueTitle("rolling-retry")))

		switch resp.StatusCode {
		case http.StatusCreated:
			var created createAdResponse
			resp.decode(t, &created)

			require.Positive(t, created.ID)

			t.Cleanup(func() {
				cleanupAd(t, created.ID, userID)
			})

			return

		case http.StatusTooManyRequests:
			time.Sleep(200 * time.Millisecond)

		default:
			t.Fatalf(
				"unexpected HTTP status while waiting for quota expiration: %d: %s",
				resp.StatusCode,
				string(resp.Body),
			)
		}
	}

	t.Fatal("publication quota did not expire within the expected window")
}
