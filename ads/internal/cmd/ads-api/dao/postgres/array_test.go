package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdIDsValue(t *testing.T) {
	tests := []struct {
		name string
		ids  adIDs
		want string
	}{
		{"nil", nil, "{}"},
		{"empty", adIDs{}, "{}"},
		{"single", adIDs{101}, "{101}"},
		{"multiple", adIDs{101, 102}, "{101,102}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ids.Value()

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAdIDsScan(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    adIDs
		wantErr bool
	}{
		{
			name:  "string",
			input: "{101,102}",
			want:  adIDs{101, 102},
		},
		{
			name:  "bytes",
			input: []byte("{101,102}"),
			want:  adIDs{101, 102},
		},
		{
			name:  "empty",
			input: "{}",
			want:  adIDs{},
		},
		{
			name:    "invalid format",
			input:   "101,102",
			wantErr: true,
		},
		{
			name:    "invalid ID",
			input:   "{101,abc}",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   42,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ids adIDs

			err := ids.Scan(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, ids)
		})
	}
}
