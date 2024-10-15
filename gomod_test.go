package make_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/anchore/go-make"
)

func Test_FindFile(t *testing.T) {
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
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			inDir(t, "testdata/some/nested/path", func() {
				path := FindFile(test.file)
				path = filepath.ToSlash(path)
				path = strings.TrimPrefix(path, testdataDir)
				requireEqual(t, test.expected, path)
			})
		})
	}
}
