package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	const (
		httpAddr    = "HTTP_ADDR"
		databaseURL = "DATABASE_URL"
		identityURL = "IDENTITY_URL"
		timeout     = "IDENTITY_TIMEOUT"
		limit       = "AD_PUBLICATION_LIMIT"
		window      = "AD_PUBLICATION_WINDOW"
	)

	envVars := []string{
		httpAddr,
		databaseURL,
		identityURL,
		timeout,
		limit,
		window,
	}

	// Ensure the tests do not depend on the developer's environment.
	// Restore all original values when the test finishes.
	for _, name := range envVars {
		unsetenv(t, name)
	}

	t.Run("uses defaults when variables are absent", func(t *testing.T) {
		cfg, err := Load()

		require.NoError(t, err)
		assert.Equal(t, Config{
			HTTPAddr:            ":8080",
			DatabaseURL:         "postgres://ads:ads@localhost:5432/ads?sslmode=disable",
			IdentityURL:         "http://localhost:8081",
			IdentityTimeout:     2 * time.Second,
			AdPublicationLimit:  5,
			AdPublicationWindow: 24 * time.Hour,
		}, cfg)
	})

	t.Run("reads environment overrides", func(t *testing.T) {
		t.Setenv(httpAddr, ":9090")
		t.Setenv(databaseURL, "postgres://test:test@localhost:5433/test")
		t.Setenv(identityURL, "http://identity:8081")
		t.Setenv(timeout, "5s")
		t.Setenv(limit, "10")
		t.Setenv(window, "12h")

		cfg, err := Load()

		require.NoError(t, err)
		assert.Equal(t, Config{
			HTTPAddr:            ":9090",
			DatabaseURL:         "postgres://test:test@localhost:5433/test",
			IdentityURL:         "http://identity:8081",
			IdentityTimeout:     5 * time.Second,
			AdPublicationLimit:  10,
			AdPublicationWindow: 12 * time.Hour,
		}, cfg)
	})

	t.Run("uses fallback for empty text variables", func(t *testing.T) {
		t.Setenv(httpAddr, "")
		t.Setenv(databaseURL, "")
		t.Setenv(identityURL, "")

		cfg, err := Load()

		require.NoError(t, err)
		assert.Equal(t, ":8080", cfg.HTTPAddr)
		assert.Equal(t, "postgres://ads:ads@localhost:5432/ads?sslmode=disable", cfg.DatabaseURL)
		assert.Equal(t, "http://localhost:8081", cfg.IdentityURL)
	})

	t.Run("rejects invalid identity timeout", func(t *testing.T) {
		t.Setenv(timeout, "invalid")

		cfg, err := Load()

		require.Error(t, err)
		assert.ErrorContains(t, err, timeout)
		assert.Equal(t, Config{}, cfg)
	})

	t.Run("rejects invalid publication limit", func(t *testing.T) {
		t.Setenv(limit, "invalid")

		cfg, err := Load()

		require.Error(t, err)
		assert.ErrorContains(t, err, limit)
		assert.Equal(t, Config{}, cfg)
	})

	t.Run("rejects invalid publication window", func(t *testing.T) {
		t.Setenv(window, "invalid")

		cfg, err := Load()

		require.Error(t, err)
		assert.ErrorContains(t, err, window)
		assert.Equal(t, Config{}, cfg)
	})

	t.Run("rejects zero publication limit", func(t *testing.T) {
		t.Setenv(limit, "0")

		cfg, err := Load()

		require.Error(t, err)
		assert.ErrorContains(t, err, limit)
		assert.Equal(t, Config{}, cfg)
	})

	t.Run("rejects negative publication window", func(t *testing.T) {
		t.Setenv(window, "-1h")

		cfg, err := Load()

		require.Error(t, err)
		assert.ErrorContains(t, err, window)
		assert.Equal(t, Config{}, cfg)
	})
}

func TestPositiveIntEnv(t *testing.T) {
	const name = "TEST_POSITIVE_INT"

	tests := []struct {
		name    string
		value   string
		set     bool
		want    int
		wantErr bool
	}{
		{
			name: "uses fallback when absent",
			want: 5,
		},
		{
			name:  "accepts positive integer",
			value: "10",
			set:   true,
			want:  10,
		},
		{
			name:  "accepts one",
			value: "1",
			set:   true,
			want:  1,
		},
		{
			name:    "rejects zero",
			value:   "0",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects negative integer",
			value:   "-5",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects malformed integer",
			value:   "abc",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects fractional number",
			value:   "1.5",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects empty value",
			value:   "",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects overflow",
			value:   "999999999999999999999999999999999",
			set:     true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(name, tt.value)
			} else {
				unsetenv(t, name)
			}

			got, err := positiveIntEnv(name, 5)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, name)
				assert.Zero(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPositiveDuration(t *testing.T) {
	const name = "TEST_POSITIVE_DURATION"

	tests := []struct {
		name    string
		value   string
		set     bool
		want    time.Duration
		wantErr bool
	}{
		{
			name: "uses fallback when absent",
			want: 24 * time.Hour,
		},
		{
			name:  "accepts hours",
			value: "12h",
			set:   true,
			want:  12 * time.Hour,
		},
		{
			name:  "accepts minutes",
			value: "30m",
			set:   true,
			want:  30 * time.Minute,
		},
		{
			name:  "accepts combined duration",
			value: "1h30m",
			set:   true,
			want:  90 * time.Minute,
		},
		{
			name:  "accepts milliseconds",
			value: "500ms",
			set:   true,
			want:  500 * time.Millisecond,
		},
		{
			name:    "rejects zero",
			value:   "0s",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects negative duration",
			value:   "-1h",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects malformed duration",
			value:   "abc",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects duration without unit",
			value:   "10",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects empty value",
			value:   "",
			set:     true,
			wantErr: true,
		},
		{
			name:    "rejects overflow",
			value:   "999999999999999999999h",
			set:     true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(name, tt.value)
			} else {
				unsetenv(t, name)
			}

			got, err := positiveDuration(name, 24*time.Hour)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, name)
				assert.Zero(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func unsetenv(t *testing.T, name string) {
	t.Helper()

	original, exists := os.LookupEnv(name)

	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(name, original)
		} else {
			_ = os.Unsetenv(name)
		}
	})

	require.NoError(t, os.Unsetenv(name))
}
