package mock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var ctx = context.Background()

func TestMock_SearchAds(t *testing.T) {
	t.Run("returns configured ads", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.SearchAdsRequest{Title: "bike"}
		want := []domain.Ad{{ID: 1, Title: "bike", Price: 5000}}

		m.On("SearchAds", ctx, req).Return(want, nil)

		got, err := m.SearchAds(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns nil slice on error", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.SearchAdsRequest{Title: "bike"}
		boom := errors.New("db down")

		m.On("SearchAds", ctx, req).Return(nil, boom)

		got, err := m.SearchAds(ctx, req)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})
}

func TestMock_GetAdByID(t *testing.T) {
	t.Run("returns configured ad", func(t *testing.T) {
		m := mock.New(t)
		want := domain.Ad{ID: 42, Title: "surfboard", Price: 15000}

		m.On("GetAdByID", ctx, domain.AdID(42)).Return(want, nil)

		got, err := m.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns ErrNotFound", func(t *testing.T) {
		m := mock.New(t)

		m.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, adsdao.ErrNotFound)

		_, err := m.GetAdByID(ctx, 99)

		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})
}

func TestMock_CreateAd(t *testing.T) {
	t.Run("returns generated ID", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.CreateAdRequest{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "user-1",
		}

		m.On("CreateAd", ctx, req).Return(domain.AdID(7), nil)

		got, err := m.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("returns ErrUnknownOwner when owner does not exist", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.CreateAdRequest{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "ghost",
		}

		m.On("CreateAd", ctx, req).Return(domain.AdID(0), adsdao.ErrUnknownOwner)

		_, err := m.CreateAd(ctx, req)

		assert.ErrorIs(t, err, adsdao.ErrUnknownOwner)
	})
}

func TestMock_DeleteAdByID(t *testing.T) {
	t.Run("returns nil error", func(t *testing.T) {
		m := mock.New(t)

		m.On(
			"DeleteAdByID",
			ctx,
			domain.AdID(42),
			domain.UserID("user-1"),
		).Return(nil)

		err := m.DeleteAdByID(ctx, 42, "user-1")

		require.NoError(t, err)
	})

	t.Run("returns ErrNotFound", func(t *testing.T) {
		m := mock.New(t)

		m.On(
			"DeleteAdByID",
			ctx,
			domain.AdID(99),
			domain.UserID("user-1"),
		).Return(adsdao.ErrNotFound)

		err := m.DeleteAdByID(ctx, 99, "user-1")

		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("returns ErrForbidden", func(t *testing.T) {
		m := mock.New(t)

		m.On(
			"DeleteAdByID",
			ctx,
			domain.AdID(99),
			domain.UserID("user-1"),
		).Return(adsdao.ErrForbidden)

		err := m.DeleteAdByID(ctx, 99, "user-1")

		assert.ErrorIs(t, err, adsdao.ErrForbidden)
	})
}

func TestMock_Begin(t *testing.T) {
	t.Run("returns the mock itself as transaction", func(t *testing.T) {
		m := mock.New(t)

		m.On("Begin", ctx).Return(nil)

		tx, err := m.Begin(ctx)

		require.NoError(t, err)
		assert.Same(t, m, tx)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("Begin", ctx).Return(boom)

		tx, err := m.Begin(ctx)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
	})
}

func TestMock_ImportAds(t *testing.T) {
	t.Run("returns configured ad IDs", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.ImportAdsRequest{
			OwnerID: "user-1",
			Ads: []adsdao.CreateAdRequest{
				{
					Title:      "bike",
					PriceCents: 5000,
					OwnerID:    "user-1",
				},
				{
					Title:      "scooter",
					PriceCents: 12000,
					OwnerID:    "user-1",
				},
			},
			IdempotencyKey: "import-123",
			RequestHash:    "hash-abc",
		}
		want := []domain.AdID{101, 102}

		m.On("ImportAds", ctx, req).Return(want, nil)

		got, err := m.ImportAds(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.ImportAdsRequest{
			OwnerID:        "user-1",
			Ads:            []adsdao.CreateAdRequest{{Title: "bike", PriceCents: 5000}},
			IdempotencyKey: "import-123",
		}
		boom := errors.New("db down")

		m.On("ImportAds", ctx, req).Return(nil, boom)

		got, err := m.ImportAds(ctx, req)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})

	t.Run("returns idempotency conflict", func(t *testing.T) {
		m := mock.New(t)

		req := adsdao.ImportAdsRequest{
			OwnerID: "user-1",
			Ads: []adsdao.CreateAdRequest{
				{
					Title:      "bike",
					PriceCents: 5000,
					OwnerID:    "user-1",
				},
			},
			IdempotencyKey: "import-123",
			RequestHash:    "hash-new",
		}

		m.On("ImportAds", ctx, req).
			Return(nil, adsdao.ErrIdempotencyConflict)

		got, err := m.ImportAds(ctx, req)

		require.ErrorIs(t, err, adsdao.ErrIdempotencyConflict)
		assert.Nil(t, got)
	})
}

func TestMock_CreateAdWithQuota(t *testing.T) {
	req := adsdao.CreateAdWithQuotaRequest{
		CreateAdRequest: adsdao.CreateAdRequest{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "user-1",
		},
		Limit:  5,
		Window: 24 * time.Hour,
	}

	t.Run("returns generated ID", func(t *testing.T) {
		m := mock.New(t)

		m.On("CreateAdWithQuota", ctx, req).
			Return(domain.AdID(7), nil)

		got, err := m.CreateAdWithQuota(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("returns publication limit error", func(t *testing.T) {
		m := mock.New(t)

		m.On("CreateAdWithQuota", ctx, req).
			Return(domain.AdID(0), adsdao.ErrPublicationLimitExceeded)

		got, err := m.CreateAdWithQuota(ctx, req)

		require.ErrorIs(t, err, adsdao.ErrPublicationLimitExceeded)
		assert.Zero(t, got)
	})

	t.Run("returns unexpected database error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("database unavailable")

		m.On("CreateAdWithQuota", ctx, req).
			Return(domain.AdID(0), boom)

		got, err := m.CreateAdWithQuota(ctx, req)

		require.ErrorIs(t, err, boom)
		assert.Zero(t, got)
	})
}

func TestMock_Commit(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("Commit").Return(nil)

		require.NoError(t, m.Commit())
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("commit failed")

		m.On("Commit").Return(boom)

		assert.ErrorIs(t, m.Commit(), boom)
	})
}

func TestMock_Rollback(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("Rollback").Return(nil)

		require.NoError(t, m.Rollback())
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("rollback failed")

		m.On("Rollback").Return(boom)

		assert.ErrorIs(t, m.Rollback(), boom)
	})
}
