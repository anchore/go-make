package binny

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/template"
)

func Test_installBinny(t *testing.T) {
	tests := []struct {
		version string
		err     func(t *testing.T, err error)
	}{
		{
			version: "v0.9.0", // has a valid release, may build from source
		},
		{
			version: "main", // does not have a release, will build from branch
		},
		{
			version: "bad\ny\nam: :l \n:", // malformed yaml should panic
			err:     require.Error,
		},
		{
			version: "definitely-not-a-valid-version", // unknown version should panic
			err:     require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			originalRoot := template.Globals["GitRoot"]
			defer func() { template.Globals["GitRoot"] = originalRoot }()

			tmpDir := t.TempDir()
			template.Globals["GitRoot"] = tmpDir
			binnyPath := ToolPath("binny")

			binnyYaml := strings.NewReader(`tools:
  # we want to use a pinned version of binny to manage the toolchain (so binny manages itself!)
  - name: binny
    version:
      want: ` + tt.version + `
    method: github-release
    with:
      repo: anchore/binny

  # used for linting
  - name: golangci-lint
    version:
      want: v2.3.1
    method: github-release
    with:
      repo: golangci/golangci-lint
`)
			if tt.err == nil {
				tt.err = require.NoError
			}

			tt.err(t, lang.Catch(func() {
				specs := readBinnyYamlSpecs(binnyYaml, "")
				require.Equal(t, tt.version, specs["binny"].Version)
				require.Equal(t, "v2.3.1", specs["golangci-lint"].Version)

				installBinny(binnyPath, tt.version)
			}))
		})
	}
}

func Test_readBinnyYamlSpecs_localModule(t *testing.T) {
	// pin the contract that a `method: go-install` entry whose `with.module`
	// looks like a local path is captured as a LocalModule spec, with the
	// path resolved against the .binny.yaml's directory.
	baseDir := t.TempDir()
	yaml := strings.NewReader(`tools:
  - name: binny
    version:
      want: current
    method: go-install
    with:
      module: .
      entrypoint: cmd/binny

  - name: chronicle
    version:
      want: v0.9.0
    method: github-release
    with:
      repo: anchore/chronicle
`)

	specs := readBinnyYamlSpecs(yaml, baseDir)

	wantAbs := lang.Return(filepath.Abs(baseDir))
	require.Equal(t, wantAbs, specs["binny"].LocalModule)
	require.Equal(t, "cmd/binny", specs["binny"].Entrypoint)
	require.Equal(t, "current", specs["binny"].Version)

	// non-local entries should still parse, but with no LocalModule
	require.Equal(t, "", specs["chronicle"].LocalModule)
	require.Equal(t, "v0.9.0", specs["chronicle"].Version)
}

func Test_isLocalPath(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{".", true},
		{"./", true},
		{"./pkg", true},
		{"../sibling", true},
		{"/abs/path", true},
		{"github.com/anchore/binny", false},
		{"", false},
		{"binny", false}, // bare names are go module imports, not local paths
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			require.Equal(t, tt.want, isLocalPath(tt.in))
		})
	}
}

func Test_installBinnyShim(t *testing.T) {
	if config.Windows || runtime.GOOS == "windows" {
		t.Skip("shim is sh-based; Windows takes the go build path which would require a real go module on disk")
	}

	toolDir := t.TempDir()
	binnyPath := filepath.Join(toolDir, "binny")
	moduleDir := t.TempDir()

	// reset the package-level installed cache so a prior test can't shadow us
	defer func(prev map[string]string) { installed = prev }(installed)
	installed = map[string]string{}

	installBinnyShim(binnyPath, toolSpec{LocalModule: moduleDir, Entrypoint: "cmd/binny"})

	content, err := os.ReadFile(binnyPath)
	require.NoError(t, err)
	body := string(content)

	expectedPkg := filepath.Join(moduleDir, "cmd/binny")
	require.Contains(t, body, "#!/bin/sh")
	require.Contains(t, body, "exec go run")
	require.Contains(t, body, expectedPkg)

	info, err := os.Stat(binnyPath)
	require.NoError(t, err)
	require.True(t, info.Mode()&0o111 != 0)
	require.Equal(t, binnyPath, installed[CMD])

	// idempotency: a second call with identical spec must leave mtime untouched
	// (matched contents — skip write path).
	mtimeBefore := info.ModTime()
	// guarantee any real write would tick the mtime
	require.NoError(t, os.Chtimes(binnyPath, mtimeBefore, mtimeBefore))
	installBinnyShim(binnyPath, toolSpec{LocalModule: moduleDir, Entrypoint: "cmd/binny"})
	info, err = os.Stat(binnyPath)
	require.NoError(t, err)
	require.True(t, info.ModTime().Equal(mtimeBefore))
}

