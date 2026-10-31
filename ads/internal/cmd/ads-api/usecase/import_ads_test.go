package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	daomock "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestImportAds(t *testing.T) {
	ctx := context.Background()
	ownerID := domain.UserID("user-1")

	ads := []usecase.CreateAdRequest{
		{
			Title:      "mountain bike",
			PriceCents: 15000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    ownerID,
		},
		{
			Title:      "scooter",
			PriceCents: 12061,
			PhotoURL:   "https://example.com/scooter.jpg",
			OwnerID:    ownerID,
		},
	}

	daoAds := []adsdao.CreateAdRequest{
		{
			Title:      "mountain bike",
			PriceCents: 15000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    ownerID,
		},
		{
			Title:      "scooter",
			PriceCents: 12061,
			PhotoURL:   "https://example.com/scooter.jpg",
			OwnerID:    ownerID,
		},
	}

	// Independently calculate the expected hash from the normalized DAO request.
	expectedHash := func(t *testing.T, ads []adsdao.CreateAdRequest) string {
		t.Helper()

		payload, err := json.Marshal(ads)
		require.NoError(t, err)

		sum := sha256.Sum256(payload)
		return hex.EncodeToString(sum[:])
	}

	t.Run("imports ads with idempotency key", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: "unique-key",
		}

		daoReq := adsdao.ImportAdsRequest{
			Ads:            daoAds,
			OwnerID:        ownerID,
			IdempotencyKey: "unique-key",
			RequestHash:    expectedHash(t, daoAds),
		}

		m.On("ImportAds", ctx, daoReq).
			Return([]domain.AdID{101, 102}, nil).
			Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{101, 102}, got)
	})

	t.Run("imports ads without idempotency key", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			Ads:     ads,
			OwnerID: ownerID,
		}

		daoReq := adsdao.ImportAdsRequest{
			Ads:     daoAds,
			OwnerID: ownerID,
			// No key means no hash.
			IdempotencyKey: "",
			RequestHash:    "",
		}

		m.On("ImportAds", ctx, daoReq).
			Return([]domain.AdID{101, 102}, nil).
			Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{101, 102}, got)
	})

	t.Run("enforces authenticated owner for every ad", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			OwnerID: ownerID,
			Ads: []usecase.CreateAdRequest{
				{
					Title:      "mountain bike",
					PriceCents: 15000,
					OwnerID:    "another-user",
				},
				{
					Title:      "scooter",
					PriceCents: 12061,
					OwnerID:    "",
				},
			},
		}

		expectedAds := []adsdao.CreateAdRequest{
			{
				Title:      "mountain bike",
				PriceCents: 15000,
				OwnerID:    ownerID,
			},
			{
				Title:      "scooter",
				PriceCents: 12061,
				OwnerID:    ownerID,
			},
		}

		m.On("ImportAds", ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
			Ads:     expectedAds,
		}).
			Return([]domain.AdID{101, 102}, nil).
			Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{101, 102}, got)
	})

	t.Run("calculates identical hash for identical requests", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: "unique-key",
		}

		hash := expectedHash(t, daoAds)

		m.On("ImportAds", ctx, mock.MatchedBy(func(r adsdao.ImportAdsRequest) bool {
			return r.OwnerID == ownerID &&
				r.IdempotencyKey == "unique-key" &&
				r.RequestHash == hash
		})).
			Return([]domain.AdID{101, 102}, nil).
			Twice()

		importAds := usecase.NewImportAdsFunc(m)

		first, err := importAds(ctx, req)
		require.NoError(t, err)

		second, err := importAds(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, first, second)
	})

	t.Run("calculates different hashes for different payloads", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: "unique-key",
		}

		modifiedAds := append([]usecase.CreateAdRequest(nil), ads...)
		modifiedAds[0].PriceCents++

		modifiedDAOAds := append([]adsdao.CreateAdRequest(nil), daoAds...)
		modifiedDAOAds[0].PriceCents++

		firstHash := expectedHash(t, daoAds)
		secondHash := expectedHash(t, modifiedDAOAds)

		require.NotEqual(t, firstHash, secondHash)

		m.On("ImportAds", ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            daoAds,
			IdempotencyKey: "unique-key",
			RequestHash:    firstHash,
		}).
			Return([]domain.AdID{101, 102}, nil).
			Once()

		m.On("ImportAds", ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            modifiedDAOAds,
			IdempotencyKey: "unique-key",
			RequestHash:    secondHash,
		}).
			Return(nil, adsdao.ErrIdempotencyConflict).
			Once()

		importAds := usecase.NewImportAdsFunc(m)

		_, err := importAds(ctx, req)
		require.NoError(t, err)

		req.Ads = modifiedAds

		_, err = importAds(ctx, req)
		require.ErrorIs(t, err, adsdao.ErrIdempotencyConflict)
	})

	t.Run("propagates dao error", func(t *testing.T) {
		m := daomock.New(t)
		dbErr := errors.New("connection reset")

		req := usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: "unique-key",
		}

		m.On("ImportAds", ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            daoAds,
			IdempotencyKey: "unique-key",
			RequestHash:    expectedHash(t, daoAds),
		}).
			Return(nil, dbErr).
			Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)
	})

	t.Run("propagates idempotency conflict", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: "unique-key",
		}

		m.On("ImportAds", ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            daoAds,
			IdempotencyKey: "unique-key",
			RequestHash:    expectedHash(t, daoAds),
		}).
			Return(nil, adsdao.ErrIdempotencyConflict).
			Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.ErrorIs(t, err, adsdao.ErrIdempotencyConflict)
		assert.Nil(t, got)
	})

	t.Run("accepts empty import", func(t *testing.T) {
		m := daomock.New(t)

		req := usecase.ImportAdsRequest{
			OwnerID: ownerID,
		}

		m.On("ImportAds", ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
			Ads:     []adsdao.CreateAdRequest{},
		}).
			Return([]domain.AdID{}, nil).
			Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
