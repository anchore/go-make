package binny

import (
	"strings"
	"testing"

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
				versions := readBinnyYamlVersions(binnyYaml)
				require.Equal(t, tt.version, versions["binny"])
				require.Equal(t, "v2.3.1", versions["golangci-lint"])

				installBinny(binnyPath, tt.version)
			}))
		})
	}
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
