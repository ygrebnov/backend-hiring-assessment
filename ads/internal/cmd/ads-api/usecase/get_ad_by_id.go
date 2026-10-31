package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	userspkg "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
)

type (
	// GetAdByIDFunc returns a single ad enriched with its owner's username.
	// It returns an error wrapping adsdao.ErrNotFound when the ad does not exist.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (domain.Ad, error)

	getAdByID struct {
		adDAO          adsdao.DAO
		identityClient userspkg.UserClient
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc backed by the given DAO.
func NewGetAdByIDFunc(adDAO adsdao.DAO, identityClient userspkg.UserClient) GetAdByIDFunc {
	return getAdByID{adDAO: adDAO, identityClient: identityClient}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	ad, err := uc.adDAO.GetAdByID(ctx, id)
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	user, err := uc.identityClient.GetUserByID(ctx, ad.UserID)
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get owner of ad %s: %w", ad.UserID, err)
	}

	ad.UserName = user.Nickname

	return ad, nil
}
