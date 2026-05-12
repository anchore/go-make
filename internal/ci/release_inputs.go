package ci

import (
	"os"
	"regexp"
)

func ReleaseInputs() (string, string) {
	deployKey := os.Getenv("DEPLOY_KEY")
	if deployKey == "" {
		panic("DEPLOY_KEY environment variable must be set")
	}

	tagName := os.Getenv("RELEASE_VERSION")
	if tagName == "" {
		panic("RELEASE_VERSION environment variable must be set")
	}

	// validate version format early before doing any work
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+`).MatchString(tagName) {
		panic("RELEASE_VERSION does not appear to be a valid semver (e.g. v1.2.3)")
	}

	return tagName, deployKey
}
