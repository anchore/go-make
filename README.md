# go-make

A golang-based build script library, with a goals of having very few dependencies for fast execution time 
and enabling consistent, cross-platform build scripts.

## Example

```golang
package main // file: make/main.go

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/golint"
	"github.com/anchore/go-make/gotest"
)

func main() {
	Makefile(
		golint.Tasks(), // centralize common task definitions 
		gotest.Test("unit"),
		// define tasks that feel somewhat like configuration / scripts:
		Task{
			Name: "build",
			Desc:  "make a custom build task",
			Deps:  All("goreleaser:snapshot-buildfile"), // can ensure other specific tasks run first, like make dependencies
			Run: func() {
				// Run function supports: global template vars, quoting within strings,
				// obtaining & providing binny-managed executable
				Run(`goreleaser build --config {{TmpDir}}/goreleaser.yaml --clean --snapshot --single-target`)
			},
		},
		Task{
			Name:  "custom-tests",
			Desc:  "do some custom formatting stuff",
			Label: All("test"),  // runs whenever "test" runs, e.g. make test
			Run: func() {
				// failed commands have convenient links here, in this file
				Run(`go test ./test`)
			},
		},
	)
}
```

Also, see [the build definition in this repository](make/main.go)
