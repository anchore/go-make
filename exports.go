package gomake

import (
	"github.com/anchore/go-make/log"
)

// Deps is a convenience function for creating string slices for Task.Dependencies.
// It provides clearer intent than using []string{} directly and makes task definitions
// more readable.
//
// Example:
//
//	Task{
//	    Name:         "test",
//	    Dependencies: Deps("build", "lint"),
//	}
func Deps(deps ...string) []string {
	return deps
}

// Log is a convenience alias for log.Info, intended for use within task Run functions
// to output informational messages during task execution.
//
// Example:
//
//	Run: func() {
//	    Log("Building version %s", version)
//	    Run(`go build ./...`)
//	}
var (
	Log = log.Info
)
