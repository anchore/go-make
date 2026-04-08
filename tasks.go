package gomake

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

// Task defines a unit of work in the build system. Tasks can have dependencies,
// respond to labels, contain subtasks, and execute arbitrary Go code.
type Task struct {
	// Name is the unique identifier for this task. Used for:
	//   - Running directly: `make taskname`
	//   - Specifying in Dependencies: Deps("taskname")
	//   - Referencing in RunsOn labels
	Name string

	// Description is shown in help output. Keep it brief and action-oriented.
	Description string

	// Dependencies lists tasks that must complete successfully before this task runs.
	// These tasks are "pulled in" as prerequisites. Use Deps() helper for cleaner syntax.
	//
	// Example: Dependencies: Deps("build", "lint")
	Dependencies []string

	// RunsOn lists label names that will trigger this task. When any task in this list runs,
	// this task will also run. This is the inverse of Dependencies - it "hooks" this task
	// to run as part of another task.
	//
	// Common labels include "test", "clean", "default", and "dependencies:update".
	//
	// Example: RunsOn: List("test") causes this task to run whenever "make test" is called.
	RunsOn []string

	// Tasks defines nested subtasks. Subtask names are automatically prefixed with the parent
	// name using ":" as separator. For example, a subtask named "snapshot" under a parent
	// named "release" becomes "release:snapshot".
	//
	// Subtasks can still hook into other tasks via RunsOn without the prefix.
	Tasks []Task

	// Run is the function that implements this task's behavior. If nil, the task acts as
	// a label/phase that other tasks can depend on or hook into.
	Run func()
}

// DependsOn adds task names as dependencies, returning a new Task for method chaining.
// The specified tasks will run before this task executes.
//
// Example:
//
//	myTask.DependsOn("build", "lint")
func (t Task) DependsOn(tasks ...string) Task {
	t.Dependencies = append(t.Dependencies, tasks...)
	return t
}

// RunOn adds labels that will trigger this task, returning a new Task for method chaining.
// When any of the specified tasks run, this task will also run.
//
// Example:
//
//	myTask.RunOn("test", "default")
func (t Task) RunOn(tasks ...string) Task {
	t.RunsOn = append(t.RunsOn, tasks...)
	return t
}

// Makefile is the main entry point for go-make. It registers all provided tasks,
// adds built-in tasks (help, clean, binny:*, etc.), sets up signal handling,
// and executes the requested task(s) from command-line arguments.
//
// Makefile handles:
//   - Signal handling for graceful shutdown (SIGINT, SIGTERM)
//   - Periodic stack traces in debug mode for diagnosing hangs
//   - Automatic cleanup via config.DoExit() on completion
//   - Panic recovery with formatted error output
//
// If no task is specified on the command line, "help" is run by default.
//
// Example:
//
//	func main() {
//	    Makefile(
//	        golint.Tasks(),
//	        gotest.Tasks(),
//	        Task{Name: "build", Run: func() { Run(`go build ./...`) }},
//	    )
//	}
func Makefile(tasks ...Task) {
	defer config.DoExit()
	run.HandleSignals()
	if config.Debug {
		run.PeriodicStackTraces(run.Backoff(30 * time.Second))
	}
	runTaskFile(tasks...)
}

