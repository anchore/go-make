package golint

import (
	. "github.com/anchore/go-make"
)

func CheckLicensesTask() Task {
	return Task{
		Name:        "check-licenses",
		Description: "ensure dependencies have allowable licenses",
		Run: func() {
			Run(`bouncer check ./...`)
		},
	}
}
