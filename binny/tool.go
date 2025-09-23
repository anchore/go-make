package binny

import (
	"path/filepath"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

func InstallAll() {
	lang.Return(run.Command(ManagedToolPath(CMD), run.Args("install", "-v")))
}

func ToolPath(toolName string) string {
	toolPath := toolName
	if config.Windows {
		toolPath += ".exe"
	}
	p := filepath.Join(template.Render(config.ToolDir), toolPath)
	return p
}
