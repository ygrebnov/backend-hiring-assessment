package usecase_test

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
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestCreateAd(t *testing.T) {
	ctx := context.Background()

	ucReq := usecase.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}
	params := usecase.CreateAdParams{
		PublicationLimit:  5,
		PublicationWindow: 24 * time.Hour,
	}
	daoReq := adsdao.CreateAdWithQuotaRequest{
		CreateAdRequest: adsdao.CreateAdRequest{
			Title:      ucReq.Title,
			PriceCents: ucReq.PriceCents,
			PhotoURL:   ucReq.PhotoURL,
			OwnerID:    ucReq.OwnerID,
		},
		Limit:  params.PublicationLimit,
		Window: params.PublicationWindow,
	}

	t.Run("nominal", func(t *testing.T) {
		m := mock.New(t)
		m.On("CreateAdWithQuota", ctx, daoReq).Return(domain.AdID(7), nil).Once()

		got, err := usecase.NewCreateAdFunc(m, params)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mock.New(t)
		dbErr := errors.New("connection reset")
		m.On("CreateAdWithQuota", ctx, daoReq).Return(domain.AdID(0), dbErr).Once()

		got, err := usecase.NewCreateAdFunc(m, params)(ctx, ucReq)

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorContains(t, err, "unable to create ad")
		assert.Zero(t, got)
	})

	t.Run("publication limit exceeded", func(t *testing.T) {
		m := mock.New(t)
		m.On("CreateAdWithQuota", ctx, daoReq).
			Return(domain.AdID(0), adsdao.ErrPublicationLimitExceeded).
			Once()

		got, err := usecase.NewCreateAdFunc(m, params)(ctx, ucReq)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrPublicationLimitExceeded)
		assert.Zero(t, got)
	})
}
