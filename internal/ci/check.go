package ci

import "os"

func EnsureInCI() {
	if os.Getenv("CI") != "true" {
		panic("this task must be run in CI (CI=true)")
	}
}
