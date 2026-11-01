package usecase

import (
	"context"
	"fmt"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// CreateAdFunc creates an ad owned by the calling user.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create.
	// OwnerID is the authenticated user: a caller can never create an ad
	// on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	// CreateAdParams configures the publication quota.
	CreateAdParams struct {
		PublicationLimit  int
		PublicationWindow time.Duration
	}

	createAd struct {
		adDAO  adsdao.DAO
		params CreateAdParams
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given DAO.
func NewCreateAdFunc(adDAO adsdao.DAO, params CreateAdParams) CreateAdFunc {
	return createAd{
		adDAO:  adDAO,
		params: params,
	}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	adID, err := uc.adDAO.CreateAdWithQuota(ctx, adsdao.CreateAdWithQuotaRequest{
		CreateAdRequest: adsdao.CreateAdRequest{
			Title:      req.Title,
			PriceCents: req.PriceCents,
			PhotoURL:   req.PhotoURL,
			OwnerID:    req.OwnerID,
		},
		Limit:  uc.params.PublicationLimit,
		Window: uc.params.PublicationWindow,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return adID, nil
}
