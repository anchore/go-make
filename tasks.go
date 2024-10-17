package make

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/anchore/go-make/color"
)

type Task struct {
	Name   string
	Desc   string
	Quiet  bool
	Deps   []string
	Labels []string
	Tasks  []Task
	Run    func()
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

	t.addTasks(tasks...)

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

func (t *taskRunner) addTasks(tasks ...Task) {
	for _, task := range tasks {
		if len(t.findByName(task.Name)) > 0 {
			panic(fmt.Errorf("duplicate task name: %s", task.Name))
		}
		if task.Name != "" {
			t.tasks = append(t.tasks, &task)
		}
		t.addTasks(task.Tasks...)
	}
}

type taskRunner struct {
	tasks []*Task
	run   map[string]struct{}
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

func (t *taskRunner) runTask(name string) {
	// each task is going to set the log prefix
	origLogPrefix := LogPrefix
	defer func() { LogPrefix = origLogPrefix }()

	tasks := t.find(name)
	if len(tasks) == 0 {
		panic(fmt.Errorf("no tasks named: %s", color.Bold(color.Underline(name))))
	}

	for _, tsk := range tasks {
		if _, ok := t.run[name]; ok {
			return
		}
		if t.run == nil {
			t.run = map[string]struct{}{}
		}
		t.run[name] = struct{}{}
		for _, dep := range tsk.Deps {
			if len(t.find(dep)) == 0 {
				panic(fmt.Errorf("no dependency named: %s specified for task: %s", dep, tsk.Name))
			}
			t.runTask(dep)
		}

		LogPrefix = fmt.Sprintf(color.Green("[%s] "), tsk.Name)

		if tsk.Run != nil {
			tsk.Run()
		}
	}
}

func (t *taskRunner) find(name string) []*Task {
	// add directly named tasks first, labeled tasks second
	return append(t.findByName(name), t.findByLabel(name)...)
}

func (t *taskRunner) findByName(name string) []*Task {
	var out []*Task
	for _, task := range t.tasks {
		if task.Name == name {
			out = append(out, task)
		}
	}
	return AllNotNil(out...)
}

func (t *taskRunner) findByLabel(name string) []*Task {
	var out []*Task
	for _, task := range t.tasks {
		if slices.Contains(task.Labels, name) {
			out = append(out, task)
		}
	}
	return AllNotNil(out...)
}
