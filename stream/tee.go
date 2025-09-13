package stream

import (
	"errors"
	"io"
	"sync"

	. "github.com/anchore/go-make/lang"
)

type TeeWriter interface {
	io.Writer
	io.Closer
	AddWriter(w io.Writer)
	RemoveWriter(w io.Writer)
	Writers() []io.Writer
	SetWriters(writers ...io.Writer)
}

// Tee creates a TeeWriter that writes to all writers
func Tee(writers ...io.Writer) TeeWriter {
	return &teeWriter{writers: writers}
}

type teeWriter struct {
	lock    sync.Mutex
	writers []io.Writer
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var errs []error
	for _, w := range t.writers {
		_, err := w.Write(p)
		// TODO handle the case when bytes written != len(p)?
		errs = append(errs, err)
	}
	return len(p), errors.Join(errs...)
}

// Writers returns a copy of the current writers
func (t *teeWriter) Writers() []io.Writer {
	t.lock.Lock()
	defer t.lock.Unlock()

	return append([]io.Writer(nil), t.writers...)
}

// SetWriters updates the writers to the new set of provided writers, be sure to close any old writers when needed
func (t *teeWriter) SetWriters(writers ...io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.writers = writers
}

func (t *teeWriter) AddWriter(w io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.writers = append(t.writers, w)
}

func (t *teeWriter) RemoveWriter(w io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()

	Remove(t.writers, func(writer io.Writer) bool {
		return writer == w
	})
}

// Close closes any referenced writers
func (t *teeWriter) Close() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	var errs []error
	for _, w := range t.writers {
		if c, _ := w.(io.Closer); c != nil {
			errs = append(errs, c.Close())
		}
	}
	return errors.Join(errs...)
}

var _ interface {
	io.Writer
	io.Closer
} = (*teeWriter)(nil)
