# go-make

A golang-based build script library, with a goals of having very few dependencies 
and enabling consistent cross-platform build scripts.

## Example

See [the build definition in this repository](make/main.go)

```golang
package main

import (
	. "github.com/anchore/go-make"
	. "github.com/anchore/go-make/tasks"
)

func main() {
  Makefile(
    LintFix, // share tasks
    Task{ // define tasks that feel somewhat like scripts
      Name: "format",
      Desc: "format all source files",
      Run: func() {
        Run(`gofmt -w -s .`)
        Run(`gosimports -local github.com/anchore -w .`)
        Run(`go mod tidy`)
      },
    },
  )
}
```
