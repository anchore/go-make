package main

import (
	"fmt"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

func main() {
	Makefile(
		Task{
			Name:        "example-failure",
			Description: "an example task that fails a run call due to an invalid command",
			RunsOn:      lang.List("example-label"),
			Run: func() {
				log.Info("running some invalid command")
				Run("some-invalid-command", run.Args("--some", "args"))
			},
		},
		Task{
			Name:        "custom-exit-code",
			Description: "an example task that returns a custom exit code",
			RunsOn:      lang.List("example-label"),
			Run: func() {
				log.Info("returning some exit code")
				lang.Throw(lang.NewStackTraceError(fmt.Errorf("some error")).WithExitCode(123))
			},
		},
	)
}
