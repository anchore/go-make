package gotest

import (
	"os"
	"path/filepath"

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
			for _, f := range file.FindAll(file.JoinPaths(script.RepoRoot(), "**", "test-fixtures", "Makefile")) {
				dir := filepath.Dir(f)
				file.InDir(dir, func() {
					log.Log("Building fixture %s", dir)
					script.Run("make")
				})
			}
		},
		Tasks: []script.Task{
			{
				Name:        "clean", // this only runs explicitly with fixtures:clean
				Description: "clean internal git test fixture caches",
				Run: func() {
					for _, f := range file.FindAll(file.JoinPaths(script.RepoRoot(), "**", "test-fixtures", "Makefile")) {
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
				Name:        "fixtures:fingerprint",
				Description: "get test fixtures cache fingerprint",
				Run: func() {
					_, _ = os.Stderr.WriteString("Returning fingerprint: " + file.Fingerprint("**/test-fixtures/*"))
					// should this be "**/test-fixtures/Makefile" ?
					lang.Return(os.Stdout.WriteString(file.Fingerprint(file.JoinPaths(script.RepoRoot(), "**", "test-fixtures", "*"))))
				},
			},
		},
	}
}
