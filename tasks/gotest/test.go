package gotest

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

type Config struct {
	Level       string
	IncludeGlob string
	ExcludeGlob string
	Verbose     bool
}

func defaultConfig() Config {
	return Config{
		Level:       "test",
		IncludeGlob: "./...",
	}
}

type Option func(*Config)

func With(cfg Config) Option {
	return func(c *Config) {
		*c = cfg
	}
}

func WithLevel(name string) Option {
	return func(c *Config) {
		c.Level = name
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

func Test(options ...Option) Task {
	cfg := defaultConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	return Task{
		Name: cfg.Level,
		Desc: fmt.Sprintf("run %s tests", cfg.Level),
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
