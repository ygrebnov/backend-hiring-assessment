package dao

import (
	"context"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// Store reads and writes ads. It is implemented both by the DAO, where every call is its own transaction, and by a Tx opened from it.
type Store interface {
	// CreateAd inserts an ad and returns its generated identifier.
	CreateAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error)
	// GetAdByID returns a single ad, or ErrNotFound.
	GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error)
	// DeleteAdByID deletes a single ad, or returns ErrNotFound.
	DeleteAdByID(ctx context.Context, id domain.AdID, userID domain.UserID) error
	// SearchAds returns all ads matching the filters.
	SearchAds(ctx context.Context, req SearchAdsRequest) ([]domain.Ad, error)
}

// DAO stores and retrieves ads.
type DAO interface {
	Store
	// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
	Begin(ctx context.Context) (Tx, error)

	// ImportAds atomically inserts all ads and returns their generated IDs.
	// If any insert fails, no ads are persisted.
	ImportAds(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// CreateAdWithQuota inserts an ad if the user has not exceeded their publication limit,
	// and returns its generated identifier.
	CreateAdWithQuota(ctx context.Context, req CreateAdWithQuotaRequest) (domain.AdID, error)
}

// Tx is a Store scoped to an opened transaction.
type Tx interface {
	Store
	// Commit applies every change made through the transaction.
	Commit() error
	// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
	Rollback() error
}

// SearchAdsRequest describes an ad search. Nil pointers and empty strings mean "no filter on this field".
type SearchAdsRequest struct {
	OwnerID       domain.UserID
	MinPriceCents int64
	MaxPriceCents int64
	Title         string
}

// CreateAdRequest carries the data needed to create an ad.
type CreateAdRequest struct {
	Title      string
	PriceCents int64
	PhotoURL   string
	OwnerID    domain.UserID
}

// CreateAdWithQuotaRequest carries the data needed to create an ad with a publication limit.
type CreateAdWithQuotaRequest struct {
	CreateAdRequest
	Limit  int
	Window time.Duration
}

// ImportAdsRequest carries the data needed to import multiple ads atomically and
// (optionally) idempotently.
type ImportAdsRequest struct {
	OwnerID        domain.UserID
	Ads            []CreateAdRequest
	IdempotencyKey string
	RequestHash    string
}

const (
	// ErrNotFound is returned when no ad matches the requested identifier.
	ErrNotFound sentinelError = "ad not found"
	// ErrUnknownOwner is returned when an ad references a user that does not exist.
	ErrUnknownOwner sentinelError = "unknown owner"
	// ErrForbidden is returned when a user attempts to modify an ad they do not own.
	ErrForbidden sentinelError = "forbidden"
	// ErrIdempotencyConflict is returned when an idempotency key is reused
	// with a different request payload.
	ErrIdempotencyConflict sentinelError = "idempotency key conflict"
	// ErrPublicationLimitExceeded is returned when a user attempts to create an ad
	// but has already reached their publication limit within the configured time window.
	ErrPublicationLimitExceeded sentinelError = "publication limit exceeded"
)

type sentinelError string

// Error implements the error interface.
func (e sentinelError) Error() string { return string(e) }
