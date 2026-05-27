package golint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

func init() {
	template.Globals["LocalPackage"] = func() string {
		gm := gomod.Read()
		if gm != nil && gm.Module != nil {
			return regexp.MustCompile(`([^/]+/[^/]+)/.*`).ReplaceAllString(gm.Module.Mod.Path, "$1")
		}
		return ""
	}
}

// Option extends run.Option for lint-specific configuration.
type Option run.Option

// SkipTests excludes test files from linting by adding --tests=false to golangci-lint.
func SkipTests() Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		if strings.Contains(cmd.Args[0], "golangci-lint") {
			cmd.Args = append(cmd.Args, "--tests=false")
		}
		return nil
	}
}

// Tasks creates the standard linting and formatting task group. Includes
// static-analysis, format, lint, and lint:fix tasks (with a lint-fix alias).
func Tasks(options ...Option) Task {
	return Task{
		Tasks: []Task{
			StaticAnalysisTask(options...),
			FormatTask(),
			LintTask(options...),
			LintFixTask(options...),
		},
	}
}

// StaticAnalysisTask creates a task that runs golangci-lint and bouncer for
// license checking. Also verifies go.mod is tidy (Go 1.23+) and checks for
// problematic filenames (containing ':').
func StaticAnalysisTask(options ...Option) Task {
	return Task{
		Name:        "static-analysis",
		Description: "run lint checks",
		RunsOn:      lang.List("default"),
		Run: func() {
			if hasModTidyDiff() {
				Run("go mod tidy -diff")
			}
			log.Debug("CWD: %s", file.Cwd())
			Run("golangci-lint run", toRunOpts(options)...)
			lang.Throw(findMalformedFilenames("."))
			Run(`bouncer check ./...`, toRunOpts(options)...)
		},
	}
}

func hasModTidyDiff() bool {
	gm := gomod.Read()
	if gm == nil || gm.Go == nil {
		return false
	}
	parts := strings.Split(gm.Go.Version, ".")
	if len(parts) < 2 {
		return false
	}
	return lang.Return(strconv.Atoi(parts[1])) >= 23
}

// FormatTask creates a task that formats Go code with golangci-lint's formatters
// (gofmt and whatever else the project's golangci config enables) and runs go mod
// tidy. When the project config doesn't enable an import-organizing formatter
// (gci or goimports), we fall back to gosimports so imports are still grouped.
func FormatTask() Task {
	return Task{
		Name:        "format",
		Description: "format all source files",
		Run: func() {
			Run("golangci-lint fmt")
			if !importFormatterEnabled() {
				// GCI has replaced gosimports, however, this is controlled in each repo within their configuration.
				// This will ensure gosimports is used when GCI is not enabled, so imports are still grouped.
				if template.Globals["LocalPackage"] != nil {
					Run(`gosimports -local {{LocalPackage}} -w .`)
				} else {
					Run(`gosimports -w .`)
				}
			}
			Run(`go mod tidy`)
		},
	}
}

// LintTask creates a task that runs golangci-lint without any fixers (no
// formatting, no --fix) — just the lint checks.
func LintTask(options ...Option) Task {
	return Task{
		Name:        "lint",
		Description: "run lint checks (no fixes)",
		Run: func() {
			Run("golangci-lint run", toRunOpts(options)...)
		},
	}
}

// LintFixTask creates a task that formats code and then runs golangci-lint
// with the --fix flag to automatically fix linting issues where possible.
// The legacy `lint-fix` name is kept as an alias.
func LintFixTask(options ...Option) Task {
	return Task{
		Name:         "lint:fix",
		Aliases:      lang.List("lint-fix"),
		Description:  "format and run lint fix",
		Dependencies: lang.List("format"),
		Run: func() {
			Run("golangci-lint run --fix", toRunOpts(options)...)
		},
	}
}

// toRunOpts converts lint Options into run.Options.
func toRunOpts(options []Option) []run.Option {
	out := make([]run.Option, len(options))
	for i, opt := range options {
		out[i] = run.Option(opt)
	}
	return out
}

func findMalformedFilenames(root string) error {
	paths, err := listCommittablePaths(root)
	if err != nil {
		return fmt.Errorf("error walking through files: %w", err)
	}

	var malformedFilenames []string
	for _, path := range paths {
		if strings.Contains(path, ":") {
			malformedFilenames = append(malformedFilenames, path)
		}
	}

	if len(malformedFilenames) > 0 {
		fmt.Println("\nfound unsupported filename characters:")
		for _, filename := range malformedFilenames {
			fmt.Println(filename)
		}
		return fmt.Errorf("\nerror: unsupported filename characters found")
	}

	return nil
}

// importFormatters are golangci-lint formatters that organize imports; when the
// project's config enables one of these, golangci-lint fmt already handles import
// grouping and we don't also need gosimports.
var importFormatters = []string{"gci", "goimports"}

// importFormatterEnabled reports whether the project's golangci-lint config
// enables a formatter that organizes imports. golangci-lint's own `formatters`
// subcommand resolves the effective config (discovered natively), so we don't
// have to parse arbitrary config ourselves.
func importFormatterEnabled() bool {
	out := Run("golangci-lint formatters --json", run.Quiet())
	for _, name := range importFormatters {
		if formatterEnabled(out, name) {
			return true
		}
	}
	return false
}

// formatterEnabled reports whether name appears in the Enabled list of
// `golangci-lint formatters --json` output. A parse failure is treated as "not
// enabled" so callers fall back to their non-golangci-lint behavior.
func formatterEnabled(formattersJSON, name string) bool {
	var result struct {
		Enabled []struct {
			Name string `json:"name"`
		}
	}
	if err := json.Unmarshal([]byte(formattersJSON), &result); err != nil {
		log.Debug("unable to parse golangci-lint formatters output: %v", err)
		return false
	}
	for _, f := range result.Enabled {
		if f.Name == name {
			return true
		}
	}
	return false
}

// listCommittablePaths returns the set of paths under root that could plausibly
// be committed: tracked files plus untracked-but-not-ignored files, as reported
// by `git ls-files`. Submodules are listed by their directory entry only — their
// inner files are never walked. Falls back to a plain filesystem walk when root
// is not inside a git working tree (or git is unavailable).
func listCommittablePaths(root string) ([]string, error) {
	if paths, ok := gitListFiles(root); ok {
		return paths, nil
	}

	var paths []string
	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

func gitListFiles(root string) ([]string, bool) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	raw := strings.TrimRight(string(out), "\x00")
	if raw == "" {
		return nil, true
	}
	return strings.Split(raw, "\x00"), true
}
