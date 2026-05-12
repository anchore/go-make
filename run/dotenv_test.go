package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/require"
)

func TestParseDotEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "simple key value pairs",
			input: "FOO=bar\nBAZ=qux",
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name: "comments and blank lines are ignored",
			input: `# top comment

FOO=bar
# inline comment line
BAZ=qux
`,
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name:     "export prefix is stripped",
			input:    "export FOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "double-quoted value",
			input:    `FOO="hello world"`,
			expected: map[string]string{"FOO": "hello world"},
		},
		{
			name:     "single-quoted value",
			input:    `FOO='hello world'`,
			expected: map[string]string{"FOO": "hello world"},
		},
		{
			name:     "value containing equals signs preserved",
			input:    "FOO=a=b=c",
			expected: map[string]string{"FOO": "a=b=c"},
		},
		{
			name:     "op reference is preserved verbatim by parser",
			input:    "GITHUB_TOKEN=op://Personal/GitHub/token",
			expected: map[string]string{"GITHUB_TOKEN": "op://Personal/GitHub/token"},
		},
		{
			name:     "line without equals is skipped",
			input:    "JUST_A_WORD\nFOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "leading equals is skipped",
			input:    "=novalue\nFOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "whitespace around key and value is trimmed",
			input:    "  FOO   =   bar  ",
			expected: map[string]string{"FOO": "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDotEnv(tt.input)
			require.Equal(t, len(tt.expected), len(got))
			for k, v := range tt.expected {
				require.Equal(t, v, got[k])
			}
		})
	}
}

func TestMergeDotEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         []string
		dotEnv      map[string]string
		wantHas     []string // entries that must be present (exact "KEY=VAL")
		wantSkipped []string
	}{
		{
			name:    "adds new keys",
			env:     []string{"PATH=/usr/bin"},
			dotEnv:  map[string]string{"FOO": "bar"},
			wantHas: []string{"PATH=/usr/bin", "FOO=bar"},
		},
		{
			name:        "process env wins on conflict",
			env:         []string{"FOO=fromprocess"},
			dotEnv:      map[string]string{"FOO": "fromdotenv"},
			wantHas:     []string{"FOO=fromprocess"},
			wantSkipped: []string{"FOO"},
		},
		{
			name:    "empty dotenv is a noop",
			env:     []string{"PATH=/usr/bin"},
			dotEnv:  map[string]string{},
			wantHas: []string{"PATH=/usr/bin"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, skipped := mergeDotEnv(tt.env, tt.dotEnv)
			joined := strings.Join(got, "\x00")
			for _, want := range tt.wantHas {
				require.Contains(t, joined, want)
			}
			if tt.wantSkipped == nil {
				require.Equal(t, 0, len(skipped))
			} else {
				require.EqualElements(t, tt.wantSkipped, skipped)
			}
		})
	}
}

func TestReadDotEnv_RefusesWhenNotGitignored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// simulate git reporting the file as NOT ignored
	require.SetAndRestore(t, &gitCheckIgnore, func(string) bool { return false })
	require.SetAndRestore(t, &opInject, func(string) (string, error) {
		t.Fatalf("opInject must not be called when load is refused")
		return "", nil
	})
	got := readDotEnv(path)
	require.Equal(t, 0, len(got))
}

func TestReadDotEnv_LoadsWhenGitignored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	require.SetAndRestore(t, &gitCheckIgnore, func(string) bool { return true })
	got := readDotEnv(path)
	require.Equal(t, "bar", got["FOO"])
}

// TestRunGitCheckIgnore_RealGit exercises the actual git invocation against a
// real, throwaway repo so we know the exit-code parsing matches reality.
func TestRunGitCheckIgnore_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	ignored := filepath.Join(dir, ".env")
	if err := os.WriteFile(ignored, []byte(""), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	notIgnored := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(notIgnored, []byte(""), 0o600); err != nil {
		t.Fatalf("write tracked: %v", err)
	}

	require.True(t, runGitCheckIgnore(ignored))
	require.False(t, runGitCheckIgnore(notIgnored))

	// outside a repo: permissive (true) so non-git workflows still function
	outside := t.TempDir()
	stray := filepath.Join(outside, ".env")
	if err := os.WriteFile(stray, []byte(""), 0o600); err != nil {
		t.Fatalf("write stray: %v", err)
	}
	require.True(t, runGitCheckIgnore(stray))
}

func TestReadDotEnv_Missing(t *testing.T) {
	dir := t.TempDir()
	got := readDotEnv(filepath.Join(dir, ".env"))
	require.Equal(t, 0, len(got))
}

