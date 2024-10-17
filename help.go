package make

import (
	"fmt"
	"strings"
)

func (t *taskRunner) Help() {
	fmt.Print("Tasks:", NewLine)
	sz := 0
	for _, t := range t.tasks {
		if len(t.Name) > sz {
			sz = len(t.Name)
		}
	}

	labelTasks := map[string][]string{}
	for _, task := range t.tasks {
		for _, label := range task.Labels {
			labelTasks[label] = append(labelTasks[label], task.Name)
		}
	}

	runs := func(taskNames []string) string {
		if len(taskNames) == 0 {
			return ""
		}
		return fmt.Sprintf("(runs: %s)", strings.Join(taskNames, ", "))
	}

	for _, t := range t.tasks {
		allTasks := labelTasks[t.Name]
		allTasks = remove(allTasks, t.Name)
		delete(labelTasks, t.Name)
		fmt.Printf("  * %s% *s - %s %s"+NewLine, t.Name, sz-len(t.Name), "", t.Desc, runs(allTasks))
	}

	for label, tasks := range sortedMapIter(labelTasks) {
		fmt.Printf("  * %s% *s %s"+NewLine, label, sz-len(label), "", runs(tasks))
	}
}
