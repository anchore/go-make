package gotest

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

type Config struct {
	IncludeGlob  string
	ExcludeGlob  string
	Dependencies []string
	Verbose      bool
}

func defaultConfig() Config {
	return Config{
		IncludeGlob: "./...",
	}
}

type Option func(*Config)

func With(cfg Config) Option {
	return func(c *Config) {
		*c = cfg
	}
}

func WithDependencies(dependencies ...string) Option {
	return func(c *Config) {
		c.Dependencies = dependencies
	}
}

func WithInclude(packages string) Option {
	return func(c *Config) {
		c.IncludeGlob = packages
	}
}

func WithExclude(packages string) Option {
	return func(c *Config) {
		c.ExcludeGlob = packages
	}
}

func WithVerbose() Option {
	return func(c *Config) {
		c.Verbose = true
	}
}

func Test(name string, options ...Option) Task {
	cfg := defaultConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	return Task{
		Name:   name,
		Desc:   fmt.Sprintf("run %s tests", name),
		Deps:   cfg.Dependencies,
		Labels: All("test"),
		Run: func() {
			var args []string
			args = append(args, "go test")
			if cfg.Verbose {
				args = append(args, "-v")
			}
			args = append(args, selectPackages(cfg.IncludeGlob, cfg.ExcludeGlob)...)

			Run(strings.Join(args, " "))
		},
	}
}

func selectPackages(include, exclude string) []string {
	if exclude == "" {
		return []string{include}
	}

	var absDirs bytes.Buffer
	// TODO: cannot use {{"{{.Dir}}"}} as a -f arg, and escaping is not working
	RunWithOptions(fmt.Sprintf(`go list %s`, include), ExecOut(&absDirs))

	// split by newline, and use relpath with cwd to get the non-absolute path
	var dirs []string
	cwd := Cwd()
	for _, dir := range strings.Split(absDirs.String(), "\n") {
		p, err := filepath.Rel(cwd, dir)
		if err != nil {
			dirs = append(dirs, dir)
			continue
		}
		dirs = append(dirs, p)
	}

	var final []string
	for _, dir := range dirs {
		matched, err := filepath.Match(exclude, dir)
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
