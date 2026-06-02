package gotask

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/goccy/go-yaml"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
)

// taskInfo is one task discovered in a Taskfile, with its namespace prefix (if
// it came from an include) already applied to Name.
type taskInfo struct {
	Name string
	Desc string
}

// rawTaskfile captures only the parts of a Taskfile we care about for listing:
// the task definitions and any includes. Values are decoded loosely (into any)
// because a task may be a map (with desc/internal) or a bare list of commands,
// and an include may be a string or a map.
type rawTaskfile struct {
	Includes map[string]any `yaml:"includes"`
	Tasks    map[string]any `yaml:"tasks"`
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

// Tasks discovers tasks defined in the project's Taskfile.yaml (and any local
// `includes:`) by parsing the YAML directly, and exposes them as go-make Tasks
// that forward to the `task` binary. This makes Taskfile tasks discoverable in
// `make help` and runnable as first-class go-make tasks during migration.
// Returns an empty Task group when no Taskfile.yaml is present so it can be
// embedded unconditionally in a Makefile.
//
// Discovery mirrors `task --list-all`: internal tasks and internal includes are
// omitted. Remote includes (http/git) are skipped, since listing them would
// require fetching — those tasks won't appear here, though RunTaskfile still
// forwards to them.
//
// PERFORMANCE: the Taskfile(s) are parsed at construction time, i.e. on every
// `make` invocation (even for native tasks). Parsing YAML is cheap but not free,
// so prefer this only while migrating away from Task; once tasks are ported to
// go-make, drop the Tasks() call to avoid the per-invocation cost.
//
// The optional globs select which discovered tasks to expose: a glob may be
// an exact task name ("build") or a pattern ("db:*"). A task is included when
// it matches any glob. When no globs are given, all tasks are exposed.
func Tasks(globs ...string) Task {
	taskfilePath := file.FindParent(git.Root(), "Taskfile.yaml")
	if taskfilePath == "" {
		return Task{}
	}

	listed := discoverTasks(taskfilePath)

	subtasks := make([]Task, 0, len(listed))
	for _, info := range listed {
		if !matchesAny(info.Name, globs) {
			continue
		}
		subtasks = append(subtasks, Task{
			Name:        info.Name,
			Description: info.Desc,
			Run: func() {
				// forward this specific task by name (not os.Args) so multi-task
				// and dependency-driven invocations dispatch the right task.
				Run("task", run.Args(info.Name))
			},
		})
	}
	return Task{Tasks: subtasks}
}

// matchesAny reports whether name matches any of the given glob patterns. An
// empty glob list matches everything. Patterns may be exact task names or
// doublestar globs (e.g. "db:*"). Panics if a glob is malformed.
func matchesAny(name string, globs []string) bool {
	if len(globs) == 0 {
		return true
	}
	for _, glob := range globs {
		if lang.Return(doublestar.Match(glob, name)) {
			return true
		}
	}
	return false
}

// discoverTasks parses the Taskfile at path and all of its local includes,
// returning the flattened, namespaced task list sorted by name. Panics on
// malformed YAML or unreadable referenced files.
func discoverTasks(path string) []taskInfo {
	var out []taskInfo
	collectTasks(path, "", nil, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// collectTasks appends the tasks defined in the Taskfile at path to out,
// prefixing each name with prefix (the accumulated include namespace), then
// recurses into local includes. ancestors holds the absolute paths in the
// current include chain so cycles terminate while diamonds are still followed.
func collectTasks(path, prefix string, ancestors []string, out *[]taskInfo) {
	abs := lang.Return(filepath.Abs(path))
	if slices.Contains(ancestors, abs) {
		return // cycle in the include graph
	}
	ancestors = append(ancestors, abs)

	var doc rawTaskfile
	lang.Throw(yaml.Unmarshal([]byte(file.Read(path)), &doc))

	for name, raw := range doc.Tasks {
		m, isMap := raw.(map[string]any)
		// `task` never lists internal tasks, even with --list-all.
		if internal, _ := m["internal"].(bool); isMap && internal {
			continue
		}
		desc, _ := m["desc"].(string) // empty for non-map (bare command list) forms
		*out = append(*out, taskInfo{Name: prefix + name, Desc: desc})
	}

	baseDir := filepath.Dir(path)
	for namespace, raw := range doc.Includes {
		inc := parseInclude(raw)
		// remote and internal includes aren't listable without fetching/running.
		if inc.remote || inc.internal {
			continue
		}
		incPath := resolveIncludePath(baseDir, inc.taskfile)
		if incPath == "" {
			continue // optional or otherwise unresolved include
		}
		childPrefix := prefix + namespace + ":"
		if inc.flatten {
			// flattened includes contribute their tasks without a namespace.
			childPrefix = prefix
		}
		collectTasks(incPath, childPrefix, ancestors, out)
	}
}

// includeRef is the normalized form of a Taskfile `includes:` entry.
type includeRef struct {
	taskfile string
	optional bool
	internal bool
	flatten  bool
	remote   bool
}

// parseInclude normalizes an include entry, which may be a bare string (the
// taskfile path) or a map with taskfile/optional/internal/flatten keys.
func parseInclude(raw any) includeRef {
	switch v := raw.(type) {
	case string:
		return includeRef{taskfile: v, remote: isRemoteInclude(v)}
	case map[string]any:
		taskfile, _ := v["taskfile"].(string)
		optional, _ := v["optional"].(bool)
		internal, _ := v["internal"].(bool)
		flatten, _ := v["flatten"].(bool)
		return includeRef{
			taskfile: taskfile,
			optional: optional,
			internal: internal,
			flatten:  flatten,
			remote:   isRemoteInclude(taskfile),
		}
	}
	return includeRef{}
}

// isRemoteInclude reports whether ref points at a remote Taskfile (http or git),
// which can't be listed without fetching it.
func isRemoteInclude(ref string) bool {
	return strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "git+") ||
		strings.Contains(ref, "git@")
}

// resolveIncludePath resolves an include's taskfile reference (relative to
// baseDir) to a concrete file. When the reference points at a directory, the
// conventional Taskfile names are tried in order. Returns "" when nothing is
// found, so optional/missing includes are simply skipped.
func resolveIncludePath(baseDir, ref string) string {
	if ref == "" {
		ref = "." // an empty/omitted taskfile means the directory itself
	}
	p := ref
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, ref)
	}
	if file.IsDir(p) {
		for _, name := range []string{"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml"} {
			if candidate := filepath.Join(p, name); file.Exists(candidate) {
				return candidate
			}
		}
		return ""
	}
	if file.Exists(p) {
		return p
	}
	return ""
}
