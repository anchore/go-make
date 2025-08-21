package node

import (
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
)

func Run(js string, args ...run.Option) string {
	return script.Run("node", run.Args("-e", js, "--input-type=commonjs", "--"), run.Options(args...))
}
