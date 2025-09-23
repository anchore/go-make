package gotask

import (
	"os"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
)

func RunTaskfile() {
	defer lang.AppendStackTraceToPanics()
	if file.FindParent(git.Root(), "Taskfile.yaml") == "" {
		return
	}
	Run("task", run.Args(os.Args[1:]...))
}
