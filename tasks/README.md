# Tasks

Common task definitions used by Anchore Go builds. These packages provide pre-built
tasks for common development workflows.

## Available Packages

| Package | Description |
|---------|-------------|
| `gotest` | Go test execution with coverage reporting |
| `golint` | Linting (golangci-lint) and formatting (gofmt, gosimports) |
| `goreleaser` | Release builds with goreleaser |
| `release` | Changelog generation and GitHub release creation |
| `gotask` | Integration with Task (Taskfile.yaml) runner |

## Usage Examples

### Basic Test and Lint Setup

```go
package main

import (
    . "github.com/anchore/go-make"
    "github.com/anchore/go-make/tasks/golint"
    "github.com/anchore/go-make/tasks/gotest"
)

func main() {
    Makefile(
        golint.Tasks(),
        gotest.Tasks(),
    )
}
```

### Customized Test Configuration

```go
gotest.Tasks(
    gotest.Name("integration"),           // suite name for logs
    gotest.ExcludeGlob("**/e2e/**"),      // skip e2e tests
    gotest.Tags("integration"),            // build tag
    gotest.Verbose(),                      // show test output
)
```

### Release Configuration

```go
import "github.com/anchore/go-make/tasks/goreleaser"

Makefile(
    goreleaser.Tasks(),  // includes snapshot, ci-release, and release tasks
)
```

## External Tool Dependencies

These packages require certain tools to be available. Configure them in `.binny.yaml`:

| Package | Required Tools |
|---------|---------------|
| `golint` | golangci-lint, gosimports, bouncer |
| `goreleaser` | goreleaser, quill (optional), syft (optional) |
| `release` | chronicle, glow (optional), gh |

Example `.binny.yaml`:

```yaml
tools:
  - name: golangci-lint
    version:
      want: v2.0.0
    method: github-release
    with:
      repo: golangci/golangci-lint

  - name: chronicle
    version:
      want: v0.8.0
    method: github-release
    with:
      repo: anchore/chronicle
```

## Creating Custom Shared Tasks

To create reusable task packages for your organization:

```go
// myorg/tasks/deploy/deploy.go
package deploy

import (
    . "github.com/anchore/go-make"
)

type Option func(*Config)

type Config struct {
    Environment string
    DryRun      bool
}

func Environment(env string) Option {
    return func(c *Config) { c.Environment = env }
}

func DryRun() Option {
    return func(c *Config) { c.DryRun = true }
}

func Tasks(options ...Option) Task {
    cfg := Config{Environment: "staging"}
    for _, opt := range options {
        opt(&cfg)
    }

    return Task{
        Name:        "deploy",
        Description: "deploy to " + cfg.Environment,
        Run: func() {
            if cfg.DryRun {
                Log("Would deploy to %s", cfg.Environment)
                return
            }
            Run(`kubectl apply -f manifests/`)
        },
    }
}
```

Usage:

```go
import "myorg/tasks/deploy"

Makefile(
    deploy.Tasks(deploy.Environment("production")),
)
```
