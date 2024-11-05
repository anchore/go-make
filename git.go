package gomake

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

func GitRoot() string {
	root := FindFile(".git")
	if root == "" {
		Throw(fmt.Errorf(".git not found"))
	}
	return filepath.Dir(root)
}

func GitRevision() string {
	buf := bytes.Buffer{}
	err := Exec("git", ExecArgs("rev-parse", "--short", "HEAD"), ExecOut(&buf))
	if err != nil {
		return "UNKNOWN"
	}
	return strings.TrimSpace(buf.String())
}

func InGitClone(repo, branch string, fn func()) {
	InTempDir(func() {
		Run("git clone --depth 1 --branch", branch, repo, "repo")
		Cd("repo")
		fn()
	})
}
