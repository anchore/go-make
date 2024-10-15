package main

import (
	. "github.com/anchore/go-make" //nolint:stylecheck
	"github.com/anchore/go-make/tasks"
)

func main() {
	Makefile(
		tasks.Format,
		tasks.LintFix,
		tasks.StaticAnalysis,
		tasks.UnitTest,
		tasks.TestAll,
	)
}