func TestReadDotEnv_EmptyPath(t *testing.T) {
	require.Equal(t, 0, len(readDotEnv("")))
}

func TestReadDotEnv_NoOpInjectWhenNoRefs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	require.SetAndRestore(t, &opInject, func(string) (string, error) {
		t.Fatalf("opInject must not be called when no op:// references exist")
		return "", nil
	})
	got := readDotEnv(path)
	require.Equal(t, "bar", got["FOO"])
}

func TestReadDotEnv_OpInjectResolves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "GITHUB_TOKEN=op://Personal/GitHub/token\nOTHER=plain\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	calls := 0
	require.SetAndRestore(t, &opInject, func(in string) (string, error) {
		calls++
		return strings.ReplaceAll(in, "op://Personal/GitHub/token", "ghp_resolved"), nil
	})
	got := readDotEnv(path)
	require.Equal(t, 1, calls)
	require.Equal(t, "ghp_resolved", got["GITHUB_TOKEN"])
	require.Equal(t, "plain", got["OTHER"])
}

// TestCommand_DotEnvReachesChildEnv proves the full path: a .env file on disk
// is loaded by run.Command and its values appear in the spawned process's
// environment. Also verifies that an op:// reference in the same file is
// resolved (via mocked opInject) before being passed through.
func TestCommand_DotEnvReachesChildEnv(t *testing.T) {
	rootDir := t.TempDir()
	// use test-unique keys so we don't collide with the user's real shell env
	dotenv := "DOTENV_TEST_PLAIN=plain-value\nDOTENV_TEST_TOKEN=op://Vault/Item/token\n"
	if err := os.WriteFile(filepath.Join(rootDir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	// point RootDir at our temp dir; restore prior value after the test.
	// using a literal string (no template directives) so template.Render is a no-op.
	require.SetAndRestore(t, &config.RootDir, rootDir)
	// fresh Once + empty cache so loadDotEnv re-reads with our config
	require.SetAndRestore(t, &dotEnvOnce, &sync.Once{})
	require.SetAndRestore(t, &dotEnvCache, map[string]string(nil))
	// resolve op:// without actually invoking the 1Password CLI
	require.SetAndRestore(t, &opInject, func(in string) (string, error) {
		return strings.ReplaceAll(in, "op://Vault/Item/token", "resolved-token"), nil
	})

	// build testapp using a fresh Once so the build invocation itself uses our config too
	buildDir := t.TempDir()
	testapp := filepath.Join(buildDir, "testapp")
	if config.Windows {
		testapp += ".exe"
	}
	_, err := Command("go", Args("build", "-C", filepath.Join("testdata", "testapp"), "-o", testapp, "."))
	require.NoError(t, err)

	// child should see the plain value
	plain, err := Command(testapp, Args("env", "DOTENV_TEST_PLAIN"))
	require.NoError(t, err)
	require.Equal(t, "plain-value", plain)

	// and the op:// reference should be resolved before reaching the child
	tok, err := Command(testapp, Args("env", "DOTENV_TEST_TOKEN"))
	require.NoError(t, err)
	require.Equal(t, "resolved-token", tok)
}

// TestCommand_ProcessEnvWinsOverDotEnv verifies that when the same key is set
// in both the process environment and .env, the process environment wins.
func TestCommand_ProcessEnvWinsOverDotEnv(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, ".env"), []byte("PRECEDENCE_KEY=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	require.SetAndRestore(t, &config.RootDir, rootDir)
	require.SetAndRestore(t, &dotEnvOnce, &sync.Once{})
	require.SetAndRestore(t, &dotEnvCache, map[string]string(nil))

	t.Setenv("PRECEDENCE_KEY", "from-process")

	buildDir := t.TempDir()
	testapp := filepath.Join(buildDir, "testapp")
	if config.Windows {
		testapp += ".exe"
	}
	_, err := Command("go", Args("build", "-C", filepath.Join("testdata", "testapp"), "-o", testapp, "."))
	require.NoError(t, err)

	got, err := Command(testapp, Args("env", "PRECEDENCE_KEY"))
	require.NoError(t, err)
	require.Equal(t, "from-process", got)
}

func TestReadDotEnv_OpInjectFailureFallsBackToRaw(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("GITHUB_TOKEN=op://x/y/z\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	require.SetAndRestore(t, &opInject, func(string) (string, error) {
		return "", fmt.Errorf("op binary not found")
	})
	got := readDotEnv(path)
	require.Equal(t, "op://x/y/z", got["GITHUB_TOKEN"])
}
