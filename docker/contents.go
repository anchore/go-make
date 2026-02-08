package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// CacheDir is the directory to use for extracting artifacts, if not set, defaults to the current directory
var CacheDir string

// ExportCached extracts the contents of the built container to a cache directory, efficiently downloading
// previously built caches, returning the cache directory path. optionally include globs to filter the contents.
// this function using a combination of docker and oras, the general process is:
//   - attempt an oras pull of the cache directory, if found, cache and return this
//   - docker buildx with output to the cache dir, with filtering
//   - oras push the cache directory
//   - return the cache directory
func ExportCached(dockerfile string, fileGlobs ...string) string {
	file.Require(dockerfile)
	absDockerfile := lang.Return(filepath.Abs(dockerfile))
	hash := file.Fingerprint(absDockerfile)

	// if we find a testdata or test-fixtures, use testdata/.cache as the base dir
	cacheDir := findCacheDir(absDockerfile, hash)

	log.Debug("checking cache dir: %v", cacheDir)

	// check if we have a cache dir and if it needs updating; return if not
	if file.IsDir(cacheDir) {
		log.Debug("fingerprint matches dockerfile; returning: %v", cacheDir)
		return cacheDir
	}

	imageName := imageCacheName(absDockerfile, "dir", hash)

	orasBinary := findOrasBinary()

	// first, try to pull with ORAS
	err := orasPull(orasBinary, imageName, cacheDir, false)
	if err != nil {
		log.Debug("oras pull  %v failed: %v", imageName, err)
	} else if !file.IsEmpty(cacheDir) {
		log.Debug("restored from oras cache:  %v to: %v", imageName, cacheDir)
		return cacheDir
	}

	// can't find the cache, build locally
	buildx(absDockerfile, cacheDir)

	// filter files to matching globs, if specified
	if len(fileGlobs) > 0 {
		keepFileGlobs(cacheDir, "", fileGlobs)
	}

	// if succeeded and we're running in CI, on the main repo, main branch, push to CI cache
	if PushImageCache {
		err = orasPush(orasBinary, imageName, cacheDir)
		if err != nil {
			log.Warn("unable to push image: %v", err)
		}
	}

	if err == nil && file.IsDir(cacheDir) {
		log.Debug("pulled from local build: %v", cacheDir)
		return cacheDir
	}

	panic(fmt.Errorf("unable to pull or build fixture image: %v, %w", imageName, err))
}

func buildx(absDockerfile, cacheDir string) {
	log.Info("cache miss, building with docker buildx: %s", absDockerfile)
	// docker buildx build command with the --output flag and the type=local
	lang.Return(run.Command("docker",
		// should this have other args by default, like --no-cache ?
		run.Args("buildx", "build", "-f", filepath.Base(absDockerfile), "--output", "type=local,dest="+cacheDir, "."),
		run.InDir(filepath.Dir(absDockerfile)),
	))
}

func findOrasBinary() string {
	// prefer a global oras binary
	orasBinary, err := exec.LookPath("oras")
	if err == nil && orasBinary != "" {
		return orasBinary
	}

	// fallback to binny downloading the oras binary
	file.InDir(git.Root(), func() {
		orasBinary = binny.ManagedToolPath("oras")
	})
	if orasBinary == "" {
		panic(fmt.Errorf("unable to find oras in path or binny config %v", git.Root()))
	}
	return orasBinary
}

func orasPull(orasBinary, imageName string, cacheDir string, isOCI bool) error {
	extraOrasArgs := orasLocalhostFlag(imageName)
	if isOCI {
		extraOrasArgs = append(extraOrasArgs, run.Args("--oci-layout"))
	}
	log.Debug("pulling image: %v to dir: %v", imageName, cacheDir)
	_, err := run.Command(orasBinary, run.Args("pull", "--output", cacheDir), run.Options(extraOrasArgs...), run.Args(imageName))
	if err != nil {
		return fmt.Errorf("unable to pull image with oras: %w", err)
	}
	return nil
}

func orasPush(orasBinary string, imageName string, cacheDir string) error {
	extraOrasArgs := orasLocalhostFlag(imageName)
	log.Info("pushing image: %v from dir: %v", imageName, cacheDir)
	_, err := run.Command(orasBinary, run.Args("push", imageName, "."), run.InDir(cacheDir), run.Options(extraOrasArgs...))
	if err != nil {
		return fmt.Errorf("unable to push image with oras: %w", err)
	}
	return nil
}

func orasLocalhostFlag(imageName string) []run.Option {
	if strings.HasPrefix(imageName, "localhost:") {
		return []run.Option{run.Args("--insecure")}
	}
	return nil
}

// keepFileGlobs filters the files in the cache dir to only those matching the globs
func keepFileGlobs(absDir, dirPath string, globs []string) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		log.Warn("unable to read cache dir: %v", err)
	}
nextEntry:
	for _, entry := range entries {
		absFile := filepath.Join(absDir, entry.Name())
		filePath := entry.Name()
		if dirPath != "" {
			filePath = path.Join(dirPath, filePath)
		}
		if entry.IsDir() {
			keepFileGlobs(absFile, filePath, globs)
			continue
		}
		// if this path matches any glob, keep it
		for _, glob := range globs {
			if matched, _ := doublestar.Match(glob, filePath); matched {
				continue nextEntry
			}
			log.Debug("removing file from cache: %v", filePath)
		}
		log.Debug("removing file from cache: %v", filePath)
		err = os.Remove(absFile)
		if err != nil {
			log.Warn("unable to remove file from cache: %v: %v", absFile, err)
		}
	}
}

// findCacheDir returns a cache dir to the dockerfile based on some arbitrary rules:
// <testdata/test-fixtures> paths follow with cache/<subpath>, others are in the same location with a `.cache` directory
func findCacheDir(absDockerfile, hash string) string {
	const testdataDir = string(os.PathSeparator) + "testdata" + string(os.PathSeparator)
	const testFixturesDir = string(os.PathSeparator) + "test-fixtures" + string(os.PathSeparator)
	for _, fixtureDir := range []string{testdataDir, testFixturesDir} {
		parts := strings.SplitN(absDockerfile, fixtureDir, 2)
		if len(parts) > 1 {
			subPath, fileName := filepath.Split(parts[len(parts)-1])
			if fileName != "Dockerfile" {
				subPath = filepath.Join(subPath, fileName)
			}
			cacheBase := CacheDir
			if cacheBase == "" {
				cacheBase = filepath.Join(parts[0]+fixtureDir[0:len(fixtureDir)-1], ".cache")
			}
			return filepath.Join(cacheBase, hash, subPath)
		}
	}
	if CacheDir != "" {
		return filepath.Join(CacheDir, hash)
	}
	return filepath.Join(filepath.Dir(absDockerfile), ".cache", hash)
}
