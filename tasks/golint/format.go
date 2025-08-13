package golint

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
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

type Option run.Option

func SkipTests() Option {
	return func(ctx context.Context, cmd *exec.Cmd) error {
		if strings.Contains(cmd.Args[0], "golangci-lint") {
			cmd.Args = append(cmd.Args, "--tests=false")
		}
		return nil
	}
}

func Tasks(options ...Option) script.Task {
	return script.Task{
		Tasks: []script.Task{
			StaticAnalysisTask(options...),
			FormatTask(),
			LintFixTask(options...),
		},
	}
}

func StaticAnalysisTask(options ...Option) script.Task {
	return script.Task{
		Name:        "static-analysis",
		Description: "run lint checks",
		RunsOn:      lang.List("default"),
		Run: func() {
			if hasModTidyDiff() {
				script.Run("go mod tidy -diff")
			}
			log.Debug("CWD: %s", file.Cwd())
			script.Run("golangci-lint run", toRunOpts(options)...)
			lang.Throw(findMalformedFilenames("."))
			script.Run(`bouncer check ./...`, toRunOpts(options)...)
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

func FormatTask() script.Task {
	return script.Task{
		Name:        "format",
		Description: "format all source files",
		Run: func() {
			script.Run(`gofmt -w -s .`)
			if template.Globals["LocalPackage"] != nil {
				script.Run(`gosimports -local {{LocalPackage}} -w .`)
			} else {
				script.Run(`gosimports -w .`)
			}
			script.Run(`go mod tidy`)
		},
	}
}

func LintFixTask(options ...Option) script.Task {
	return script.Task{
		Name:         "lint-fix",
		Description:  "format and run lint fix",
		Dependencies: lang.List("format"),
		Run: func() {
			script.Run("golangci-lint run --fix", toRunOpts(options)...)
		},
	}
}

func toRunOpts(options []Option) []run.Option {
	var out []run.Option
	for _, opt := range options {
		out = append(out, run.Option(opt))
	}
	return out
}

func findMalformedFilenames(root string) error {
	var malformedFilenames []string

	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// check if the filename contains the ':' character
		if strings.Contains(path, ":") {
			malformedFilenames = append(malformedFilenames, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking through files: %w", err)
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
