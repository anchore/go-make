package gomake

import (
	"bytes"
	"testing"

	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/run"
)

func Test_taskAliasResolution(t *testing.T) {
	ran := 0
	r := taskRunner{}
	r.addTasks(Task{
		Name:        "lint:fix",
		Aliases:     []string{"lint-fix"},
		Description: "format and run lint fix",
		Run:         func() { ran++ },
	})

	// the alias resolves to the real task
	found := r.findByName("lint-fix")
	require.Equal(t, 1, len(found))
	require.Equal(t, "lint:fix", found[0].Name)

	// an unknown name resolves to nothing
	require.Equal(t, 0, len(r.findByName("nope")))

	// running via the alias executes the real task exactly once
	r.Run("lint-fix")
	require.Equal(t, 1, ran)
}

func Test_errorsIncludeStackTrace(t *testing.T) {
	stderr := bytes.Buffer{}
	_, err := run.Command("go", run.Args("run", "./testdata/failure-example", "example-failure"), run.Stderr(&stderr))
	require.Error(t, err)
	require.Contains(t, stderr.String(), "error executing")

	// includes the failed command
	require.Contains(t, stderr.String(), "some-invalid-command")

	// includes a link to the file:line in the script where the error occurred -- IMPORTANT!
	require.Contains(t, stderr.String(), "main.go:20")
}
