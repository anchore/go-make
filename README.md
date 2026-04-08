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

## How Tool Versions Work

go-make uses a two-tier system for tool versions with local overrides:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Your Project                                │
│  ┌─────────────────┐                                            │
│  │ .binny.yaml     │  ← Local overrides (optional)              │
│  │ golangci-lint:  │    Takes precedence if defined             │
│  │   v1.55.0       │                                            │
│  └────────┬────────┘                                            │
│           │                                                     │
│           ▼ overrides                                           │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ go-make module (in Go module cache)                         ││
│  │  ┌─────────────────┐                                        ││
│  │  │ .binny.yaml     │  ← Embedded defaults                   ││
│  │  │ golangci-lint:  │    Used when no local override         ││
│  │  │   v2.11.4       │                                        ││
│  │  └─────────────────┘                                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

Specifically we hav ethe following capabilities:

1. **Embedded defaults**: go-make's `.binny.yaml` is embedded into the module using Go's `//go:embed` directive. When you import go-make, this config is in your Go module cache alongside the source.

2. **Local overrides**: If your project has a `.binny.yaml`, those versions take precedence.

3. **Automatic installation**: When a task uses a tool, go-make checks your local config first, falls back to embedded defaults, then uses [binny](https://github.com/anchore/binny) to install.


To override `go-make`'s version for a specific tool simply create a `.binny.yaml` in your project root:

```yaml
tools:
  - name: golangci-lint
    version:
      want: v1.55.0  # Use this instead of go-make's default
    method: github-release
    with:
      repo: golangci/golangci-lint
```

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

## Task DSL Reference

### Task Structure

A `Task` is the fundamental building block for defining work:

```go
Task{
    Name:         "build",           // unique identifier for the task
    Description:  "build the app",   // shown in help output
    Dependencies: Deps("clean"),     // tasks that must run first
    RunsOn:       List("default"),   // labels that trigger this task
    Tasks:        []Task{...},       // nested subtasks
    Run: func() {
        // task implementation
    },
}
```

### Dependencies vs RunsOn

These two fields serve opposite purposes:

- **Dependencies**: Lists tasks that must complete *before* this task runs. Use when your task requires another task's output.
  ```go
  Task{
      Name:         "test",
      Dependencies: Deps("build"),  // build runs before test
  }
  ```

- **RunsOn**: Lists labels (task names) that will cause this task to run. Use when your task should automatically run as part of another task.
  ```go
  Task{
      Name:   "unit-tests",
      RunsOn: List("test"),  // runs whenever "make test" is called
  }
  ```

Think of it this way: `Dependencies` pulls tasks to run before you, while `RunsOn` hooks your task to run when another task is invoked.

### Hierarchical Tasks

Use `Task.Tasks` to group related tasks. Subtasks are automatically prefixed with the parent name:

```go
Task{
    Name: "release",
    Tasks: []Task{
        {Name: "snapshot"},      // becomes "release:snapshot"
        {Name: "ci-release"},    // becomes "release:ci-release"
    },
}
```

### Builder Methods

Tasks support method chaining for convenience:

```go
myTask.DependsOn("other-task")   // adds to Dependencies
myTask.RunOn("label")            // adds to RunsOn
```

## Template Variables

Commands passed to `Run()` support Go template syntax with the following built-in variables:

| Variable | Description | Example Value |
|----------|-------------|---------------|
| `{{RootDir}}` | Repository root (where .git is located) | `/home/user/myproject` |
| `{{ToolDir}}` | Directory for managed tools | `/home/user/myproject/.tool` |
| `{{TmpDir}}` | Temporary directory for build artifacts | System temp or empty for default |
| `{{OS}}` | Current operating system | `linux`, `darwin`, `windows` |
| `{{Arch}}` | Current architecture | `amd64`, `arm64` |
| `{{GitRoot}}` | Same as RootDir (alias) | `/home/user/myproject` |

Example usage:

```go
Run(`goreleaser build --config {{TmpDir}}/goreleaser.yaml`)
Run(`{{ToolDir}}/mytool --version`)
```

### Custom Template Variables

Extend the template context via `template.Globals`:

```go
import "github.com/anchore/go-make/template"

func init() {
    template.Globals["Version"] = func() string {
        return "1.0.0"
    }
}

// Later in a task:
Run(`echo "Building version {{Version}}"`)
```

## Built-in Tasks

go-make automatically adds these tasks to every Makefile:

| Task | Description |
|------|-------------|
| `help` | Prints all available tasks and descriptions |
| `clean` | Meta-task label for cleanup (no default action) |
| `binny:clean` | Deletes the `.tool` directory (runs on `clean`) |
| `binny:update` | Updates all managed tools (runs on `dependencies:update`) |
| `binny:install` | Installs all configured tools |
| `dependencies:update` | Meta-task label for dependency updates |
| `debuginfo` | Outputs environment variables and GitHub Actions event data |
| `dos2unix` | Converts CRLF to LF in text files (supports glob argument) |
| `test` | Meta-task label for tests (no default action) |
| `makefile` | Generates a traditional Makefile with all defined targets |

**Meta-task labels** like `clean`, `test`, and `dependencies:update` have no default action but provide hooks for your tasks to attach to via `RunsOn`. For example:

```go
Task{
    Name:   "my-clean",
    RunsOn: List("clean"),  // runs when "make clean" is called
    Run: func() {
        file.Delete("build/")
    },
}
```

## Error Handling

### Default Behavior

`Run()` panics on command failure. This stops task execution immediately:

```go
Run(`go build ./...`)  // panics if build fails
```

### Handling Failures Gracefully

Use `run.NoFail()` to suppress panics and return an empty string on failure:

```go
result := Run(`git describe --tags`, run.NoFail())
if result == "" {
    // command failed, handle gracefully
}
```

### Control Flow Functions

The `lang` package provides panic-based control flow:

| Function | Behavior |
|----------|----------|
| `lang.Throw(err)` | Panics if `err` is non-nil |
| `lang.Return(val, err)` | Returns `val` if `err` is nil, otherwise panics |
| `lang.Continue(val, err)` | Returns `val` regardless, logs `err` without panicking |
| `lang.Catch(fn)` | Executes `fn`, catches any panic and returns it as an error |

Example:

```go
// panic if file can't be read
contents := lang.Return(os.ReadFile("config.yaml"))

// log error but continue
data := lang.Continue(fetchOptionalData())

// catch panics from a function
if err := lang.Catch(riskyOperation); err != nil {
    log.Info("operation failed: %v", err)
}
```

### Error Recovery

`lang.HandleErrors()` is automatically deferred in `Makefile()` to catch panics and print formatted error messages with stack traces.
