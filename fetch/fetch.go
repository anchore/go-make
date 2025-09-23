package fetch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

type Option func(*fetchOptions) error

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

func Writer(writer io.Writer) Option {
	return func(opts *fetchOptions) error {
		opts.writer = writer
		return nil
	}
}

func Delete(urlString string, options ...Option) (err error) {
	_, err = Fetch(urlString, append(options,
		func(opts *fetchOptions) error {
			opts.req.Method = http.MethodDelete
			return nil
		},
	)...)
	return err
}

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

	log.Info("fetch: %s", urlString)
	log.Debug("  └─ headers: %v", req.Header)

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
