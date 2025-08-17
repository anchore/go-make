package fetch

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_BinaryRelease(t *testing.T) {
	content := "test binary content"
	fileName := "thething"
	if runtime.GOOS == "windows" {
		fileName += ".exe"
	}

	ext := "tar.gz"
	archive := require.Gzip(require.Tar(map[string][]byte{
		fileName: []byte(content),
	}))
	if runtime.GOOS == "windows" {
		ext = "zip"
		archive = require.Zip(map[string][]byte{
			fileName: []byte(content),
		})
	}

	serverURL := require.Server(t, map[string]any{
		fmt.Sprintf("/releases/thething-v1.2.3-%s_%s.%s", runtime.GOARCH, runtime.GOOS, ext): archive,
	})

	// test the binary release download and extraction
	tmpDir := t.TempDir()
	fullPath := filepath.Join(tmpDir, fileName)

	// Add your BinaryRelease function test here using server.URL and tmpDir
	require.NoError(t, BinaryRelease(fullPath, ReleaseSpec{
		URL: serverURL + "/releases/thething-{{.version}}-{{.arch}}_{{.os}}.{{.ext}}",
		Args: map[string]string{
			"ext":     "tar.gz",
			"version": "v1.2.3",
		},
		Platform: map[string]map[string]string{
			"windows": {
				"ext": "zip",
			},
		},
	}))

	extractedContent, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	require.Equal(t, content, string(extractedContent))
}

func Test_ReleaseSpec_render(t *testing.T) {
	tests := []struct {
		expected string
		os       string
		arch     string
		spec     ReleaseSpec
	}{
		{
			expected: "https://example.com/releases/thething-v1.2.3-amd64_linux.tar.gz",
			os:       "linux",
			arch:     "amd64",
			spec: ReleaseSpec{
				URL: "https://example.com/releases/thething-{{.version}}-{{.arch}}_{{.os}}.{{.ext}}",
				Args: map[string]string{
					"ext":     "tar.gz",
					"version": "v1.2.3",
				},
			},
		},
		{
			expected: "https://example.com/releases/thething-v1.2.3-arm64_windows.zip",
			os:       "windows",
			arch:     "arm64",
			spec: ReleaseSpec{
				URL: "https://example.com/releases/thething-{{.version}}-{{.arch}}_{{.os}}.{{.ext}}",
				Args: map[string]string{
					"ext":     "tar.gz",
					"version": "v1.2.3",
				},
				Platform: map[string]map[string]string{
					"windows": {
						"ext": "zip",
					},
				},
			},
		},
		{
			expected: "https://example.com/releases/thething-v1.2.3-arm64_8.0_darwin.xz",
			os:       "darwin",
			arch:     "arm64",
			spec: ReleaseSpec{
				URL: "https://example.com/releases/thething-{{.version}}-{{.arch}}_{{.os}}.{{.ext}}",
				Args: map[string]string{
					"ext":     "tar.gz",
					"version": "v1.2.3",
				},
				Platform: map[string]map[string]string{
					"*/arm64": {
						"arch": "arm64_8.0",
					},
					"darwin": {
						"arch": "less_specific_arch",
						"ext":  "xz",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.spec.render(tt.os, tt.arch)
			require.Equal(t, tt.expected, got)
		})
	}
}
