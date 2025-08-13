package release

import (
	. "github.com/anchore/go-make"
)

func Tasks() Task {
	return Task{
		Tasks: []Task{
			ChangelogTask(),
		},
	}
}
