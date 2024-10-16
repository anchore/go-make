package gobuild

import (
	. "github.com/anchore/go-make" //nolint:stylecheck
)

func SnapshotTask() Task {
	return Task{
		Name: "snapshot",
		Desc: "build a snapshot release with goreleaser",
		Run: func() {
			Run(`goreleaser release --clean --snapshot --skip=publish --skip=sign`)
		},
	}
}
