package http

import (
	"errors"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func handlerDeleteAd(deleteAd usecase.DeleteAdByIDFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		userID := middleware.UserIDFromRequest(r)

		if errDelete := deleteAd(r.Context(), adID, userID); errDelete != nil {
			switch {
			case errors.Is(errDelete, adsdao.ErrForbidden):
				rw.WriteHeader(http.StatusForbidden)
			case errors.Is(errDelete, adsdao.ErrNotFound):
				rw.WriteHeader(http.StatusNotFound)
			default:
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.WriteHeader(http.StatusNoContent)
	}
}
