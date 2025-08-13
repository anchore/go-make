package gomake

import (
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/script"
)

type (
	Task = script.Task
)

func List[T any](items ...T) []T {
	return lang.List(items...)
}

var (
	Makefile = script.Makefile

	Run = script.Run

	Log = log.Log
)
