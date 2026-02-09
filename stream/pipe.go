package stream

import (
	"io"
	"sync"
)

// Pipe creates a pipe that can be used to write to take the results of a command
// which accepts io.Writer for the output and pipe to an io.Reader with synchronization
// appropriately waiting for the reader to complete before continuing on the writer process
func Pipe(readerFn func(io.Reader)) io.Writer {
	pr, pw := io.Pipe()
	t := &piper{
		pipeW: pw,
	}

	t.rdrGrp.Add(1)

	// Need a separate goroutine for pipe
	go func() {
		defer t.rdrGrp.Done()
		readerFn(pr)
	}()

	return t
}

type piper struct {
	rdrGrp sync.WaitGroup
	pipeW  *io.PipeWriter
}

// Write satisfies the io.Writer interface
func (t *piper) Write(p []byte) (n int, err error) {
	return t.pipeW.Write(p)
}

// Close ensures the pipe is shut down correctly, wait for readerFn to complete
func (t *piper) Close() error {
	defer t.rdrGrp.Wait()
	return t.pipeW.Close()
}
