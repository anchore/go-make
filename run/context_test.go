package run_test

import (
	"sync"
	"testing"
	"time"

	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/run"
)

func Test_Cancel(t *testing.T) {
	errored := false
	startTime := time.Now()
	sleepDone := sync.WaitGroup{}
	sleepDone.Add(1)
	sleepReady := sync.WaitGroup{}
	sleepReady.Add(1)
	go func() {
		defer sleepDone.Done()
		sleepReady.Done()
		_, err := run.Command("sleep", run.Args("60")) // sleep for 5 seconds
		errored = err != nil
	}()
	go func() {
		sleepReady.Wait()
		// let the sleep command run
		time.Sleep(100 * time.Millisecond)
		run.Cancel()
	}()
	sleepDone.Wait() // will wait for 1 minute if not canceled

	elapsed := time.Since(startTime)
	t.Log(elapsed)

	require.True(t, errored)
	require.True(t, elapsed < 1*time.Second)

	goOutput, err := run.Command("go", run.Args("help"))
	require.NoError(t, err)
	require.True(t, goOutput != "")
}
