package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const adColumns = `id, title, price_cents, photo_url, user_id::text, created_at`

// DAO is a PostgreSQL-backed ad store.
type DAO struct {
	db *sql.DB
	// tx is set when the DAO is scoped to a transaction opened by WithTx.
	tx *sql.Tx
}

// queryRunner is the subset of *sql.DB and *sql.Tx the queries need.
type queryRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// New opens a connection pool to the given DSN and checks that the database is reachable. The caller is responsible for calling Close.
func New(ctx context.Context, dsn string) (*DAO, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("unable to reach database: %w", err)
	}

	return &DAO{db: db}, nil
}

// Close releases the connection pool.
func (d *DAO) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}

	return nil
}

// runner returns the transaction the DAO is scoped to, or the connection pool.
func (d *DAO) runner() queryRunner {
	if d.tx != nil {
		return d.tx
	}

	return d.db
}

// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
func (d *DAO) Begin(ctx context.Context) (adsdao.Tx, error) {
	if d.tx != nil {
		return nil, errors.New("unable to open a nested transaction")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &DAO{db: d.db, tx: tx}, nil
}

// Commit applies every change made through the transaction.
func (d *DAO) Commit() error {
	if d.tx == nil {
		return errors.New("unable to commit outside a transaction")
	}

	if err := d.tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	return nil
}

// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
func (d *DAO) Rollback() error {
	if d.tx == nil {
		return errors.New("unable to roll back outside a transaction")
	}

	if err := d.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("unable to roll back transaction: %w", err)
	}

	return nil
}

// SearchAds returns all ads matching the filters.
func (d *DAO) SearchAds(ctx context.Context, req adsdao.SearchAdsRequest) ([]domain.Ad, error) {
	const query = `
		SELECT ` + adColumns + `
		FROM ads
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = 0 OR price_cents >= $2)
		  AND ($3 = 0 OR price_cents <= $3)
		  AND ($4 = '' OR title ILIKE '%' || $4 || '%')`

	rows, err := d.runner().QueryContext(ctx, query,
		req.OwnerID,
		req.MinPriceCents,
		req.MaxPriceCents,
		req.Title,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to search ads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ads []domain.Ad
	for rows.Next() {
		var ad domain.Ad
		if err := rows.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
			return nil, fmt.Errorf("unable to scan ad: %w", err)
		}

		ads = append(ads, ad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unable to read ads: %w", err)
	}

	return ads, nil
}

// GetAdByID returns a single ad, or dao.ErrNotFound.
func (d *DAO) GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	const query = `SELECT ` + adColumns + ` FROM ads WHERE id = $1`

	ad, err := scanAd(d.runner().QueryRowContext(ctx, query, int64(id)))
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad: %w", err)
	}

	return ad, nil
}

// CreateAd inserts an ad and returns its generated identifier.
func (d *DAO) CreateAd(ctx context.Context, req adsdao.CreateAdRequest) (domain.AdID, error) {
	const query = `
		INSERT INTO ads (title, price_cents, photo_url, user_id)
		VALUES ($1, $2, $3, $4::uuid)
		RETURNING id`

	var id domain.AdID
	err := d.runner().QueryRowContext(ctx, query, req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return 0, fmt.Errorf("unable to create ad: %w", adsdao.ErrUnknownOwner)
		}

		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return id, nil
}

// DeleteAdByID deletes an ad owned by userID.
// It returns ErrNotFound if the ad does not exist,
// or ErrForbidden if the ad belongs to another user.
func (d *DAO) DeleteAdByID(ctx context.Context, id domain.AdID, userID domain.UserID) error {
	const queryDelete = `DELETE FROM ads WHERE id = $1 AND user_id = $2::uuid`

	result, err := d.runner().ExecContext(ctx, queryDelete, int64(id), string(userID))
	if err != nil {
		return fmt.Errorf("unable to delete ad: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to determine rows affected: %w", err)
	}
	if rowsAffected == 0 {
		const queryExists = `SELECT 1 FROM ads WHERE id = $1`
		var exists int
		if errExists := d.runner().
			QueryRowContext(ctx, queryExists, int64(id)).
			Scan(&exists); errExists != nil {
			if errors.Is(errExists, sql.ErrNoRows) {
				return adsdao.ErrNotFound
			}
			return fmt.Errorf("unable to check ad existence: %w", errExists)
		}

		return adsdao.ErrForbidden
	}

	return nil
}

