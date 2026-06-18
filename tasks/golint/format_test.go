package golint

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func TestFormatterEnabled(t *testing.T) {
	tests := []struct {
		name           string
		formattersJSON string
		formatter      string
		want           bool
	}{
		{
			name:           "gci enabled",
			formattersJSON: `{"Enabled":[{"name":"gci"},{"name":"gofmt"}],"Disabled":[{"name":"gofumpt"}]}`,
			formatter:      "gci",
			want:           true,
		},
		{
			name:           "goimports enabled",
			formattersJSON: `{"Enabled":[{"name":"gofmt"},{"name":"goimports"}],"Disabled":[{"name":"gci"}]}`,
			formatter:      "goimports",
			want:           true,
		},
		{
			name:           "formatter only in disabled",
			formattersJSON: `{"Enabled":[{"name":"gofmt"}],"Disabled":[{"name":"gci"}]}`,
			formatter:      "gci",
			want:           false,
		},
		{
			name:           "no formatters enabled (null)",
			formattersJSON: `{"Enabled":null,"Disabled":[{"name":"gci"}]}`,
			formatter:      "gci",
			want:           false,
		},
		{
			name:           "malformed json",
			formattersJSON: `not json`,
			formatter:      "gci",
			want:           false,
		},
		{
			name:           "empty string",
			formattersJSON: ``,
			formatter:      "gci",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatterEnabled(tt.formattersJSON, tt.formatter))
		})
	}
}
