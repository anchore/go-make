package golint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

// func init() {
//	Globals["app"] = "go-make"
//	Globals["now"] = func() string {
//		return fmt.Sprintf("%v", time.Now())
//	}
// s}

func StaticAnalysis() Task {
	return Task{
		Name: "static-analysis",
		Desc: "run lint checks",
		Run: func() {
			Run("go mod tidy -diff") // TODO: valid only >= go 1.23, should we support previous versions with custom go function?
			Run("golangci-lint run")
			NoErr(findMalformedFilenames("."))
			Run(`bouncer check ./...`)
		},
	}
}

func FormatTask() Task {
	return Task{
		Name: "format",
		Desc: "format all source files",
		Run: func() {
			Run(`gofmt -w -s .`)
			Run(`gosimports -local github.com/anchore -w .`)
			Run(`go mod tidy`)
		},
	}
}

func LintFixTask() Task {
	return Task{
		Name: "lint-fix",
		Desc: "format and run lint fix",
		Deps: All("format"),
		Run: func() {
			Run("golangci-lint run --fix")
		},
	}
}

func findMalformedFilenames(root string) error {
	var malformedFilenames []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
