package postgres

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// adIDs represents a PostgreSQL BIGINT[].
type adIDs []domain.AdID

// Value implements driver.Valuer.
func (ids adIDs) Value() (driver.Value, error) {
	values := make([]string, len(ids))

	for i, id := range ids {
		values[i] = strconv.FormatInt(int64(id), 10)
	}

	return "{" + strings.Join(values, ",") + "}", nil
}

// Scan implements sql.Scanner.
func (ids *adIDs) Scan(src any) error {
	var value string

	switch v := src.(type) {
	case string:
		value = v
	case []byte:
		value = string(v)
	default:
		return fmt.Errorf("unsupported PostgreSQL array type %T", src)
	}

	if len(value) < 2 || value[0] != '{' || value[len(value)-1] != '}' {
		return fmt.Errorf("invalid PostgreSQL array: %q", value)
	}

	content := value[1 : len(value)-1]

	if content == "" {
		*ids = adIDs{}
		return nil
	}

	parts := strings.Split(content, ",")
	result := make(adIDs, 0, len(parts))

	for _, part := range parts {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ad ID %q: %w", part, err)
		}

		result = append(result, domain.AdID(id))
	}

	*ids = result

	return nil
}
