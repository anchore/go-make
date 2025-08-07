package script

import (
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/template"
)

// RepoRoot returns the root directory of the repository, typically this found by the .git directory
func RepoRoot() string {
	return template.Render(config.RootDir)
}
