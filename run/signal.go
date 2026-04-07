package run

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/anchore/go-make/log"
)

// HandleSignals sets up a top-level signal handler that cancels the run context on the first
// interrupt and force-exits on the second.
func HandleSignals() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signals
		log.Debug("received %v, cancelling...", sig)
		Cancel()

		// a second signal means the user really wants out
		<-signals
		log.Debug("received second signal, forcing exit")
		os.Exit(1)
	}()
}
