package git

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

func init() {
	template.Globals["GitRoot"] = Root
}

func Root() string {
	root := file.FindParent(file.Cwd(), ".git")
	if root == "" {
		panic(fmt.Errorf(".git not found"))
	}
	return filepath.Dir(root)
}

func Revision() string {
	return lang.Return(run.Command("git", run.Args("rev-parse", "--short", "HEAD")))
}

func InClone(repo, ref string, fn func()) {
	file.InTempDir(func() {
		lang.Return(run.Command("git", run.Args("clone", "--depth", "1", "--branch", ref, repo, "."), run.Stderr(io.Discard)))
		file.LogWorkdir()
		fn()
	})
}
