package file_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/require"
)

func Test_FindParent(t *testing.T) {
	tests := []struct {
		file     string
		expected string
	}{
		{
			file:     ".config.yaml",
			expected: "some/.config.yaml",
		},
		{
			file:     ".config.json",
			expected: "some/nested/path/.config.json",
		},
		{
			file:     ".other",
			expected: "some/.other",
		},
		{
			file:     ".missing",
			expected: "",
		},
	}
	testdataDir, _ := os.Getwd()
	testdataDir = filepath.ToSlash(filepath.Join(testdataDir, "testdata")) + "/"
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			require.InDir(t, "testdata/some/nested/path", func() {
				path := file.FindParent(file.Cwd(), tt.file)
				path = filepath.ToSlash(path)
				path = strings.TrimPrefix(path, testdataDir)
				require.Equal(t, tt.expected, path)
			})
		})
	}
}
