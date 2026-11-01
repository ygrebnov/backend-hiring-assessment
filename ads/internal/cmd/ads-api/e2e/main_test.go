//go:build e2e

package e2e

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	db    *sql.DB
	api   *apiClient
	users []userResponse
)

type userResponse struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
}

func TestMain(m *testing.M) {
	databaseURL := os.Getenv("ADS_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://ads:ads@localhost:15432/ads?sslmode=disable"
	}

	var err error

	db, err = sql.Open("pgx", databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E database setup failed: %v\n", err)
		os.Exit(1)
	}

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "E2E database unavailable: %v\n", err)
		os.Exit(1)
	}

	defer db.Close()

	adsURL := strings.TrimRight(os.Getenv("ADS_API_URL"), "/")
	if adsURL == "" {
		adsURL = "http://localhost:18080"
	}

	usersURL := strings.TrimRight(os.Getenv("USERS_API_URL"), "/")
	if usersURL == "" {
		usersURL = "http://localhost:18081"
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	api = &apiClient{
		baseURL:    adsURL,
		httpClient: httpClient,
	}

	if err := waitForAPI(adsURL, 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "E2E setup failed: %v\n", err)
		os.Exit(1)
	}

	if err := waitForAPI(usersURL, 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "E2E setup failed: %v\n", err)
		os.Exit(1)
	}

	users, err = fetchUsers(httpClient, usersURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E setup failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func waitForAPI(baseURL string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("API at %s did not become healthy within %s", baseURL, timeout)
}

func fetchUsers(client *http.Client, baseURL string) ([]userResponse, error) {
	resp, err := client.Get(baseURL + "/v1/users")
	if err != nil {
		return nil, fmt.Errorf("fetch users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch users: unexpected status %d", resp.StatusCode)
	}

	var result []userResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("users service returned no users")
	}

	return result, nil
}

func userByNickname(t *testing.T, nickname string) userResponse {
	t.Helper()

	for _, user := range users {
		if user.Nickname == nickname {
			return user
		}
	}

	t.Fatalf("user %q not found in users service", nickname)
	return userResponse{}
}
