package run

import (
	"errors"
	"io"
	"sync"
)

func TeeWriter(w1, w2 io.Writer) interface {
	io.Writer
	Reset(w1, w2 io.Writer)
} {
	return &teeWriter{w1: w1, w2: w2}
}

type teeWriter struct {
	lock   sync.Mutex
	w1, w2 io.Writer
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.w1 != nil {
		_, _ = t.w1.Write(p)
	}
	if t.w2 != nil {
		return t.w2.Write(p)
	}
	return len(p), nil
}

// Reset unsets writers and outputs remaining write calls to the given writer
func (t *teeWriter) Reset(w1, w2 io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.w1 = w1
	t.w2 = w2
}

// Close closes any referenced writers
func (t *teeWriter) Close() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	var err1, err2 error
	if c1, _ := t.w1.(io.Closer); c1 != nil {
		err1 = c1.Close()
	}
	if c2, _ := t.w2.(io.Closer); c2 != nil {
		err2 = c2.Close()
	}
	return errors.Join(err1, err2)
}

var _ interface {
	io.Writer
	io.Closer
} = (*teeWriter)(nil)
