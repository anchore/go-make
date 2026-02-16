package stream

import (
	"io"
	"strings"

	"github.com/anchore/go-make/log"
)

type Passthrough struct {
	io.Writer
}

func NewPassthrough(w io.Writer) *Passthrough {
	return &Passthrough{w}
}

func (w *Passthrough) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "start") {
		log.Info("start")
	}
	if strings.Contains(string(p), "ready") {
		log.Info("ready")
	}
	if strings.Contains(string(p), "ready for") {
		log.Info("ready for")
	}
	if strings.Contains(string(p), "start up") {
		log.Info("start up")
	}
	return w.Writer.Write(p)
}
