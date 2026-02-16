package stream

import (
	"io"
	"regexp"
	"sync"
)

type Option func(*regexpScanner)

// Size is the guaranteed scan size to return results reliably up to this size
func Size(size int) Option {
	return func(r *regexpScanner) {
		// the buffer size is twice, so we are able to match expressions across the entire stream up to `size` bytes
		r.buf = make([]byte, size*2)
	}
}

func NewRegexpScanner(re *regexp.Regexp, opts ...Option) (io.Writer, chan map[string]string) {
	r := &regexpScanner{
		re:     re,
		result: make(chan map[string]string),
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.buf == nil {
		r.buf = make([]byte, defaultSize)
	}
	return r, r.result
}

const (
	defaultSize = 1024
)

type regexpScanner struct {
	result chan map[string]string
	re     *regexp.Regexp
	lock   sync.Mutex
	buf    []byte
	pos    int
}

func (r *regexpScanner) SetRegex(re *regexp.Regexp) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.re = re
}

func (r *regexpScanner) Write(p []byte) (n int, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	total := len(p)
	for len(p) != 0 {
		remain := len(r.buf) - r.pos
		if len(p) <= remain {
			// if the incoming buffer fits into the remaining space in the ring, just copy it in
			copy(r.buf[r.pos:], p)
			r.pos += len(p)
			p = nil
		} else {
			start := r.pos - len(r.buf)/2
			if start < 0 {
				start = 0
			}
			// copy half of buf to the beginning, so the next scan can find matches that span writes
			copy(r.buf, r.buf[start:r.pos])
			r.pos -= start

			remain = len(r.buf) - r.pos
			if remain > len(p) {
				remain = len(p)
			}

			// copy as much of p to our ring as we can
			copy(r.buf[r.pos:], p[:remain])
			p = p[remain:]
			r.pos += remain
		}

		// each iteration, test to find matches
		s := string(r.buf[:r.pos])
		indexes := r.re.FindAllStringSubmatchIndex(s, -1)
		if indexes != nil {
			lastMatch := 0
			names := r.re.SubexpNames()
			for _, match := range indexes {
				matches := map[string]string{}
				for i, name := range names {
					start := match[2*i]
					end := match[2*i+1]
					matches[name] = s[start:end]
					if end > lastMatch {
						lastMatch = end
					}
				}
				r.result <- matches
			}

			// don't want to return duplicate matches: remove all contents through the last match so subsequent
			// searches won't return it
			if lastMatch > 0 {
				copy(r.buf, r.buf[lastMatch:r.pos])
				r.pos -= lastMatch
			}
		}
	}
	return total, err
}
