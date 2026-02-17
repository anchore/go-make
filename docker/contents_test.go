package docker

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/require"
)

func Test_fixtureBuildPushRestore(t *testing.T) {
	if config.CI && runtime.GOOS == "darwin" {
		t.Skip("skipping on macos in CI due to docker registry issues")
	}

	setupLocalRegistry(t)

	// use a new tempdir as the cache root so there is no existing cache
	tempDir := newCacheDir(t)

	// this test should push images
	require.SetAndRestore(t, &PushImageCache, true)

	// capture the log output to verify specific exected steps
	logOutput := captureLogs(t)

	// we need to use the hash to verify paths are expected
	dockerfileHash := file.Fingerprint("testdata/example-fixture/Dockerfile")

	// execute once, without cache to verify build and execute push
	dir := ExportCached("testdata/example-fixture/Dockerfile", "**/some*")

	require.Equal(t, filepath.Join(tempDir, dockerfileHash, "example-fixture"), dir)

	// verify the expected file was put in the cache dir from the build
	someFileContents := file.Read("testdata/example-fixture/some-file.txt")
	require.NotEmpty(t, someFileContents)
	require.Equal(t, someFileContents, file.Read(filepath.Join(dir, "some-file.txt")))

	// verify files are filtered based on glob
	require.True(t, !file.Exists(filepath.Join(dir, "testdata/example-fixture/other-file.txt")))

	// verify log contains expected steps
	require.Contains(t, *logOutput, "cache miss")
	require.Contains(t, *logOutput, "building")
	require.Contains(t, *logOutput, "pushing")
	require.NotContains(t, *logOutput, "restored from oras cache")

	// ensure the cache is downloaded; reset log output to make sure we restored from cache and did not build
	*logOutput = ""

	// use a new tmpdir so there is no build cache
	tempDir = newCacheDir(t)

	// fetch from cache
	dir = ExportCached("testdata/example-fixture/Dockerfile", "**/some*")

	// most important: the file copied from the build step was pushed and restored
	require.Equal(t, someFileContents, file.Read(filepath.Join(dir, "some-file.txt")))
	require.Equal(t, file.Read("testdata/example-fixture/some-thing.txt"), file.Read(filepath.Join(dir, "some-thing.txt")))

	require.NotContains(t, *logOutput, "cache miss")
	require.NotContains(t, *logOutput, "building")
	require.Contains(t, *logOutput, "restored from oras cache")
}

func Test_FixtureBuildFailures(t *testing.T) {
	logOutput := ""
	capture := func(format string, args ...any) {
		logOutput += fmt.Sprintf(format, args...)
		t.Log(fmt.Sprintf(format, args...))
	}
	require.SetAndRestore(t, &log.Info, capture)
	require.SetAndRestore(t, &log.Debug, capture)

	// fetch from cache
	err := lang.Catch(func() {
		_ = ExportCached("testdata/bad-dockerfile/Dockerfile")
	})
	require.Error(t, err)
	require.Contains(t, err, "error executing: 'docker buildx")

	require.Contains(t, logOutput, "building with docker buildx")

	logOutput = ""

	// building again should not have cached build
	err = lang.Catch(func() {
		_ = ExportCached("testdata/bad-dockerfile/Dockerfile")
	})
	require.Error(t, err)
	require.Contains(t, err, "error executing: 'docker buildx")

	require.Contains(t, logOutput, "building with docker buildx")
}

func Test_findCacheDir(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "testdata with Dockerfile",
			input:    filepath.FromSlash("/home/user/project/testdata/example/Dockerfile"),
			expected: filepath.FromSlash("/home/user/project/testdata/.cache/<hash>/example"),
		},
		{
			name:     "test-fixtures with Dockerfile",
			input:    filepath.FromSlash("/home/user/project/test-fixtures/example/Dockerfile"),
			expected: filepath.FromSlash("/home/user/project/test-fixtures/.cache/<hash>/example"),
		},
		{
			name:     "testdata with non-standard dockerfile name",
			input:    filepath.FromSlash("/home/user/project/testdata/example/Dockerfile.build"),
			expected: filepath.FromSlash("/home/user/project/testdata/.cache/<hash>/example/Dockerfile.build"),
		},
		{
			name:     "no testdata or test-fixtures - uses .cache",
			input:    filepath.FromSlash("/home/user/project/build/Dockerfile"),
			expected: filepath.FromSlash("/home/user/project/build/.cache/<hash>"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findCacheDir(tt.input, "<hash>")
			require.Equal(t, tt.expected, result)
		})
	}
}

func newCacheDir(t *testing.T) string {
	tempDir := t.TempDir()
	log.Info("using temp dir: %s", tempDir)
	require.SetAndRestore(t, &CacheDir, tempDir)
	return tempDir
}

func findRandomPort() int {
	listener := lang.Return(net.Listen("tcp", ":0"))
	defer lang.Close(listener)
	return listener.Addr().(*net.TCPAddr).Port
}