// ImportAds atomically inserts all ads and returns their generated IDs.
// If an idempotency key is provided, successful imports can be retried
// without creating duplicate ads.
func (d *DAO) ImportAds(ctx context.Context, req adsdao.ImportAdsRequest) ([]domain.AdID, error) {
	if d.tx != nil {
		return nil, errors.New("unable to import ads inside an existing transaction")
	}

	tx, err := d.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to begin import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	store := tx.(*DAO)

	if req.IdempotencyKey != "" {
		// Claim the idempotency key before inserting any ads.
		//
		// PostgreSQL's unique constraint serializes concurrent attempts
		// to claim the same (user_id, idempotency_key) pair.
		//
		// A conflicting transaction waits until the first transaction
		// commits or rolls back.
		const claimQuery = `
			INSERT INTO ad_imports (
				user_id, idempotency_key, request_hash, ad_ids
			)
			VALUES ($1::uuid, $2, $3, '{}'::bigint[])
			ON CONFLICT (user_id, idempotency_key) DO NOTHING
			RETURNING idempotency_key`

		var claimedKey string
		err := store.tx.QueryRowContext(
			ctx,
			claimQuery,
			string(req.OwnerID),
			req.IdempotencyKey,
			req.RequestHash,
		).Scan(&claimedKey)

		switch {
		case err == nil:
			// This transaction owns the key and can proceed.

		case errors.Is(err, sql.ErrNoRows):
			// The key already exists. Retrieve the previous result.
			const lookupQuery = `
				SELECT request_hash, ad_ids
				FROM ad_imports
				WHERE user_id = $1::uuid
				  AND idempotency_key = $2`

			var storedHash string
			var storedIDs adIDs

			err = store.tx.QueryRowContext(
				ctx,
				lookupQuery,
				string(req.OwnerID),
				req.IdempotencyKey,
			).Scan(&storedHash, &storedIDs)

			if err != nil {
				return nil, fmt.Errorf("unable to retrieve previous import: %w", err)
			}

			if storedHash != req.RequestHash {
				return nil, adsdao.ErrIdempotencyConflict
			}

			return storedIDs, nil

		default:
			return nil, fmt.Errorf("unable to claim idempotency key: %w", err)
		}
	}

	ids := make([]domain.AdID, 0, len(req.Ads))

	for _, ad := range req.Ads {
		id, err := store.CreateAd(ctx, ad)
		if err != nil {
			return nil, fmt.Errorf("unable to import ad: %w", err)
		}

		ids = append(ids, id)
	}

	if req.IdempotencyKey != "" {
		const updateQuery = `
			UPDATE ad_imports
			SET ad_ids = $3::bigint[]
			WHERE user_id = $1::uuid
	  		AND idempotency_key = $2`

		_, err = store.tx.ExecContext(
			ctx,
			updateQuery,
			string(req.OwnerID),
			req.IdempotencyKey,
			adIDs(ids),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to store import result: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("unable to commit import: %w", err)
	}

	return ids, nil
}

// CreateAdWithQuota atomically creates an ad if the user has not reached
// their publication limit within the configured rolling window.
//
// A per-user advisory lock serializes concurrent publication attempts,
// preventing multiple requests from exceeding the quota.
//
// Bulk imports use CreateAd directly and do not consume publication quota.
func (d *DAO) CreateAdWithQuota(
	ctx context.Context,
	req adsdao.CreateAdWithQuotaRequest,
) (domain.AdID, error) {
	if d.tx != nil {
		return 0, errors.New("unable to create ad with quota inside an existing transaction")
	}

	if req.Limit <= 0 {
		return 0, errors.New("publication limit must be positive")
	}

	if req.Window <= 0 {
		return 0, errors.New("publication window must be positive")
	}

	tx, err := d.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to begin publication: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	store := tx.(*DAO)

	// Serialize publication attempts for the same user.
	// A hash collision only causes unnecessary serialization
	// between different users; it cannot bypass the quota.
	const lockQuery = `
		SELECT pg_advisory_xact_lock(
			hashtextextended($1::text, 0)
		)`

	if _, err := store.tx.ExecContext(
		ctx,
		lockQuery,
		string(req.OwnerID),
	); err != nil {
		return 0, fmt.Errorf("unable to acquire publication lock: %w", err)
	}

	// Count publications in the rolling window.
	// clock_timestamp() reflects the actual time after acquiring the lock,
	// unlike now(), which is fixed at the transaction start.
	const countQuery = `
		SELECT COUNT(*)
		FROM ad_publications
		WHERE user_id = $1::uuid
		  AND published_at > clock_timestamp() -
		      ($2::double precision * INTERVAL '1 second')`

	var count int

	if err := store.tx.QueryRowContext(
		ctx,
		countQuery,
		string(req.OwnerID),
		req.Window.Seconds(),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("unable to count publications: %w", err)
	}

	if count >= req.Limit {
		return 0, adsdao.ErrPublicationLimitExceeded
	}

	// Insert the ad within the same transaction.
	id, err := store.CreateAd(ctx, req.CreateAdRequest)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	// Record quota consumption within the same transaction.
	const publicationQuery = `
		INSERT INTO ad_publications (user_id, published_at)
		VALUES ($1::uuid, clock_timestamp())`

	if _, err := store.tx.ExecContext(
		ctx,
		publicationQuery,
		string(req.OwnerID),
	); err != nil {
		return 0, fmt.Errorf("unable to record publication: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("unable to commit publication: %w", err)
	}

	return id, nil
}

func scanAd(row *sql.Row) (domain.Ad, error) {
	var ad domain.Ad
	if err := row.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ad{}, adsdao.ErrNotFound
		}

		return domain.Ad{}, err
	}

	return ad, nil
}
