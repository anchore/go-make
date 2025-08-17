package binny

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/go-make/file"
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
			version: "bad\nyam: :l \n:", // malformed yaml should panic
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

			binnyYaml := filepath.Join(tmpDir, ".binny.yaml")
			require.NoError(t, os.WriteFile(binnyYaml, []byte(`tools:
  # we want to use a pinned version of binny to manage the toolchain (so binny manages itself!)
  - name: binny
    version:
      want: `+tt.version+`
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
`), 0o700))
			file.DosToUnix(binnyYaml)

			if tt.err == nil {
				tt.err = require.NoError
			}

			tt.err(t, lang.Catch(func() {
				versions := readBinnyYamlVersions()
				require.Equal(t, tt.version, versions["binny"])
				require.Equal(t, "v2.3.1", versions["golangci-lint"])

				installBinny(binnyPath)
			}))
		})
	}
}

func Test_isVersion(t *testing.T) {
	tests := []struct {
		version1 string
		version2 string
		want     bool
	}{
		{
			version1: "0.9.0",
			version2: "v0.9.0",
			want:     true,
		},
		{
			version1: "v0.9.0",
			version2: "0.9.0",
			want:     true,
		},
		{
			version1: "v0.8.0",
			version2: "v0.9.0",
			want:     false,
		},
		{
			version1: "v0.9.0",
			version2: "v0.8.0",
			want:     false,
		},
		{
			version1: "0.9.0",
			version2: "v0.8.0",
			want:     false,
		},
		{
			version1: "v0.9",
			version2: "v0.9.0",
			want:     true,
		},
		{
			version1: "v0.9.0-rc.1",
			version2: "v0.9.0-rc.1",
			want:     true,
		},
		{
			version1: "v0.9.0-rc.1",
			version2: "v0.9.0-rc.2",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.version1+" "+tt.version2, func(t *testing.T) {
			require.Equal(t, tt.want, isVersion(tt.version1, tt.version2))
		})
	}
}
