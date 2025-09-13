package gomake

import (
	"embed"

	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/template"
)

//go:embed .binny.yaml
var defaultBinnyConfig embed.FS

func init() {
	binny.DefaultConfig(lang.Return(defaultBinnyConfig.Open(".binny.yaml")))
}

// RootDir returns the root directory of the repository; typically the repository root, located by the .git directory
func RootDir() string {
	return template.Render(config.RootDir)
}
