package gomake

import (
	"github.com/anchore/go-make/log"
)

func Deps(deps ...string) []string {
	return deps
}

var (
	Log = log.Info
)
