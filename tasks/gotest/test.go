package gotest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/github"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// Tasks creates a test task that runs Go tests with coverage reporting.
// The task hooks into the "test" label, so it runs whenever "make test" is called.
// By default, it runs tests for all packages with coverage enabled and race detection
// in CI environments.
//
// Example:
//
//	gotest.Tasks()                           // default: run all tests
//	gotest.Tasks(gotest.Name("integration")) // named test suite
//	gotest.Tasks(gotest.ExcludeGlob("**/test/**")) // exclude paths
func Tasks(options ...Option) Task {
	cfg := defaultConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	return Task{
		Name:        cfg.Name,
		Description: fmt.Sprintf("run %s tests", cfg.Name),
		RunsOn:      Deps("test"),
		Run: func() {
			start := time.Now()
			args := Deps("test")
			if cfg.Verbose {
				args = append(args, "-v")
			}
			if cfg.RunFilter != "" {
				args = append(args, "-run", cfg.RunFilter)
			}
			args = append(args, selectPackages(cfg.IncludeGlob, cfg.ExcludeGlob)...)

			coverageFile := cfg.CoverageFile
			if cfg.Coverage {
				if coverageFile == "" {
					coverageDir, err := os.MkdirTemp(config.TmpDir, "cover-dir-")
					if err == nil {
						defer func() {
							log.Error(os.RemoveAll(coverageDir))
						}()
						coverageDir, err = filepath.Abs(coverageDir)
						if err == nil {
							coverageFile = filepath.Join(coverageDir, "cover.out")
						}
					}
				}
				args = append(args, "-coverprofile", coverageFile)
				args = append(args, "-covermode=atomic", "-coverpkg=./...")
				// add coverage tag to existing tags
				cfg.Tags = append(cfg.Tags, "coverage")
			}
			if len(cfg.Tags) > 0 {
				args = append(args, "-tags="+strings.Join(cfg.Tags, ","))
			}

			if cfg.Race {
				args = append(args, "-race")
			}

			Run("go", run.Args(args...), run.Stdout(os.Stderr), run.Env("GODEBUG", "dontfreezetheworld=1"))

			Log("Done running %s tests in %v", cfg.Name, time.Since(start))

			if coverageFile != "" && cfg.CoverageFile == "" {
				report := Run("go tool cover", run.Args("-func", coverageFile), run.Quiet())
				if cfg.Verbose {
					Log(" -------------- Coverage Report -------------- ")
					Log(report)
				} else {
					coverage := regexp.MustCompile(`total:[^\n%]+?(\d+\.\d+)%`).FindStringSubmatch(report)
					if len(coverage) > 1 {
						Log("Coverage: %s%%", coverage[1])
					} else {
						Log(" -------------- Coverage Report -------------- ")
						log.Error(fmt.Errorf("unable to find coverage percentage in report"))
						Log(report)
					}
				}
			}

			if coverageFile != "" && config.OS == "linux" {
				err := lang.Catch(func() {
					dir := filepath.Dir(coverageFile)
					lang.Return(github.NewClient().UploadArtifactDir(dir, github.UploadArtifactOption{
						ArtifactName: "code-coverage",
						Overwrite:    false, // we only need one, failures are logged but ignored
						Files:        []string{coverageFile},
					}))
				})
				if err != nil {
					log.Debug("error uploading coverage file: %v", err)
				}
			}
		},
	}
}

// Config holds configuration for the test task.
type Config struct {
	// Name identifies this test suite (e.g., "unit", "integration"). Used in logs.
	Name string
	// IncludeGlob specifies which packages to test (default: "./...").
	IncludeGlob string
	// ExcludeGlob filters out packages matching this pattern.
	ExcludeGlob string
	// Verbose enables verbose test output (-v flag).
	Verbose bool
	// Coverage enables coverage reporting (default: true).
	Coverage bool
	// CoverageFile specifies where to write coverage data. If empty, uses temp file.
	CoverageFile string
	// Race enables race detector (-race flag). Defaults to true in CI on non-Windows.
	Race bool
	// Tags specifies build tags to use during testing.
	Tags []string
	// RunFilter limits which tests run (-run flag pattern).
	RunFilter string
}

func defaultConfig() Config {
	return Config{
		Name:        "unit",
		IncludeGlob: "./...",
		Coverage:    true,
		Race:        config.CI && !config.Windows,
	}
}

// Option is a functional option for configuring test tasks.
type Option func(*Config)

// Name sets the test suite name (used in log output and task naming).
func Name(name string) Option {
	return func(c *Config) {
		c.Name = name
	}
}

// IncludeGlob sets which packages to include in testing (default: "./...").
func IncludeGlob(packages string) Option {
	return func(c *Config) {
		c.IncludeGlob = packages
	}
}

// ExcludeGlob sets a pattern to exclude packages from testing.
func ExcludeGlob(packages string) Option {
	return func(c *Config) {
		c.ExcludeGlob = packages
	}
}

// Verbose enables verbose test output.
func Verbose() Option {
	return func(c *Config) {
		c.Verbose = true
	}
}

// NoCoverage disables coverage reporting.
func NoCoverage() Option {
	return func(c *Config) {
		c.Coverage = false
	}
}

// Tags adds build tags to use during testing.
func Tags(tags ...string) Option {
	return func(c *Config) {
		c.Tags = append(c.Tags, tags...)
	}
}

// RunFilter sets a -run pattern to limit which tests execute.
func RunFilter(filter string) Option {
	return func(c *Config) {
		c.RunFilter = filter
	}
}

func selectPackages(include, exclude string) []string {
	if exclude == "" {
		return []string{include}
	}

	// TODO: cannot use {{"{{.Dir}}"}} as a -f arg, and escaping is not working
	absDirs := Run(`go list`, run.Args(include))

	// split by newline, and use relpath with cwd to get the non-absolute path
	var dirs []string
	cwd := file.Cwd()
	for _, dir := range strings.Split(absDirs, "\n") {
		p, err := filepath.Rel(cwd, dir)
		if err != nil {
			dirs = append(dirs, dir)
			continue
		}
		dirs = append(dirs, p)
	}

	var final []string
	for _, dir := range dirs {
		matched, err := doublestar.Match(exclude, dir)
		if err != nil {
			final = append(final, dir)
			continue
		}
		if !matched {
			final = append(final, dir)
		}
	}
	return final
}
