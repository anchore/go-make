package script

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/anchore/go-make/color"
)

func (t *taskRunner) Help() {
	fmt.Print("Tasks:\n")
	sz := 0
	for _, t := range t.tasks {
		if len(t.Name) > sz {
			sz = len(t.Name)
		}
	}

	allTaskNames := set[string]{}
	for _, task := range t.tasks {
		allTaskNames.Add(task.Name)
		for _, label := range task.RunsOn {
			allTaskNames.Add(label)
		}
	}

	runs := func(taskNames set[string]) string {
		if len(taskNames) == 0 {
			return ""
		}
		return fmt.Sprintf("(runs: %s)", color.Grey(strings.Join(slices.Collect(taskNames.Sorted()), ", ")))
	}

	for taskName := range allTaskNames.Sorted() {
		description := ""
		deps := set[string]{}
		for _, task := range t.findByName(taskName) {
			if description == "" {
				description = task.Description
			} else {
				description += "; " + task.Description
			}
			for _, label := range task.Dependencies {
				deps.Add(label)
			}
		}

		for _, task := range t.findByLabel(taskName) {
			deps.Add(task.Name)
			if description == "" {
				description = task.Description
			}
		}

		if description == "" {
			continue
		}
		fmt.Printf("  * %s% *s - %s %s\n", color.Green(taskName), sz-len(taskName), "", description, runs(deps))
	}
}

type set[T cmp.Ordered] map[T]struct{}

func (s set[T]) Add(items ...T) {
	for _, item := range items {
		s[item] = struct{}{}
	}
}

func (s set[T]) Contains(item T) bool {
	_, ok := s[item]
	return ok
}

func (s set[T]) Sorted() iter.Seq[T] {
	items := slices.Collect(maps.Keys(s))
	slices.Sort(items)
	return func(yield func(T) bool) {
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	}
}
