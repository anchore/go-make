package golint

import (
	. "github.com/anchore/go-make"
)

// CheckLicensesTask creates a task that verifies all dependencies have
// allowable licenses using the bouncer tool. Requires bouncer to be configured
// in .binny.yaml.
func CheckLicensesTask() Task {
	return Task{
		Name:        "check-licenses",
		Description: "ensure dependencies have allowable licenses",
		Run: func() {
			Run(`bouncer check ./...`)
		},
	}
}
