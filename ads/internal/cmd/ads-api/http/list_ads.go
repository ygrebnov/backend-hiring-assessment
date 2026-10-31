package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

type listAdsItem struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`

	// PriceCents is the price of the ad in cents.
	// PriceCents is being deprecated in favor of the new Price field,
	// which is a decimal representation of the price.
	// During the transition period, both fields will be present in the response.
	PriceCents int64 `json:"price_cents"`

	// Price is the price of the ad in decimal format.
	// Price is a new field that replaces PriceCents.
	// During the transition period, both fields will be present in the response.
	Price float64 `json:"price"`

	PhotoURL   string    `json:"photo_url"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type listAdsResponse struct {
	Ads []listAdsItem `json:"ads"`
}

func adToListAdsItem(a domain.Ad) listAdsItem {
	return listAdsItem{
		ID:         int64(a.ID),
		Title:      a.Title,
		PriceCents: a.Price,
		Price:      float64(a.Price) / 100,
		PhotoURL:   a.PhotoURL,
		UserID:     a.UserID.String(),
		CreatedAt:  a.CreatedAt.UTC(),
	}
}

func handlerListAds(listAds usecase.ListAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		req := usecase.ListAdsRequest{
			Title: q.Get("title"),
		}

		if raw := q.Get("user_id"); raw != "" {
			ownerID := domain.UserID(raw)
			req.OwnerID = ownerID
		}

		if raw := q.Get("min_price_cents"); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			req.MinPriceCents = v
		}

		if raw := q.Get("max_price_cents"); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			req.MaxPriceCents = v
		}

		ads, err := listAds(r.Context(), req)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		items := make([]listAdsItem, 0, len(ads))
		for _, a := range ads {
			items = append(items, adToListAdsItem(a))
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(listAdsResponse{Ads: items})
	}
}
