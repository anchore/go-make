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
			file.InDir(tt.dir, func() {
				expected := lang.Return(filepath.Abs(filepath.Join(startDir, tt.dir)))
				got := lang.Return(filepath.Abs(file.Cwd()))
				require.Equal(t, expected, got)
			})
		})
	}
}

func Test_Copy(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "srcfile.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("hello world"), 0o742))
	srcPerms := lang.Return(os.Stat(srcFile)).Mode()
	dstFile := filepath.Join(tempDir, "tmpdir", "dstfile.txt")

	file.Copy(srcFile, dstFile)

	require.Equal(t, "hello world", file.Read(dstFile))
	perms := lang.Return(os.Stat(dstFile)).Mode()
	require.Equal(t, srcPerms, perms.Perm())
}

func Test_IsRegular(t *testing.T) {
	tempDir := t.TempDir()
	require.True(t, !file.IsRegular(tempDir))

	srcFile := filepath.Join(tempDir, "srcfile.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("hello world"), 0o742))
	require.True(t, file.IsRegular(srcFile))
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
			file.InDir("testdata/some/nested/path", func() {
				path := file.FindParent(file.Cwd(), tt.file)
				path = filepath.ToSlash(path)
				path = strings.TrimPrefix(path, testdataDir)
				require.Equal(t, tt.expected, path)
			})
		})
	}
}

func Test_EnsureDir(t *testing.T) {
	testDir := t.TempDir()

	newDir := filepath.Join(testDir, "newdir")

	require.True(t, !file.Exists(newDir))

	file.EnsureDir(newDir)

	require.True(t, file.Exists(newDir))

	// existing dir, does nothing, no error
	file.EnsureDir(newDir)

	require.True(t, file.Exists(newDir))

	newFile := filepath.Join(testDir, "newfile")
	require.NoError(t, os.WriteFile(newFile, []byte("hello world"), 0o700))

	// should panic if unable to create dir such as when a file exists
	require.Error(t, lang.Catch(func() {
		file.EnsureDir(newFile)
	}))
}

func Test_FindAll(t *testing.T) {
	tests := []struct {
		pattern       string
		expectedCount int
	}{
		{
			pattern:       "**/*.json",
			expectedCount: 2,
		},
		{
			pattern:       "**/*",
			expectedCount: 4,
		},
		{
			pattern:       ".config.yaml",
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			file.InDir("testdata/some", func() {
				got := file.FindAll(tt.pattern)
				require.Equal(t, tt.expectedCount, len(got))
			})
		})
	}
}
