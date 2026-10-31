package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// ImportAdsFunc imports ads owned by the calling user.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// ImportAdsRequest describes the ads to import.
	// OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	ImportAdsRequest struct {
		Ads            []CreateAdRequest
		OwnerID        domain.UserID
		IdempotencyKey string
	}

	importedAds struct {
		adDAO adsdao.DAO
	}
)

// NewImportAdsFunc returns an ImportAdsFunc backed by the given DAO.
func NewImportAdsFunc(adDAO adsdao.DAO) ImportAdsFunc {
	return importedAds{adDAO: adDAO}.importAds
}

func (uc importedAds) importAds(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error) {
	ads := make([]adsdao.CreateAdRequest, len(req.Ads))

	for i, ad := range req.Ads {
		ads[i] = adsdao.CreateAdRequest{
			Title:      ad.Title,
			PriceCents: ad.PriceCents,
			PhotoURL:   ad.PhotoURL,
			OwnerID:    req.OwnerID,
		}
	}

	var requestHash string

	// Without an idempotency key, the DAO performs an atomic import
	// without deduplication, so no request hash is needed.
	if req.IdempotencyKey != "" {
		payload, err := json.Marshal(ads)
		if err != nil {
			return nil, fmt.Errorf("unable to serialize import request: %w", err)
		}

		hash := sha256.Sum256(payload)
		requestHash = hex.EncodeToString(hash[:])
	}

	adIDs, err := uc.adDAO.ImportAds(ctx, adsdao.ImportAdsRequest{
		OwnerID:        req.OwnerID,
		Ads:            ads,
		IdempotencyKey: req.IdempotencyKey,
		RequestHash:    requestHash,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}

	return adIDs, nil
}
