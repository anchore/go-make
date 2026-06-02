package fetch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/anchore/go-make/internal/redact"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// Option is a functional option for customizing HTTP fetch behavior.
type Option func(*fetchOptions) error

// Headers adds custom HTTP headers to the request. Can be called multiple times
// to accumulate headers.
func Headers(headers map[string]string) Option {
	return func(opts *fetchOptions) error {
		if opts.req.Header == nil {
			opts.req.Header = make(http.Header)
		}
		for k, v := range headers {
			opts.req.Header[k] = append(opts.req.Header[k], v)
		}
		return nil
	}
}

// redactHeaders returns a copy of h with the values of credential-bearing
// headers (Authorization, Cookie, ...) masked, so request headers can be logged
// for diagnostics without dumping the bearer token.
func redactHeaders(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, values := range h {
		if redact.IsSensitiveName(k) {
			out[k] = []string{redact.Mask}
			continue
		}
		out[k] = values
	}
	return out
}

// Writer redirects the response body to the provided writer instead of returning
// it as a string. Useful for downloading large files directly to disk.
func Writer(writer io.Writer) Option {
	return func(opts *fetchOptions) error {
		opts.writer = writer
		return nil
	}
}

// Delete performs an HTTP DELETE request to the specified URL.
// Returns an error if the response status code is >= 300.
func Delete(urlString string, options ...Option) (err error) {
	_, err = Fetch(urlString, append(options,
		func(opts *fetchOptions) error {
			opts.req.Method = http.MethodDelete
			return nil
		},
	)...)
	return err
}

// Fetch performs an HTTP GET request to the specified URL and returns the response
// body as a string. Use the Writer() option to redirect output to a writer instead.
// Returns an error if the response status code is >= 300.
func Fetch(urlString string, options ...Option) (contents string, err error) {
	u := lang.Return(url.Parse(urlString))

	req := &http.Request{
		Method: "GET",
		URL:    u,
	}

	client := *http.DefaultClient

	opts := fetchOptions{
		writer: nil,
		client: &client,
		req:    req,
	}

	for _, option := range options {
		lang.Throw(option(&opts))
	}

	log.Info("fetch: %s", redact.Secrets(urlString))
	log.Debug("  └─ headers: %v", redactHeaders(req.Header))

	rsp := lang.Return(client.Do(req)) //nolint:bodyclose
	defer lang.Close(rsp.Body, urlString)

	// TODO: add a StatusCheck option
	if rsp.StatusCode >= 300 {
		err = lang.NewStackTraceError(fmt.Errorf("error: %v '%v' fetching: %v", rsp.StatusCode, rsp.Status, urlString))
	}

	if opts.writer != nil {
		_ = lang.Return(io.Copy(opts.writer, rsp.Body))
		return "", err
	}

	return string(lang.Return(io.ReadAll(rsp.Body))), err
}

type fetchOptions struct {
	writer io.Writer
	client *http.Client
	req    *http.Request
}
