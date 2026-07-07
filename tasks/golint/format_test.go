package golint

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anchore/go-make/require"
)

func TestFormatterEnabled(t *testing.T) {
	tests := []struct {
		name           string
		formattersJSON string
		formatter      string
		want           bool
	}{
		{
			name:           "gci enabled",
			formattersJSON: `{"Enabled":[{"name":"gci"},{"name":"gofmt"}],"Disabled":[{"name":"gofumpt"}]}`,
			formatter:      "gci",
			want:           true,
		},
		{
			name:           "goimports enabled",
			formattersJSON: `{"Enabled":[{"name":"gofmt"},{"name":"goimports"}],"Disabled":[{"name":"gci"}]}`,
			formatter:      "goimports",
			want:           true,
		},
		{
			name:           "formatter only in disabled",
			formattersJSON: `{"Enabled":[{"name":"gofmt"}],"Disabled":[{"name":"gci"}]}`,
			formatter:      "gci",
			want:           false,
		},
		{
			name:           "no formatters enabled (null)",
			formattersJSON: `{"Enabled":null,"Disabled":[{"name":"gci"}]}`,
			formatter:      "gci",
			want:           false,
		},
		{
			name:           "malformed json",
			formattersJSON: `not json`,
			formatter:      "gci",
			want:           false,
		},
		{
			name:           "empty string",
			formattersJSON: ``,
			formatter:      "gci",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatterEnabled(tt.formattersJSON, tt.formatter))
		})
	}
}

func Test_findMalformedFilenames(t *testing.T) {
	// on Windows ':' is the NTFS alternate-data-stream separator, so
	// os.WriteFile("bad:name.txt") creates a file named "bad" instead of
	// failing — the malformed filename these tests rely on cannot exist
	if runtime.GOOS == "windows" {
		t.Skip("cannot create filenames containing ':' on Windows")
	}

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
