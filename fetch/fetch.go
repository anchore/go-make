package fetch

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
)

type Option func(context.Context, *http.Request) error

func Headers(headers map[string]string) Option {
	return func(_ context.Context, req *http.Request) error {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		for k, v := range headers {
			req.Header[k] = append(req.Header[k], v)
		}
		return nil
	}
}

func Writer(writer io.Writer) Option {
	return func(ctx context.Context, _ *http.Request) error {
		w, _ := ctx.Value(fetchWriter{}).(*io.Writer)
		*w = writer
		return nil
	}
}

func Fetch(urlString string, options ...Option) (contents string, statusCode int, statusLine string) {
	u := lang.Return(url.Parse(urlString))

	req := &http.Request{
		Method: "GET",
		URL:    u,
	}

	var writer io.Writer
	ctx := context.WithValue(config.Context, fetchWriter{}, &writer)

	for _, option := range options {
		lang.Throw(option(ctx, req))
	}

	rsp := lang.Return(http.DefaultClient.Do(req)) //nolint:bodyclose
	defer lang.Close(rsp.Body, urlString)

	if writer != nil {
		_ = lang.Return(io.Copy(writer, rsp.Body))
		return "", rsp.StatusCode, rsp.Status
	}

	return string(lang.Return(io.ReadAll(rsp.Body))), rsp.StatusCode, rsp.Status
}

type fetchWriter struct{}
