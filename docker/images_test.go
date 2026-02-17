package docker

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/run"
)

func Test_imageBuildNoPush(t *testing.T) {
	if config.CI && runtime.GOOS == "darwin" {
		t.Skip("skipping on macos in CI due to docker registry issues")
	}

	setupLocalRegistry(t)

	// this test should NOT push images
	require.SetAndRestore(t, &PushImageCache, false)

	// capture the log output to verify specific expected steps
	logOutput := captureLogs(t)

	// execute once, without cache to verify build and push
	img := PullCached("testdata/example-fixture/Dockerfile")
	require.NotEmpty(t, img)

	defer func() {
		_, err := run.Command("docker", run.Args("rmi", img))
		if err != nil {
			log.Warn("unable to remove image: %s", img)
		}
	}()

	// verify the log indicates we built and pushed the image
	require.Contains(t, *logOutput, "cache miss")
	require.Contains(t, *logOutput, "building")
	require.NotContains(t, *logOutput, "pushing")
	require.NotContains(t, *logOutput, "restored")

	//ensure the cache is downloaded
	// reset log output to make sure we restored from cache and did not build
	*logOutput = ""

	// fetch from cache
	img = PullCached("testdata/example-fixture/Dockerfile")

	// verify log indicates we got the image from the cache
	require.Contains(t, *logOutput, "cache miss")
	require.Contains(t, *logOutput, "building")
	require.NotContains(t, *logOutput, "pushing")
	require.NotContains(t, *logOutput, "restored")
}

func Test_imageBuildPushRestore(t *testing.T) {
	if config.CI && runtime.GOOS == "darwin" {
		t.Skip("skipping on macos in CI due to docker registry issues")
	}

	setupLocalRegistry(t)

	// this test should push images
	require.SetAndRestore(t, &PushImageCache, true)

	// capture the log output to verify specific expected steps
	logOutput := captureLogs(t)

	// execute once, without cache to verify build and push
	img := PullCached("testdata/example-fixture/Dockerfile")
	require.NotEmpty(t, img)

	defer func() {
		_, err := run.Command("docker", run.Args("rmi", img))
		if err != nil {
			log.Warn("unable to remove image: %s", img)
		}
	}()

	// verify the log indicates we built and pushed the image
	require.Contains(t, *logOutput, "cache miss")
	require.Contains(t, *logOutput, "building")
	require.Contains(t, *logOutput, "pushing")
	require.NotContains(t, *logOutput, "pulled from registry")

	//ensure the cache is downloaded
	// reset log output to make sure we restored from cache and did not build
	*logOutput = ""

	// fetch from cache
	img = PullCached("testdata/example-fixture/Dockerfile")

	// verify log indicates we got the image from the cache
	require.NotContains(t, *logOutput, "cache miss")
	require.NotContains(t, *logOutput, "building")
	require.NotContains(t, *logOutput, "pushing")
	require.Contains(t, *logOutput, "cache hit")
	require.Contains(t, *logOutput, "pulled from registry")
}

func captureLogs(t *testing.T) *string {
	logOutput := ""
	capture := func(format string, args ...any) {
		logOutput += fmt.Sprintf(format, args...)
		t.Log(fmt.Sprintf(format, args...))
	}
	require.SetAndRestore(t, &log.Info, capture)
	require.SetAndRestore(t, &log.Debug, capture)
	return &logOutput
}

// setupLocalRegistry sets up a local registry and temporarily for the duration of the test sets the CachePrefix so operations will
// use the local registry instead of ghcr.io
func setupLocalRegistry(t *testing.T) (hostPort string) {
	testRegistry := os.Getenv("TEST_REGISTRY")
	if testRegistry != "" {
		log.Info("using preconfigured registry at: %s", testRegistry)
		require.SetAndRestore(t, &CachePrefix, testRegistry)
		return testRegistry
	}

	// set up a real registry we use to verify push/restore behavior:
	randomPort := findRandomPort()

	const registryImage = "registry:3"

	registryContainerId, err := run.Command("docker", run.Args("container", "run",
		"--platform", "linux",
		"--rm",
		"--env", "REGISTRY_STORAGE=inmemory",
		"--detach",
		"--publish", fmt.Sprintf("%v:5000", randomPort),
		registryImage))
	require.NoError(t, err)

	log.Info("running registry container on port: %v", randomPort)
	t.Cleanup(func() {
		_, err = run.Command("docker", run.Args("container", "stop", "--signal", "9", registryContainerId))
		if err != nil {
			log.Warn("unable to stop docker container: %v %v", registryContainerId, err)

			_, err = run.Command("docker", run.Args("rm", "--force", registryContainerId))
			if err != nil {
				log.Warn("unable to remove docker container: %v %v", registryContainerId, err)
			}
		}
	})

	// use the local registry as the cache target
	hostPort = fmt.Sprintf("localhost:%v", randomPort)
	require.SetAndRestore(t, &CachePrefix, hostPort)
	return hostPort
}
