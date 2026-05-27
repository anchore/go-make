package golint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_findMalformedFilenames(t *testing.T) {
	t.Run("gitignored bad-named file is skipped", func(t *testing.T) {
		requireGit(t)
		root := setupGitRoot(t)
		writeFile(t, root, ".gitignore", "ignored/\n")
		mkdir(t, root, "ignored")
		writeFile(t, root, "ignored/bad:name.txt", "")

		require.NoError(t, findMalformedFilenames("."))
	})

	t.Run("tracked bad-named file fails", func(t *testing.T) {
		requireGit(t)
		root := setupGitRoot(t)
		writeFile(t, root, "bad:name.txt", "")
		runGit(t, root, "add", "bad:name.txt")

		require.Error(t, findMalformedFilenames("."))
	})

	t.Run("untracked-but-not-ignored bad-named file fails", func(t *testing.T) {
		requireGit(t)
		root := setupGitRoot(t)
		writeFile(t, root, "bad:name.txt", "")

		require.Error(t, findMalformedFilenames("."))
	})

	t.Run("no-git fallback walks everything", func(t *testing.T) {
		root := t.TempDir()
		t.Chdir(root)
		writeFile(t, root, "bad:name.txt", "")

		require.Error(t, findMalformedFilenames("."))
	})
}

// setupGitRoot creates a temp dir, runs `git init` in it, and chdirs into it.
// Returns the root so callers can write files via absolute paths.
func setupGitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	t.Chdir(root)
	return root
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func mkdir(t *testing.T, dir, rel string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}