func runTaskFile(tasks ...Task) {
	defer lang.HandleErrors()

	file.Cd(template.Render(config.RootDir))

	t := taskRunner{}

	t.addTasks(tasks...)

	t.tasks = append(t.tasks,
		&Task{
			Name:        "help",
			Description: "print this help message",
			Run:         t.Help,
		},
		&Task{
			Name:        "clean",
			Description: "clean all generated files",
		},
		&Task{
			Name:   "binny:clean",
			RunsOn: lang.List("clean"),
			Run: func() {
				file.Delete(".tool")
			},
		},
		&Task{
			Name:        "dependencies:update",
			Description: "update all dependencies",
		},
		&Task{
			Name:   "binny:update",
			RunsOn: lang.List("dependencies:update"),
			Run: func() {
				Run("binny update")
			},
		},
		&Task{
			Name: "binny:install",
			Run: func() {
				binny.InstallAll()
			},
		},
		&Task{
			Name: "debuginfo",
			Run: func() {
				log.Debug("ENV: %v", os.Environ())
				ciEventFile := os.Getenv("GITHUB_EVENT_PATH")
				if ciEventFile != "" {
					log.Debug("GitHub Action event:\n%s", log.FormatJSON(string(lang.Continue(os.ReadFile(ciEventFile))))) //nolint:gosec // G703: path from GITHUB_EVENT_PATH env var set by CI runner
				}
			},
		},
		&Task{
			Name: "dos2unix",
			Run: func() {
				files := "**/*.{go,sh,md,yml,yaml,js,json,txt}"
				if len(os.Args) > 2 {
					files = os.Args[2]
				}
				file.DosToUnix(files)
			},
		},
		&Task{
			Name:        "test",
			Description: "run all tests",
		},
		&Task{
			Name: "makefile",
			Run:  t.Makefile,
		},
	)

	args := os.Args[1:]
	if len(args) == 0 {
		args = append(args, "help")
	}
	t.Run(args...)
}

type taskRunner struct {
	tasks []*Task
	run   set[*Task]
}

func (t *taskRunner) addTasks(tasks ...Task) {
	for _, task := range tasks {
		t.tasks = append(t.tasks, &task)
		t.addTasks(task.Tasks...)
	}
}

func (t *taskRunner) Run(args ...string) {
	allTasks := t.tasks
	if len(allTasks) == 0 {
		panic("no tasks defined")
	}
	if len(args) == 0 {
		// run the default/first task
		args = append(args, allTasks[0].Name)
	}
	t.run = set[*Task]{}
	for _, taskName := range args {
		t.runTask(taskName)
	}
}

func (t *taskRunner) runTask(name string) {
	// each task is going to set the log prefix
	origLogPrefix := log.Prefix
	defer func() { log.Prefix = origLogPrefix }()

	tasks := t.findByName(name)
	if len(tasks) == 0 {
		panic(fmt.Errorf("no tasks named: %s", color.Bold(color.Underline(name))))
	}

	for _, tsk := range tasks {
		// don't re-run the same task
		if t.run.Contains(tsk) {
			continue
		}
		t.run.Add(tsk)
		for _, dep := range t.findByLabel(tsk.Name) {
			t.runTask(dep.Name)
		}
		for _, dep := range tsk.Dependencies {
			t.runTask(dep)
		}

		log.Prefix = fmt.Sprintf(color.Green("[%s] "), tsk.Name)

		if tsk.Run != nil {
			tsk.Run()
		}
	}
}

func (t *taskRunner) findByName(name string) []*Task {
	var out []*Task
	for _, task := range t.tasks {
		if task.Name == name {
			out = append(out, task)
		}
	}
	return out
}

func (t *taskRunner) findByLabel(name string) []*Task {
	var out []*Task
	for _, task := range t.tasks {
		if slices.Contains(task.RunsOn, name) {
			out = append(out, task)
		}
	}
	return out
}

func (t *taskRunner) Makefile() {
	buildCmdDir := strings.TrimLeft(strings.TrimPrefix(file.Cwd(), RootDir()), `\/`)
	for _, t := range t.tasks {
		fmt.Printf(".PHONY: %s\n", t.Name)
		fmt.Printf("%s:\n", t.Name)
		fmt.Printf("\t@go run -C %s . %s\n", buildCmdDir, t.Name)
	}
	// catch-all, could be the entire script except for FreeBSD
	fmt.Printf(".PHONY: *\n")
	fmt.Printf(".DEFAULT:\n")
	fmt.Printf("\t@go run -C %s . $@\n", buildCmdDir)
}
