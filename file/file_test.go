package file_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/require"
)

func Test_Cwd(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{
			name: "current directory",
			dir:  ".",
		},
		{
			name: "other directory",
			dir:  "testdata",
		},
	}

	startDir := lang.Return(os.Getwd())
	defer func() { require.NoError(t, os.Chdir(startDir)) }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.InDir(t, tt.dir, func() {
				expected := lang.Return(filepath.Abs(filepath.Join(startDir, tt.dir)))
				got := lang.Return(filepath.Abs(file.Cwd()))
				require.Equal(t, expected, got)
			})
		})
	}
}

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