func Test_installBinnyShim_replacesReadOnlyFile(t *testing.T) {
	// regression: a prior `binny` release binary at .tool/binny is often
	// installed with -r-x------ (read+exec only). os.WriteFile honors the
	// existing file's mode, so writing the shim over it must remove first.
	if config.Windows || runtime.GOOS == "windows" {
		t.Skip("shim path is sh-only; windows uses go build which is exercised elsewhere")
	}

	toolDir := t.TempDir()
	binnyPath := filepath.Join(toolDir, "binny")
	moduleDir := t.TempDir()

	require.NoError(t, os.WriteFile(binnyPath, []byte("old-binary"), 0o500)) //nolint:gosec
	require.NoError(t, os.Chmod(binnyPath, 0o500))

	defer func(prev map[string]string) { installed = prev }(installed)
	installed = map[string]string{}

	installBinnyShim(binnyPath, toolSpec{LocalModule: moduleDir, Entrypoint: "cmd/binny"})

	content, err := os.ReadFile(binnyPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "exec go run")
}

func Test_matchesVersion(t *testing.T) {
	tests := []struct {
		version1 string
		version2 string
		want     bool
	}{
		{
			// baseline: leading "v" is optional on either side.
			version1: "0.9.0",
			version2: "v0.9.0",
			want:     true,
		},
		{
			// baseline: surrounding whitespace is trimmed before comparison.
			version1: " v0.9.0 ",
			version2: "0.9.0",
			want:     true,
		},
		{
			// baseline: a differing numeric component is a mismatch.
			version1: "v0.8.0",
			version2: "v0.9.0",
			want:     false,
		},
		{
			// baseline: when parts1 has fewer tokens than parts2, the extras
			// in parts2 are ignored — a less-specific request matches a
			// more-specific installed version as long as the shared prefix agrees.
			version1: "v0.9",
			version2: "v0.9.0",
			want:     true,
		},
		{
			// baseline: prerelease tokens are compared component-wise; equal here.
			version1: "v0.9.0-rc.1",
			version2: "v0.9.0-rc.1",
			want:     true,
		},
		{
			// baseline: a differing prerelease component is a mismatch.
			version1: "v0.9.0-rc.1",
			version2: "v0.9.0-rc.2",
			want:     false,
		},
		{
			// regression: parts1 has more digit-bearing tokens than parts2.
			// Previously panicked because the loop used i <= len(parts2)
			// instead of i < len(parts2), so at i==len(parts2) it accessed
			// parts2[len(parts2)] which is out of bounds.
			version1: "v0.9.0",
			version2: "v0.9",
			want:     true,
		},
		{
			// regression: previously panicked with index out of range because
			// "main" produces zero digit-bearing tokens while "0.13.0" produces
			// three, and the loop accessed parts2[0] anyway.
			version1: "v0.13.0",
			version2: "main",
			want:     false,
		},
		{
			// non-numeric refs on both sides fall back to direct string equality.
			version1: "main",
			version2: "main",
			want:     true,
		},
		{
			// "current" is not a sentinel: it's treated as any other non-numeric
			// ref. A configured target of "current" will never match a real
			// --version output, so it will trigger a reinstall. Pin this so a
			// future change doesn't quietly re-introduce special handling.
			version1: "v0.13.0",
			version2: "current",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.version1+" "+tt.version2, func(t *testing.T) {
			require.Equal(t, tt.want, matchesVersion(tt.version1, tt.version2))
		})
	}
}
