package run

import (
	"bytes"
	"runtime/pprof"
	"time"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/log"
)

func PeriodicStackTraces(interval func() time.Duration) {
	go func() {
		for {
			time.Sleep(interval())
			log.Info(color.Blue("stack trace:"))
			buf := bytes.Buffer{}
			log.Error(pprof.Lookup("goroutine").WriteTo(&buf, 1))
			log.Info(buf.String())
		}
	}()
}

func Backoff(interval time.Duration) func() time.Duration {
	interval /= 2 // so the first iteration is the requested interval
	return func() time.Duration {
		interval *= 2
		return interval
	}
}
