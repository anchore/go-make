package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
	"github.com/anchore/go-make/tasks/release"
)

func main() {
	Makefile(
		golint.Tasks(),
		gotest.Tasks(),
		release.Tasks(),
		Task{
			Name:        "generate:github-ssh-keys",
			Description: "refresh GitHub SSH known_hosts from the GitHub meta API",
			Run: func() {
				Run("go generate ./git/...", run.Stderr(os.Stderr), run.Stdout(os.Stdout))
			},
		},
		Task{
			Name:        "integration",
			Description: "run integration tests in Docker",
			Run: func() {
				dockerfile := "git/testdata/Dockerfile"
				cwd := file.Cwd()

				// use dockerfile content hash as image tag for cache busting
				dockerfileContent := file.Read(dockerfile)
				hash := sha256.Sum256([]byte(dockerfileContent))
				tag := hex.EncodeToString(hash[:])[:12]
				image := "go-make-integration-test:" + tag

				// build image if needed (rebuilds when Dockerfile changes)
				if Run("docker images -q "+image, run.Quiet()) == "" {
					Log("building Docker image %q...", image)
					Run("docker build -t " + image + " -f " + dockerfile + " .")
				}

				// run all integration tests in container
				Log("running integration tests in Docker...")
				Run("docker run --pull never --rm -t -v "+cwd+":/app -w /app -e IN_DOCKER=true "+image+" go test -v -tags=integration -run TestIntegration ./git/...", run.Stdout(os.Stdout), run.Stderr(os.Stderr))
			},
		}.RunOn("test"),
	)
}
