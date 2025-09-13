package node

import (
	gomake "github.com/anchore/go-make"
	"github.com/anchore/go-make/run"
)

func Run(js string, args ...run.Option) string {
	return gomake.Run("node", run.Args("-e", js), run.Options(args...))
}
