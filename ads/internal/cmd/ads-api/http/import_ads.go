package http

import (
	"encoding/json"
	"errors"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type importAdsBody struct {
	Ads []createAdBody `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body importAdsBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		ownerID := middleware.UserIDFromRequest(r)

		ads := make([]usecase.CreateAdRequest, len(body.Ads))

		for i, item := range body.Ads {
			priceCents, err := item.priceCents()
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			ads[i] = usecase.CreateAdRequest{
				Title:      item.Title,
				PriceCents: priceCents,
				PhotoURL:   item.PhotoURL,
				OwnerID:    ownerID,
			}
		}

		adIDs, err := importAds(r.Context(), usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: r.Header.Get("Idempotency-Key"),
		})
		if err != nil {
			if errors.Is(err, adsdao.ErrIdempotencyConflict) {
				rw.WriteHeader(http.StatusConflict)
				return
			}

			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		ids := make([]int64, len(adIDs))
		for i, id := range adIDs {
			ids[i] = int64(id)
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(importAdsResponse{IDs: ids})
	}
}
