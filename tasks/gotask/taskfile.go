package gotask

import (
	"os"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
)

// RunTaskfile delegates execution to the "task" runner if a Taskfile.yaml exists
// in the project. This allows gradual migration from Task to go-make by forwarding
// unknown commands to the existing Taskfile. Does nothing if no Taskfile is found.
func RunTaskfile() {
	defer lang.AppendStackTraceToPanics()
	if file.FindParent(git.Root(), "Taskfile.yaml") == "" {
		return
	}
	Run("task", run.Args(os.Args[1:]...))
}
