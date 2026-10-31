package util

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecimalToCents(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Valid prices.
		{
			name:  "integer price",
			input: "150",
			want:  15000,
		},
		{
			name:  "two decimal places",
			input: "150.50",
			want:  15050,
		},
		{
			name:  "one decimal place",
			input: "150.5",
			want:  15050,
		},
		{
			name:  "zero",
			input: "0",
			want:  0,
		},
		{
			name:  "zero with decimals",
			input: "0.00",
			want:  0,
		},
		{
			name:  "one cent",
			input: "0.01",
			want:  1,
		},
		{
			name:  "nine cents",
			input: "0.09",
			want:  9,
		},
		{
			name:  "leading zeros",
			input: "001.05",
			want:  105,
		},
		{
			name:  "maximum supported price",
			input: "92233720368547758.07",
			want:  math.MaxInt64,
		},

		// Invalid prices.
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing integer part",
			input:   ".50",
			wantErr: true,
		},
		{
			name:    "missing fractional part",
			input:   "150.",
			wantErr: true,
		},
		{
			name:    "more than two decimal places",
			input:   "150.555",
			wantErr: true,
		},
		{
			name:    "multiple decimal separators",
			input:   "150.5.0",
			wantErr: true,
		},
		{
			name:    "comma separator",
			input:   "150,50",
			wantErr: true,
		},
		{
			name:    "negative integer",
			input:   "-150",
			wantErr: true,
		},
		{
			name:    "negative decimal",
			input:   "-150.50",
			wantErr: true,
		},
		{
			name:    "non-numeric input",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "non-numeric fractional part",
			input:   "150.ab",
			wantErr: true,
		},
		{
			name:    "leading whitespace",
			input:   " 150.50",
			wantErr: true,
		},
		{
			name:    "trailing whitespace",
			input:   "150.50 ",
			wantErr: true,
		},
		{
			name:    "exponent notation",
			input:   "1e2",
			wantErr: true,
		},
		{
			name:    "integer overflow",
			input:   "9223372036854775808",
			wantErr: true,
		},
		{
			name:    "price overflow",
			input:   "92233720368547758.08",
			wantErr: true,
		},
		{
			name:    "large integer part",
			input:   "9223372036854775807",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecimalToCents(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
