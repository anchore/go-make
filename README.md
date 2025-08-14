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
// file: .make/main.go
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

See also: [the build definition in this repository](.make/main.go) and 
[script tests](script/testdata).

### Q & A

**Q:** It's too verbose to type `go run -C .make .`

**A:** Most modern `make` implementations seem to support the wildcard,
so you can just run `make <task>` by copying [what we have in this repo](Makefile):
```makefile
.PHONY: *
.DEFAULT:
%:
	@go run -C .make . $@
```
If that doesn't work for you, you can generate a Makefile with all the targets
using the hidden `makefile` task. Or just use an alias.

**Q:** I see something like: `make: Nothing to be done for 'test'`

**A:** This is how `make` works when you have directory matching the task name.
Add an explicit `.PHONY` directive and make target to your `Makefile`, e.g:
```makefile
.PHONY: test
test:
	@go run -C .make . $@
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

**Q:** Do I need to put my files in `.make`?

**A:** No, we just do this as a convention. The only thing this affects is what you
specify in your `Makefile`, e.g. `@go -C <dir>`

**Q:** Why make this? Surely there are already build tools.

**A:** Yes, there are plenty of build tools.
We have a fairly small and specific set of tasks we want to keep consistent across many repositories
and allow running builds and tests on all platforms, and since we're already using Go,
this seemed like a fairly simple solution to leverage the existing module system for distribution.

For example, we used [Task](https://github.com/go-task/task), which works great but leads to
difficulties implementing functionality for Windows, since common *nix tools are not available,
like `grep`, and at present doesn't offer an ideal way to share task definitions.

It's perfectly fine to use additional tools to implement your own functionality,
such as Task or [Bitfield script](https://github.com/bitfield/script) [*](https://bitfieldconsulting.com/posts/scripting).
Although it does provide some utilities, this library is intended as a means of bootstrapping sharable
task definitions in a platform-agnostic manner.
