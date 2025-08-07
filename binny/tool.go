package binny

import (
	"path/filepath"
	"runtime"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

func InstallAll() {
	run.Command("binny install -v")
}

func ToolPath(toolName string) string {
	toolPath := toolName
	if runtime.GOOS == "windows" {
		toolPath += ".exe"
	}
	p := filepath.Join(template.Render(config.ToolDir), toolPath)
	return p
}
