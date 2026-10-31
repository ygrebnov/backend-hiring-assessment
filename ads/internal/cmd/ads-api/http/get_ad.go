package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

type getAdResponse struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`

	// PriceCents is the price of the ad in cents.
	// PriceCents is being deprecated in favor of the new Price field,
	// which is a decimal representation of the price.
	// During the transition period, both fields will be present in the response.
	PriceCents int64 `json:"price_cents"`

	// Price is the price of the ad in decimal format.
	// Price is a new field that replaces PriceCents.
	// During the transition period, both fields will be present in the response.
	Price float64 `json:"price"`

	PhotoURL  string    `json:"photo_url"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
}

func adToGetAdResponse(a domain.Ad) getAdResponse {
	return getAdResponse{
		ID:         int64(a.ID),
		Title:      a.Title,
		PriceCents: a.Price,
		Price:      float64(a.Price) / 100,
		PhotoURL:   a.PhotoURL,
		UserID:     a.UserID.String(),
		UserName:   a.UserName,
		CreatedAt:  a.CreatedAt.UTC(),
	}
}

func handlerGetAd(getAd usecase.GetAdByIDFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		ad, err := getAd(r.Context(), adID)
		if err != nil {
			if errors.Is(err, adsdao.ErrNotFound) {
				rw.WriteHeader(http.StatusNotFound)
			} else {
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(adToGetAdResponse(ad))
	}
}
