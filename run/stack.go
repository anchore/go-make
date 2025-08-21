package run

import (
	"os"
	"runtime/pprof"
	"time"

	"github.com/anchore/go-make/log"
)

func PeriodicStackTraces(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			log.Error(pprof.Lookup("goroutine").WriteTo(os.Stderr, 1))
		}
	}()
}
