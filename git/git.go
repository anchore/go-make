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

// Root returns the root directory of the git repository by searching upward
// for a .git directory. Panics if no .git directory is found.
func Root() string {
	root := file.FindParent(file.Cwd(), ".git")
	if root == "" {
		panic(fmt.Errorf(".git not found"))
	}
	return filepath.Dir(root)
}

// Revision returns the short commit SHA of HEAD.
func Revision() string {
	return lang.Return(run.Command("git", run.Args("rev-parse", "--short", "HEAD")))
}

// InClone performs a shallow clone of the repository at the specified ref into a
// temporary directory, runs the provided function, then cleans up. Useful for
// operations that need to work with a specific version of a repo without affecting
// the current working directory.
func InClone(repo, ref string, fn func()) {
	file.InTempDir(func() {
		lang.Return(run.Command("git", run.Args("clone", "--depth", "1", "--branch", ref, repo, "."), run.Stderr(io.Discard)))
		file.LogWorkdir()
		fn()
	})
}
