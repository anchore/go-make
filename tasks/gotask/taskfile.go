package gotask

import (
	"os"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
)

func Run() {
	defer lang.AppendStackTraceToPanics()
	if file.FindParent(git.Root(), "Taskfile.yaml") == "" {
		return
	}
	script.Run("task", run.Args(os.Args[1:]...))
}
