package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds everything the API needs to start.
type Config struct {
	// HTTPAddr is the address the API listens on, for example ":8080".
	HTTPAddr string
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// IdentityURL is the base URL of the identity service.
	IdentityURL string
	// IdentityTimeout bounds every call to the identity service.
	IdentityTimeout time.Duration
	// AdPublicationLimit is the maximum number of ads a user can publish per time window.
	AdPublicationLimit int
	// AdPublicationWindow is the time window for the ad publication limit.
	AdPublicationWindow time.Duration
}

// Load reads and validates the configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    text("HTTP_ADDR", ":8080"),
		DatabaseURL: text("DATABASE_URL", "postgres://ads:ads@localhost:5432/ads?sslmode=disable"),
		IdentityURL: text("IDENTITY_URL", "http://localhost:8081"),
	}

	var err error

	cfg.IdentityTimeout, err = positiveDuration("IDENTITY_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg.AdPublicationLimit, err = positiveIntEnv("AD_PUBLICATION_LIMIT", 5)
	if err != nil {
		return Config{}, err
	}

	cfg.AdPublicationWindow, err = positiveDuration("AD_PUBLICATION_WINDOW", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func text(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func positiveIntEnv(name string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be positive", name)
	}

	return value, nil
}

func positiveDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be positive", name)
	}

	return value, nil
}
