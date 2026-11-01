package http

import (
	"encoding/json"
	"errors"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/util"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type createAdBody struct {
	Title string `json:"title"`

	// PriceCents is being deprecated in favor of Price,
	// but is still supported for backward compatibility.
	// Pointer is used to distinguish between "not provided" and "provided as 0".
	PriceCents *int64 `json:"price_cents,omitempty"`

	// Price is a string representation of the new price field in float.
	// Price replaces PriceCents.
	// Pointer is used to distinguish between "not provided" and "provided as 0".
	Price *json.Number `json:"price,omitempty"`

	PhotoURL string `json:"photo_url"`
}

func (b createAdBody) priceCents() (int64, error) {
	if (b.Price == nil) == (b.PriceCents == nil) {
		return 0, errors.New("exactly one of price or price_cents must be provided")
	}

	if b.PriceCents != nil {
		if *b.PriceCents < 0 {
			return 0, errors.New("price_cents cannot be negative")
		}

		return *b.PriceCents, nil
	}

	price, err := util.DecimalToCents((*b.Price).String())
	if err != nil {
		return 0, err
	}

	return price, nil
}

type createAdResponse struct {
	ID int64 `json:"id"`
}

// handlerCreateAd serves POST /ads.
func handlerCreateAd(createAd usecase.CreateAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body createAdBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		priceCents, err := body.priceCents()
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		adID, err := createAd(r.Context(), usecase.CreateAdRequest{
			Title:      body.Title,
			PriceCents: priceCents,
			PhotoURL:   body.PhotoURL,
			OwnerID:    middleware.UserIDFromRequest(r),
		})
		if err != nil {
			if errors.Is(err, adsdao.ErrPublicationLimitExceeded) {
				rw.WriteHeader(http.StatusTooManyRequests)
				return
			}

			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(createAdResponse{ID: int64(adID)})
	}
}
