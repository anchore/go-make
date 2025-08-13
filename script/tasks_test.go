package script

import (
	"bytes"
	"testing"

	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/run"
)

func Test_errorsIncludeStackTrace(t *testing.T) {
	stderr := bytes.Buffer{}
	err := lang.Catch(func() {
		run.Command("go", run.Args("run", "./testdata/failure-example", "example-failure"), run.Stderr(&stderr))
	})
	require.Error(t, err)
	require.Contains(t, stderr.String(), "error executing")

	// includes the failed command
	require.Contains(t, stderr.String(), "some-invalid-command")

	// includes a link to the file:line in the script where the error occurred -- IMPORTANT!
	require.Contains(t, stderr.String(), "main.go:20")
}
