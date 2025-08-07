# Go, make!

`gomake` is a golang-based scripting utility, consisting of a simple DSL to help with common development tasks,
including cross-platform development utilities and a task runner with minimal dependencies.

Some functionality expects certain binaries to be available on the path:
* `go` -- for running in the first place, but also some commands may invoke `go`
* `git` -- in order to get revision information and build certain dependencies
* `docker` -- for running container-based tasks (configurable for CLI compatible commands such as `podman`)

Other binaries used should be configured in a binny config (or `go.mod` `tools` section ** TODO **) and will be downloaded
as needed during execution.

## Example

```golang
// file: make/main.go
package main 

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
)

func main() {
	Makefile(
		golint.Tasks(), // centralize common task definitions 
		gotest.Tasks(),
		// define tasks that resemble configuration / scripts:
		Task{
			Name:         "build",
			Description:  "make a custom build task",
			Dependencies:  List("goreleaser:snapshot-buildfile"), // can ensure other specific tasks run first, like make dependencies
			Run: func() {
				// Run function supports: global template vars, quoting within strings,
				// downloading and referencing managed executables
				Run(`goreleaser build --config {{TmpDir}}/goreleaser.yaml --clean --snapshot --single-target`)
			},
		},
		Task{
			Name:         "custom-tests",
			Description:  "do some custom testing",
			RunsOn:       List("test"),  // runs whenever "test" runs, e.g. make test
			Run: func() {
				// failed commands have convenient links here, in this file
				Run(`go test ./test`)
			},
		},
	)
}
```

See also: [the build definition in this repository](build/main.go) and 
[script tests](script/testdata).

### Q & A

**Q:** I see something like: `make: Nothing to be done for 'test'`

**A:** This is how `make` works when you have directory matching the task name.
Add an explicit `.PHONY` directive and make target to your `Makefile`, e.g:
```makefile
.PHONY: test
test:
	@go run -C build . $@
```

**Q:** I have a `golangci-lint` linter rule: _no dot imports_

**A:** Use a configuration like this:
```yaml
    staticcheck:
      dot-import-whitelist:
        - github.com/anchore/go-make
        - github.com/anchore/go-make/lang
    revive:
      rules:
        - name: dot-imports
          arguments:
            - allowed-packages:
                - github.com/anchore/go-make
                - github.com/anchore/go-make/lang
```

**Q:** Why make this? Surely there are already build tools.

**A:** Yes, there are plenty of build tools.
We have a fairly small and specific set of tasks we want to keep consistent across many repositories
and allow running builds and tests on all platforms, and since we're already using Go,
this seemed like a fairly simple solution to leverage the existing module system for distribution.

For example, we used [Task](https://github.com/go-task/task), which works great but leads to
difficulties implementing functionality for Windows, since common *nix tools are not available,
like `grep`. And at present doesn't offer a great way to share task definitions, though
there appears to be something in the works.

It's perfectly fine to use additional tools to implement your own functionality,
such as Task or [Bitfield script](https://github.com/bitfield/script) [*](https://bitfieldconsulting.com/posts/scripting).
Although it does provide some utilities, this library is intended as a means of bootstrapping sharable
task definitions in a platform-agnostic manner.
