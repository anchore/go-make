package tasks

import . "github.com/anchore/go-make" //nolint:stylecheck

var UnitTest = Task{
	Name: "unit",
	Desc: "run unit tests",
	Run: func() {
		Run(`go test ./...`)
	},
}

var TestAll = Task{
	Name: "test",
	Deps: All("unit"),
}
