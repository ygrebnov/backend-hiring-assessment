package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/client/identity"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/http"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/httpserver"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("unable to load configuration: %w", err)
	}

	adDAO, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("unable to initialize ads database: %w", err)
	}
	defer func() {
		if err := adDAO.Close(); err != nil {
			slog.Error("unable to close the database", "error", err)
		}
	}()

	identityClient := identity.New(cfg.IdentityURL, cfg.IdentityTimeout)

	createAdParams := usecase.CreateAdParams{
		PublicationLimit:  cfg.AdPublicationLimit,
		PublicationWindow: cfg.AdPublicationWindow,
	}

	router := http.NewRouter(http.Usecases{
		CreateAd:  usecase.NewCreateAdFunc(adDAO, createAdParams),
		GetAd:     usecase.NewGetAdByIDFunc(adDAO, identityClient),
		ListAds:   usecase.NewListAdsFunc(adDAO),
		DeleteAd:  usecase.NewDeleteAdByIDFunc(adDAO),
		ImportAds: usecase.NewImportAdsFunc(adDAO),
	})

	if errRun := httpserver.Run(ctx, cfg.HTTPAddr, router); errRun != nil {
		return fmt.Errorf("unable to run HTTP server: %w", errRun)
	}

	return nil
}
