package make

import (
	"fmt"
	"os"
	"strings"

	"github.com/anchore/go-make/color"
)

type Task struct {
	Name  string
	Desc  string
	Quiet bool
	Deps  []string
	Run   func()
}

func RollupTask(name string, desc string, deps ...string) Task {
	return Task{
		Name: name,
		Desc: desc,
		Deps: deps,
	}
}

func Makefile(tasks ...Task) {
	defer HandleErrors()
	defer appendStackOnPanic()

	Cd(RootDir())

	t := taskRunner{}
	names := map[string]struct{}{}
	for i := range tasks {
		if _, ok := names[tasks[i].Name]; ok {
			panic(fmt.Errorf("duplicate task name: %s", tasks[i].Name))
		}
		t.tasks = append(t.tasks, &tasks[i])
		names[tasks[i].Name] = struct{}{}
	}

	t.tasks = append(t.tasks, &Task{
		Name: "help",
		Desc: "print this help message",
		Run:  t.Help,
	})

	t.tasks = append(t.tasks, &Task{
		Name: "makefile",
		Desc: "generate an explicit Makefile for all tasks",
		Run:  t.Makefile,
	})

	t.Run(os.Args[1:]...)
}

type taskRunner struct {
	tasks []*Task
	run   map[string]struct{}
}

func (t *taskRunner) Help() {
	fmt.Print("Tasks:", NewLine)
	sz := 0
	for _, t := range t.tasks {
		if len(t.Name) > sz {
			sz = len(t.Name)
		}
	}
	for _, t := range t.tasks {
		fmt.Printf("  * %s% *s - %s"+NewLine, t.Name, sz-len(t.Name), "", t.Desc)
	}
}

var startWd = Cwd()

func (t *taskRunner) Makefile() {
	buildCmdDir := strings.TrimLeft(strings.TrimPrefix(startWd, RepoRoot()), `\/`)
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

func (t *taskRunner) Run(args ...string) {
	allTasks := t.tasks
	if len(allTasks) == 0 {
		panic("no tasks defined")
	}
	if len(args) == 0 {
		// run the default/first task
		args = append(args, allTasks[0].Name)
	}
	for _, taskName := range args {
		t.runTask(taskName)
	}
}

func (t *taskRunner) find(name string) *Task {
	for _, task := range t.tasks {
		if task.Name == name {
			return task
		}
	}
	return nil
}

func (t *taskRunner) runTask(name string) {
	tsk := t.find(name)
	if tsk == nil {
		panic(fmt.Errorf("no task named: %s", color.Bold(color.Underline(name))))
	}
	if _, ok := t.run[name]; ok {
		return
	}
	if t.run == nil {
		t.run = map[string]struct{}{}
	}
	t.run[name] = struct{}{}
	for _, dep := range tsk.Deps {
		d := t.find(dep)
		if d == nil {
			panic(fmt.Errorf("no dependency named: %s specified for task: %s", dep, tsk.Name))
		}
		t.runTask(dep)
	}

	if tsk.Run != nil && !tsk.Quiet {
		Log(color.Green(color.Bold("-- %s --")), tsk.Name)
	}

	origLog := Log
	defer func() { Log = origLog }()
	Log = func(format string, args ...any) {
		if !tsk.Quiet {
			format = fmt.Sprintf(color.Green("[%s] "), tsk.Name) + format
		}
		origLog(format, args...)
	}

	if tsk.Run != nil {
		tsk.Run()
	}
}
