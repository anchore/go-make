package ci

import (
	"os"
	"regexp"

	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// ReleaseTagInput returns the validated release tag name from the
// RELEASE_VERSION environment variable. Panics if missing or not semver.
func ReleaseTagInput() string {
	tagName := os.Getenv("RELEASE_VERSION")
	if tagName == "" {
		panic("RELEASE_VERSION environment variable must be set")
	}

	// validate version format early before doing any work
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+`).MatchString(tagName) {
		panic("RELEASE_VERSION does not appear to be a valid semver (e.g. v1.2.3)")
	}

	return tagName
}

// ReleasePushCredentials returns the credentials used to push a release tag.
// Reads TAG_TOKEN (preferred) and the legacy DEPLOY_KEY. Exactly one of the
// returned values will be non-empty: when both env vars are set, TAG_TOKEN
// wins and a warning is logged so the workflow can be cleaned up.
// Panics if neither credential is set.
//
// As a defense-in-depth measure, both TAG_TOKEN and DEPLOY_KEY are unset from
// the process environment after being read. This keeps the credential confined
// to the returned strings (and from there, to the git child process during the
// push) rather than letting it be inherited by every subsequent child process
// (goreleaser, gh, etc.) that go-make spawns.
func ReleasePushCredentials() (deployKey, tagToken string) {
	deployKey = os.Getenv("DEPLOY_KEY")
	tagToken = os.Getenv("TAG_TOKEN")

	// purge from os.Environ() so they are not inherited by later children. Do
	// this unconditionally, before any validation panic, so a misconfigured
	// workflow still doesn't leak the credential downstream.
	lang.Throw(os.Unsetenv("DEPLOY_KEY"))
	lang.Throw(os.Unsetenv("TAG_TOKEN"))

	if deployKey == "" && tagToken == "" {
		panic("either TAG_TOKEN or DEPLOY_KEY environment variable must be set")
	}

	if tagToken != "" && deployKey != "" {
		log.Warn("both TAG_TOKEN and DEPLOY_KEY are set; using TAG_TOKEN (DEPLOY_KEY has been unset from the environment)")
		deployKey = ""
	}

	return deployKey, tagToken
}
