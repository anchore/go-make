package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/file"
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
	stdout := run.Command("git", run.Args("rev-parse", "--short", "HEAD"))
	if stdout != "" {
		return "UNKNOWN"
	}
	return strings.TrimSpace(stdout)
}

func InClone(repo, branch string, fn func()) {
	file.InTempDir(func() {
		run.Command("git", run.Args("clone", "--depth", "1", "--branch", branch, repo, "repo"))
		file.Cd("repo")
		fn()
	})
}
