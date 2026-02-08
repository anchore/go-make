package docker

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// CachePrefix may be set to override the default image name prefix derived from go.mod
var CachePrefix string

// PushImageCache if true, will push the built image to the configured / identified registry
var PushImageCache = os.Getenv("GOMAKE_PUSH_IMAGE_CACHE") == "true"

// PullCached provides content-addressable Docker image caching, generally via ghcr.io.
// It hashes the Dockerfile, tries to pull a pre-built image, falls back to
// a local build on cache miss, returning the full image name
func PullCached(dockerfile string) string {
	file.Require(dockerfile)
	absDockerfile := lang.Return(filepath.Abs(dockerfile))
	hash := file.Fingerprint(absDockerfile)
	imageName := imageCacheName(absDockerfile, "", hash)

	_, err := run.Command("docker", run.Args("pull", imageName), run.Quiet())
	if err == nil {
		log.Info("cache hit: %s pulled from registry", imageName)
		return imageName
	}

	_ = lang.Return(build(absDockerfile, imageName))

	if PushImageCache {
		log.Info("pushing image: %v", imageName)
		_, err = run.Command("docker", run.Args("push", imageName))
		if err != nil {
			log.Warn("unable to push image: %v", err)
		}
	}
	return imageName
}

func build(absDockerfile, imageName string) (containerId string, err error) {
	log.Info("cache miss, building: %s", imageName)
	return run.Command("docker",
		// should this have other args by default, like --no-cache ?
		run.Args("build", "-t", imageName, "-f", filepath.Base(absDockerfile), "."),
		run.InDir(filepath.Dir(absDockerfile)),
	)
}

func imageCachePrefix() string {
	if CachePrefix != "" {
		return CachePrefix
	}
	mod := gomod.Read()
	if mod != nil && mod.Module != nil {
		return repoName(mod.Module.Mod.Path)
	}
	return ""
}

// repoName extracts the github.com/owner/repo portion from a full module path,
// stripping any subdirectory suffix (e.g. github.com/owner/repo/sub -> github.com/owner/repo)
func repoName(modPath string) string {
	parts := strings.Split(modPath, "/")
	if len(parts) >= 3 && parts[0] == "github.com" {
		return strings.Join(parts[:3], "/")
	}
	return modPath
}

func imageCacheName(absDockerfile, contentType, hash string) string {
	repo := imageCachePrefix()
	// convert github.com/owner/repo to ghcr.io/owner/repo
	registry := strings.Replace(repo, "github.com", "ghcr.io", 1)

	dir, fileName := filepath.Split(absDockerfile)
	if fileName == "Dockerfile" {
		absDockerfile = dir
	}
	// get the dockerfile path relative to the git root
	root := git.Root()
	relPath := lang.Return(filepath.Rel(root, absDockerfile))
	relPath = filepath.ToSlash(relPath)
	relPath = strings.ToLower(relPath)

	if contentType != "" {
		relPath += "-" + contentType
	}

	return registry + "/" + relPath + ":" + hash
}
