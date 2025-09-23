package github

import (
	"encoding/json"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_matrixSuffix(t *testing.T) {
	tests := []struct {
		expected string
		matrix   map[string]any
	}{
		{
			expected: "-windows",
			matrix: map[string]any{
				"os": "windows",
			},
		},
		{
			expected: "-arch-x64-os-windows",
			matrix: map[string]any{
				"platform": map[string]any{
					"oS":   "windows",
					"arch": "x64",
				},
			},
		},
		{
			expected: "-arch-x64",
			matrix: map[string]any{
				"platform": map[string]any{
					"os":   "",
					"arCH": "x64",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			matrixJSON, _ := json.Marshal(tt.matrix)
			t.Setenv("MATRIX_JSON", string(matrixJSON))
			got := matrixSuffix()
			require.Equal(t, tt.expected, got)
		})
	}
}
