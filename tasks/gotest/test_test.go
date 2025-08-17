package gotest_test

import (
	"testing"

	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/tasks/gotest"
)

func Test_Task(t *testing.T) {
	task := gotest.Tasks(
		gotest.Name("secondary"),
	)

	require.Equal(t, "secondary", task.Name)
	require.Contains(t, task.Description, "secondary")

	cfg := gotest.Config{}

	gotest.Verbose()(&cfg)
	require.Equal(t, true, cfg.Verbose)

	gotest.IncludeGlob("**/*")(&cfg)
	require.Equal(t, "**/*", cfg.IncludeGlob)

	gotest.ExcludeGlob("**/*skip*")(&cfg)
	require.Equal(t, "**/*skip*", cfg.ExcludeGlob)
}
