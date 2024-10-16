package main

import (
	. "github.com/anchore/go-make" //nolint:stylecheck
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
)

func main() {
	Makefile(
		RollupTask("default", "run all validations", "static-analysis", "test"),
		golint.FormatTask(),
		golint.LintFixTask(),
		golint.StaticAnalysisTask(),
		gotest.Test("unit"),
		RollupTask("test", "run all levels of test", "unit"),
	)
}
