package script

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/template"
)

type Task struct {
	// Name the name of the tasks, which is used to refer to it everywhere: running, specifying dependencies, etc.
	Name string

	// Description provides a brief summary or purpose of the task
	Description string

	// Dependencies is a list of tasks that will be executed and must complete successfully before this task executes
	Dependencies []string

	// RunsOn when a task in RunsOn is executed, it will cause this task to be executed as a dependency
	RunsOn []string

	// Tasks allows a hierarchy of tasks to be registered together; subtasks will be prefixed with the "<parent name>:",
	// but their name will be added to the phase matching the name itself, so `binny` -> `clean` results in
	// `binny:clean` and `clean` execution for the subtask.
	Tasks []Task

	// Run is the function to execute this task's functionality
	Run func()
}

// DependsOn adds the provided task names as dependencies to the current task
func (t Task) DependsOn(tasks ...string) Task {
	t.Dependencies = append(t.Dependencies, tasks...)
	return t
}

// RunOn adds the provided tasks to the RunsOn list
func (t Task) RunOn(tasks ...string) Task {
	t.RunsOn = append(t.RunsOn, tasks...)
	return t
}

// Makefile will execute the provided tasks much like make with dependencies,
// as per the Task behavior declared above
func Makefile(tasks ...Task) {
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

	t.Run(os.Args[1:]...)
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
	buildCmdDir := strings.TrimLeft(strings.TrimPrefix(file.Cwd(), RepoRoot()), `\/`)
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
