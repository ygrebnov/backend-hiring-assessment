package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var adCols = []string{"id", "title", "price_cents", "photo_url", "user_id", "created_at"}

func newMockDAO(t *testing.T) (*DAO, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &DAO{db: db}, mock
}

func TestSearchAds(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns matching ads", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(1), "bike", int64(5000), "https://example.com/bike.jpg", "user-1", createdAt).
			AddRow(int64(2), "scooter", int64(12000), "https://example.com/scooter.jpg", "user-2", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnRows(rows)

		got, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{})

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.AdID(1), got[0].ID)
		assert.Equal(t, "bike", got[0].Title)
		assert.Equal(t, int64(5000), got[0].Price)
		assert.Equal(t, domain.AdID(2), got[1].ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns nil when no ads match", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "bike").
			WillReturnRows(sqlmock.NewRows(adCols))

		got, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{Title: "bike"})

		require.NoError(t, err)
		assert.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnError(boom)

		_, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{})

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns ad on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(42), "mountain bike", int64(15000), "https://example.com/bike.jpg", "user-1", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(42)).
			WillReturnRows(rows)

		got, err := dao.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(42), got.ID)
		assert.Equal(t, "mountain bike", got.Title)
		assert.Equal(t, int64(15000), got.Price)
		assert.Equal(t, domain.UserID("user-1"), got.UserID)
		assert.Equal(t, createdAt, got.CreatedAt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrNotFound when no row", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(99)).
			WillReturnRows(sqlmock.NewRows(adCols))

		_, err := dao.GetAdByID(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")
		mock.ExpectQuery("SELECT").
			WithArgs(int64(1)).
			WillReturnError(boom)

		_, err := dao.GetAdByID(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCreateAd(t *testing.T) {
	ctx := context.Background()
	req := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("returns generated ID on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

		got, err := dao.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUnknownOwner on FK violation", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		pgErr := &pgconn.PgError{Code: "23503"}
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(pgErr)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on other failures", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(boom)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, adsdao.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeleteAdByID(t *testing.T) {
	ctx := context.Background()
	userID := domain.UserID("user-1")

	t.Run("returns nil on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectExec("DELETE FROM ads WHERE id").
			WithArgs(int64(42), string(userID)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := dao.DeleteAdByID(ctx, 42, userID)

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrNotFound when ad does not exist", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectExec("DELETE FROM ads WHERE id").
			WithArgs(int64(99), string(userID)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectQuery("SELECT 1 FROM ads WHERE id").
			WithArgs(int64(99)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}))

		err := dao.DeleteAdByID(ctx, 99, userID)

		require.ErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrForbidden when ad belongs to another user", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectExec("DELETE FROM ads WHERE id").
			WithArgs(int64(42), string(userID)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectQuery("SELECT 1 FROM ads WHERE id").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))

		err := dao.DeleteAdByID(ctx, 42, userID)

		require.ErrorIs(t, err, adsdao.ErrForbidden)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on delete failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")

		mock.ExpectExec("DELETE FROM ads WHERE id").
			WithArgs(int64(1), string(userID)).
			WillReturnError(boom)

		err := dao.DeleteAdByID(ctx, 1, userID)

		require.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when rows affected is unavailable", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("rows affected unavailable")

		mock.ExpectExec("DELETE FROM ads WHERE id").
			WithArgs(int64(42), string(userID)).
			WillReturnResult(sqlmock.NewErrorResult(boom))

		err := dao.DeleteAdByID(ctx, 42, userID)

		require.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when existence check fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("database unavailable")

		mock.ExpectExec("DELETE FROM ads WHERE id").
			WithArgs(int64(42), string(userID)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectQuery("SELECT 1 FROM ads WHERE id").
			WithArgs(int64(42)).
			WillReturnError(boom)

		err := dao.DeleteAdByID(ctx, 42, userID)

		require.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTransaction(t *testing.T) {
	ctx := context.Background()
	req := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("commits every call made on the transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		first, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)
		second, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())
		assert.Equal(t, domain.AdID(1), first)
		assert.Equal(t, domain.AdID(2), second)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back without committing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the transaction cannot be opened", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectBegin().WillReturnError(boom)

		tx, err := dao.Begin(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback after commit does nothing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects a nested transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, nestedErr := tx.(*DAO).Begin(ctx)

		require.Error(t, nestedErr)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses commit and rollback outside a transaction", func(t *testing.T) {
		dao, _ := newMockDAO(t)

		require.Error(t, dao.Commit())
		require.Error(t, dao.Rollback())
	})
}

func TestImportAds(t *testing.T) {
	ctx := context.Background()

	ownerID := domain.UserID("00000000-0000-0000-0000-000000000001")

	ads := []adsdao.CreateAdRequest{
		{
			Title:      "mountain bike",
			PriceCents: 15050,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    ownerID,
		},
		{
			Title:      "scooter",
			PriceCents: 12061,
			PhotoURL:   "https://example.com/scooter.jpg",
			OwnerID:    ownerID,
		},
	}

	expectInsert := func(mock sqlmock.Sqlmock, ad adsdao.CreateAdRequest, id int64) {
		mock.ExpectQuery("INSERT INTO ads").
			WithArgs(ad.Title, ad.PriceCents, ad.PhotoURL, string(ad.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	}

	t.Run("imports all ads atomically without idempotency key", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectBegin()
		expectInsert(mock, ads[0], 101)
		expectInsert(mock, ads[1], 102)
		mock.ExpectCommit()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
			Ads:     ads,
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{101, 102}, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("imports all ads and stores idempotency result", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectBegin()

		mock.ExpectQuery("INSERT INTO ad_imports").
			WithArgs(string(ownerID), "import-123", "hash-abc").
			WillReturnRows(
				sqlmock.NewRows([]string{"idempotency_key"}).
					AddRow("import-123"),
			)

		expectInsert(mock, ads[0], 101)
		expectInsert(mock, ads[1], 102)

		mock.ExpectExec("UPDATE ad_imports").
			WithArgs(
				string(ownerID),
				"import-123",
				adIDs{101, 102},
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectCommit()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            ads,
			IdempotencyKey: "import-123",
			RequestHash:    "hash-abc",
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{101, 102}, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns previous IDs when idempotency key and payload match", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectBegin()

		mock.ExpectQuery("INSERT INTO ad_imports").
			WithArgs(string(ownerID), "import-123", "hash-abc").
			WillReturnRows(sqlmock.NewRows([]string{"idempotency_key"}))

		mock.ExpectQuery("SELECT request_hash, ad_ids").
			WithArgs(string(ownerID), "import-123").
			WillReturnRows(
				sqlmock.NewRows([]string{"request_hash", "ad_ids"}).
					AddRow("hash-abc", "{101,102}"),
			)

		mock.ExpectRollback()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            ads,
			IdempotencyKey: "import-123",
			RequestHash:    "hash-abc",
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{101, 102}, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns conflict when idempotency key is reused with different payload", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectBegin()

		mock.ExpectQuery("INSERT INTO ad_imports").
			WithArgs(string(ownerID), "import-123", "hash-new").
			WillReturnRows(sqlmock.NewRows([]string{"idempotency_key"}))

		mock.ExpectQuery("SELECT request_hash, ad_ids").
			WithArgs(string(ownerID), "import-123").
			WillReturnRows(
				sqlmock.NewRows([]string{"request_hash", "ad_ids"}).
					AddRow("hash-original", "{101,102}"),
			)

		mock.ExpectRollback()

		_, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            ads,
			IdempotencyKey: "import-123",
			RequestHash:    "hash-new",
		})

		require.ErrorIs(t, err, adsdao.ErrIdempotencyConflict)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back all inserts when one ad fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		boom := errors.New("insert failed")

		mock.ExpectBegin()
		expectInsert(mock, ads[0], 101)

		mock.ExpectQuery("INSERT INTO ads").
			WithArgs(
				ads[1].Title,
				ads[1].PriceCents,
				ads[1].PhotoURL,
				string(ads[1].OwnerID),
			).
			WillReturnError(boom)

		mock.ExpectRollback()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
			Ads:     ads,
		})

		require.ErrorIs(t, err, boom)
		assert.Nil(t, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back when claiming idempotency key fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		boom := errors.New("database unavailable")

		mock.ExpectBegin()

		mock.ExpectQuery("INSERT INTO ad_imports").
			WithArgs(string(ownerID), "import-123", "hash-abc").
			WillReturnError(boom)

		mock.ExpectRollback()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            ads,
			IdempotencyKey: "import-123",
			RequestHash:    "hash-abc",
		})

		require.ErrorIs(t, err, boom)
		assert.Nil(t, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back when saving idempotency result fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		boom := errors.New("update failed")

		mock.ExpectBegin()

		mock.ExpectQuery("INSERT INTO ad_imports").
			WithArgs(string(ownerID), "import-123", "hash-abc").
			WillReturnRows(
				sqlmock.NewRows([]string{"idempotency_key"}).
					AddRow("import-123"),
			)

		expectInsert(mock, ads[0], 101)
		expectInsert(mock, ads[1], 102)

		mock.ExpectExec("UPDATE ad_imports").
			WithArgs(
				string(ownerID),
				"import-123",
				"{101,102}",
			).
			WillReturnError(boom)

		mock.ExpectRollback()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            ads,
			IdempotencyKey: "import-123",
			RequestHash:    "hash-abc",
		})

		require.ErrorIs(t, err, boom)
		assert.Nil(t, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when transaction commit fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		boom := errors.New("commit failed")

		mock.ExpectBegin()
		expectInsert(mock, ads[0], 101)
		expectInsert(mock, ads[1], 102)

		mock.ExpectCommit().WillReturnError(boom)

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
			Ads:     ads,
		})

		require.ErrorIs(t, err, boom)
		assert.Nil(t, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("accepts empty import without idempotency key", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectBegin()
		mock.ExpectCommit()

		ids, err := dao.ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
		})

		require.NoError(t, err)
		assert.Empty(t, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects import inside existing transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.(*DAO).ImportAds(ctx, adsdao.ImportAdsRequest{
			OwnerID: ownerID,
			Ads:     ads,
		})

		require.Error(t, err)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
