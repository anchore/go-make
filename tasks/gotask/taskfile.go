package gotask

import (
	"encoding/json"
	"os"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
)

// taskInfo is one entry from `task --list-all --json` output.
type taskInfo struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

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

// Tasks discovers tasks defined in the project's Taskfile.yaml (via
// `task --list-all --json`) and exposes them as go-make Tasks that forward
// to the `task` binary. Returns an empty Task group when no Taskfile.yaml
// is present so it can be embedded unconditionally in a Makefile.
func Tasks() Task {
	if file.FindParent(git.Root(), "Taskfile.yaml") == "" {
		return Task{}
	}

	out := lang.Return(run.Command("task", run.Args("--list-all", "--json"), run.Quiet()))
	listed := parseTaskListing(out)

	subtasks := make([]Task, 0, len(listed))
	for _, info := range listed {
		subtasks = append(subtasks, Task{
			Name:        info.Name,
			Description: info.Desc,
			Run: func() {
				Run("task", run.Args(info.Name))
			},
		})
	}
	return Task{Tasks: subtasks}
}

// parseTaskListing parses the JSON output of `task --list-all --json` into the
// list of tasks it describes. Returns nil for empty input or a payload with no
// tasks. Panics if the JSON is malformed.
func parseTaskListing(out string) []taskInfo {
	if out == "" {
		return nil
	}
	var parsed struct {
		Tasks []taskInfo `json:"tasks"`
	}
	lang.Throw(json.Unmarshal([]byte(out), &parsed))
	if len(parsed.Tasks) == 0 {
		return nil
	}
	return parsed.Tasks
}
