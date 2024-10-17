package main

import (
	. "github.com/anchore/go-make" //nolint:stylecheck
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
)

func main() {
	Makefile(
		golint.Tasks(),
		gotest.Test("unit"),
	)
}
