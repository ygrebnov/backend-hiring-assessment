package identity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/client/identity"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	userspkg "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

func TestAdapterGetUserByID(t *testing.T) {
	ctx := context.Background()
	userID := model.UserID("user-1")

	t.Run("nominal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/v1/users/user-1", r.URL.Path)

			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"user_id":"user-1","nickname":"alice"}`))
		}))
		defer srv.Close()

		client := identity.New(srv.URL, time.Second)

		got, err := client.GetUserByID(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, userspkg.User{
			UserID:   userID,
			Nickname: "alice",
		}, got)
	})

	t.Run("user not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		client := identity.New(srv.URL, time.Second)

		_, err := client.GetUserByID(ctx, userID)

		require.Error(t, err)
		assert.ErrorIs(t, err, usersapi.ErrNotFound)
	})

	t.Run("unexpected response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := identity.New(srv.URL, time.Second)

		_, err := client.GetUserByID(ctx, userID)

		require.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			_, _ = rw.Write([]byte(`{invalid}`))
		}))
		defer srv.Close()

		client := identity.New(srv.URL, time.Second)

		_, err := client.GetUserByID(ctx, userID)

		require.Error(t, err)
	})
}

func TestAdapterListUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/v1/users", r.URL.Path)

			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`[
				{"user_id":"user-1","nickname":"alice"},
				{"user_id":"user-2","nickname":"bob"}
			]`))
		}))
		defer srv.Close()

		client := identity.New(srv.URL, time.Second)

		got, err := client.ListUsers(ctx)

		require.NoError(t, err)
		assert.Equal(t, []userspkg.User{
			{UserID: "user-1", Nickname: "alice"},
			{UserID: "user-2", Nickname: "bob"},
		}, got)
	})

	t.Run("error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := identity.New(srv.URL, time.Second)

		_, err := client.ListUsers(ctx)

		require.Error(t, err)
	})
}
