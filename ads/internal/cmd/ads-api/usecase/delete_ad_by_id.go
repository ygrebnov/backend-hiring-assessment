package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// DeleteAdByIDFunc deletes an ad owned by userID.
	// It returns dao.ErrNotFound if the ad does not exist,
	// or dao.ErrForbidden if it belongs to another user.
	DeleteAdByIDFunc func(ctx context.Context, id domain.AdID, userID domain.UserID) error

	deleteAdByID struct {
		adDAO adsdao.DAO
	}
)

// NewDeleteAdByIDFunc returns a DeleteAdByIDFunc backed by the given DAO.
func NewDeleteAdByIDFunc(adDAO adsdao.DAO) DeleteAdByIDFunc {
	return deleteAdByID{adDAO: adDAO}.deleteAdByID
}

func (uc deleteAdByID) deleteAdByID(
	ctx context.Context,
	id domain.AdID,
	userID domain.UserID,
) error {
	if err := uc.adDAO.DeleteAdByID(ctx, id, userID); err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", id, err)
	}

	return nil
}
