package gotest

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// FixtureTasks creates tasks for building and managing test fixtures.
// It looks for Makefiles in test-fixtures and testdata directories and runs them.
// Hooks into "unit" to build fixtures before tests run.
//
// Created tasks:
//   - fixtures: builds all test fixtures (runs on "unit")
//   - fixtures:clean: cleans all fixture caches (runs on "clean")
//   - fixtures:directories: lists all fixture directories
//   - fixtures:fingerprint: outputs a hash of all fixture files for caching
func FixtureTasks() Task {
	return Task{
		Name:        "fixtures",
		Description: "build test fixtures",
		RunsOn:      lang.List("unit"),
		Run: func() {
			for _, f := range file.FindAll(file.JoinPaths(RootDir(), "**/{test-fixtures,testdata}/Makefile")) {
				dir := filepath.Dir(f)
				file.InDir(dir, func() {
					log.Info("Building fixture %s", dir)
					Run("make")
				})
			}
		},
		Tasks: []Task{
			{
				Name:        "fixtures:clean",
				Description: "clean internal git test fixture caches",
				RunsOn:      lang.List("clean"),
				Run: func() {
					for _, f := range file.FindAll(file.JoinPaths(RootDir(), "**/{test-fixtures,testdata}/Makefile")) {
						dir := filepath.Dir(f)
						file.InDir(dir, func() {
							log.Info("Cleaning fixture %s", dir)
							// allow errors to continue
							log.Error(lang.Catch(func() {
								Run("make clean")
							}))
						})
					}
				},
			},
			{
				Name:        "fixtures:directories",
				Description: "list all fixture directories",
				Run: func() {
					// find all direct subdirectories of our root dir's test-fixtures directories
					paths := file.FindAll(file.JoinPaths(RootDir(), "**/{test-fixtures,testdata}/*/.gitignore"))
					// return relative paths
					paths = lang.Map(paths, func(path string) string {
						path = filepath.Dir(path)
						path = strings.TrimPrefix(path, RootDir())
						path = filepath.ToSlash(path)
						path = strings.TrimPrefix(path, "/")
						return path
					})
					lang.Return(os.Stdout.WriteString(strings.Join(paths, "\n")))
				},
			},
			{
				Name:        "fixtures:fingerprint",
				Description: "get test fixtures cache fingerprint",
				Run: func() {
					// should this be ".../Makefile" ?
					lang.Return(os.Stdout.WriteString(file.Fingerprint(file.JoinPaths(RootDir(), "**/{test-fixtures,testdata}/*"))))
				},
			},
		},
	}
}
