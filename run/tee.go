package run

import "io"

func TeeWriter(w1, w2 io.Writer) interface {
	io.Writer
	Reset(w1, w2 io.Writer)
} {
	return &teeWriter{w1: w1, w2: w2}
}

type teeWriter struct {
	w1, w2 io.Writer
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	if t.w2 == nil {
		if t.w1 == nil {
			return 0, nil
		}
		return t.w1.Write(p)
	}
	if t.w1 != nil {
		_, _ = t.w1.Write(p)
	}
	return t.w2.Write(p)
}

// Reset unsets writers and outputs remaining write calls to the given writer
func (t *teeWriter) Reset(w1, w2 io.Writer) {
	t.w1 = w1
	t.w2 = w2
}

var _ io.Writer = (*teeWriter)(nil)
