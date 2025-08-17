package gotest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/run"
)

func Tasks(options ...Option) Task {
	cfg := defaultConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	return Task{
		Name:        cfg.Name,
		Description: fmt.Sprintf("run %s tests", cfg.Name),
		RunsOn:      List("test"),
		Run: func() {
			start := time.Now()
			args := List("test")
			if cfg.Verbose {
				args = append(args, "-v")
			}
			args = append(args, selectPackages(cfg.IncludeGlob, cfg.ExcludeGlob)...)

			Run("go", run.Args(args...), run.Stdout(os.Stderr))

			Log("Done running %s tests in %v", cfg.Name, time.Since(start))
		},
	}
}

type Config struct {
	Name        string
	IncludeGlob string
	ExcludeGlob string
	Verbose     bool
}

func defaultConfig() Config {
	return Config{
		Name:        "unit",
		IncludeGlob: "./...",
	}
}

type Option func(*Config)

func Name(name string) Option {
	return func(c *Config) {
		c.Name = name
	}
}

func IncludeGlob(packages string) Option {
	return func(c *Config) {
		c.IncludeGlob = packages
	}
}

func ExcludeGlob(packages string) Option {
	return func(c *Config) {
		c.ExcludeGlob = packages
	}
}

func Verbose() Option {
	return func(c *Config) {
		c.Verbose = true
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
