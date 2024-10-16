package golint

import . "github.com/anchore/go-make" //nolint:stylecheck

func CheckLicensesTask() Task {
	return Task{
		Name: "check-licenses",
		Desc: "ensure dependencies have allowable licenses",
		Run: func() {
			Run(`bouncer check ./...`)
		},
	}
}
