package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	userspkg "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersmock "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/mock"
)

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		ad := domain.Ad{
			ID:     domain.AdID(42),
			Title:  "bike",
			Price:  5000,
			UserID: "user-1",
		}

		m := mockdao.New(t)
		identityClient := usersmock.New(t)

		m.On("GetAdByID", ctx, ad.ID).
			Return(ad, nil).
			Once()

		identityClient.
			On("GetUserByID", ctx, ad.UserID).
			Return(userspkg.User{
				UserID:   ad.UserID,
				Nickname: "alice",
			}, nil).
			Once()

		got, err := usecase.NewGetAdByIDFunc(m, identityClient)(ctx, ad.ID)

		require.NoError(t, err)

		want := ad
		want.UserName = "alice"

		assert.Equal(t, want, got)
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, domain.AdID(99)).
			Return(domain.Ad{}, adsdao.ErrNotFound)

		_, err := usecase.NewGetAdByIDFunc(m, nil)(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")
		m.On("GetAdByID", ctx, domain.AdID(1)).
			Return(domain.Ad{}, dbErr)

		_, err := usecase.NewGetAdByIDFunc(m, nil)(ctx, 1)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("error from identity", func(t *testing.T) {
		ad := domain.Ad{
			ID:     domain.AdID(42),
			Title:  "bike",
			Price:  5000,
			UserID: "user-1",
		}

		m := mockdao.New(t)
		identityClient := usersmock.New(t)
		dbErr := errors.New("connection reset")
		identityClient.On("GetUserByID", ctx, ad.UserID).
			Return(userspkg.User{}, dbErr)

		m.On("GetAdByID", ctx, ad.ID).Return(ad, nil)

		_, err := usecase.NewGetAdByIDFunc(m, identityClient)(ctx, ad.ID)

		assert.ErrorIs(t, err, dbErr)
	})
}
