package make

import (
	"io"
	"net/http"
	"net/url"
)

type FetchOption func(req *http.Request)

func FetchHeaders(headers map[string]string) FetchOption {
	return func(req *http.Request) {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		for k, v := range headers {
			req.Header[k] = append(req.Header[k], v)
		}
	}
}

func Fetch(urlString string, options ...FetchOption) (contents []byte, statusCode int, statusLine string) {
	u := Get(url.Parse(urlString))

	req := &http.Request{
		Method: "GET",
		URL:    u,
	}
	for _, option := range options {
		option(req)
	}

	rsp := Get(http.DefaultClient.Do(req)) //nolint:bodyclose
	defer Close(rsp.Body)

	return Get(io.ReadAll(rsp.Body)), rsp.StatusCode, rsp.Status
}
