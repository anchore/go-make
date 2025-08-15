package file_test

import (
	"path/filepath"
	"testing"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/require"
)

func Test_Ls(t *testing.T) {
	ls := file.Ls("testdata/some/.other")
	require.Contains(t, ls, ".config.json")
	require.Contains(t, ls, ".config.yaml")
}

func Test_LogWorkdir(t *testing.T) {
	tmp := t.TempDir()
	file.InDir(tmp, func() {
		file.Write(filepath.Join(tmp, "file.txt"), "hello world")
		file.LogWorkdir()
	})
}
