package gotest

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/script"
)

func FixtureTasks() script.Task {
	return script.Task{
		Name:        "fixtures",
		Description: "build test fixtures",
		RunsOn:      lang.List("unit"),
		Run: func() {
			for _, f := range file.FindAll(file.JoinPaths(script.RepoRoot(), "**/test-fixtures/Makefile")) {
				dir := filepath.Dir(f)
				file.InDir(dir, func() {
					log.Log("Building fixture %s", dir)
					script.Run("make")
				})
			}
		},
		Tasks: []script.Task{
			{
				Name:        "fixtures:clean",
				Description: "clean internal git test fixture caches",
				RunsOn:      lang.List("clean"),
				Run: func() {
					for _, f := range file.FindAll(file.JoinPaths(script.RepoRoot(), "**/test-fixtures/Makefile")) {
						dir := filepath.Dir(f)
						file.InDir(dir, func() {
							log.Log("Cleaning fixture %s", dir)
							// allow errors to continue
							log.Error(lang.Catch(func() {
								script.Run("make clean")
							}))
						})
					}
				},
			},
			{
				Name:        "fixtures:directories",
				Description: "list all fixture directories",
				Run: func() {
					repoRoot := script.RepoRoot()
					// find all direct subdirectories of our repoRoot's test-fixtures directories
					paths := file.FindAll(file.JoinPaths(repoRoot, "**/test-fixtures/*"))
					// only return subdirectories
					paths = lang.Remove(paths, func(path string) bool {
						return !file.IsDir(path)
					})
					// return relative paths
					paths = lang.Map(paths, func(path string) string {
						path = strings.TrimPrefix(path, repoRoot)
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
					// should this be "**/test-fixtures/Makefile" ?
					lang.Return(os.Stdout.WriteString(file.Fingerprint(file.JoinPaths(script.RepoRoot(), "**/test-fixtures/*"))))
				},
			},
		},
	}
}
