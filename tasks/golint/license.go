package golint

import (
	"github.com/anchore/go-make/script"
)

func CheckLicensesTask() script.Task {
	return script.Task{
		Name:        "check-licenses",
		Description: "ensure dependencies have allowable licenses",
		Run: func() {
			script.Run(`bouncer check ./...`)
		},
	}
}
