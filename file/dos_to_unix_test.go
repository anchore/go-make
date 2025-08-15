package file

import (
	"path/filepath"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_DosToUnix(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		glob     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unix line endings",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "dos line endings",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "dos line endings matching glob",
			glob:     "**/*.txt",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "dos line endings not matching glob",
			glob:     "**/*.txt.json",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\r\nline2\r\nline3",
		},
		{
			name:     "mixed line endings",
			input:    "line1\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.file == "" {
				tt.file = "test.txt"
			}
			if tt.glob == "" {
				tt.glob = tt.file
			}

			tempDir := t.TempDir()
			tempFile := filepath.Join(tempDir, tt.file)
			Write(tempFile, tt.input)

			DosToUnix(filepath.Join(tempDir, tt.glob))
			require.Equal(t, tt.expected, Read(tempFile))
		})
	}
}
