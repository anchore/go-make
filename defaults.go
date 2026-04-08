package gomake

import (
	"embed"

	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/template"
)

// defaultBinnyConfig embeds go-make's .binny.yaml into the compiled binary.
//
// This enables "inherited" tool versions: when a project imports go-make, the
// Go module cache contains this .binny.yaml alongside the source code. The
// //go:embed directive embeds this file at compile time (relative to THIS file,
// not the importing project). At init(), these versions become defaults that
// can be overridden by the project's own .binny.yaml.
//
//go:embed .binny.yaml
var defaultBinnyConfig embed.FS

func init() {
	// register embedded tool versions as defaults; local .binny.yaml takes precedence
	binny.DefaultConfig(lang.Return(defaultBinnyConfig.Open(".binny.yaml")))
}

// RootDir returns the root directory of the repository; typically the repository root, located by the .git directory
func RootDir() string {
	return template.Render(config.RootDir)
}
