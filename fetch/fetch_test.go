package fetch

import (
	"net/http"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_Fetch(t *testing.T) {
	var lastHeaders http.Header
	serverURL := require.Server(t, map[string]any{
		"/file1": "file1 content",
		"/file2": func(w http.ResponseWriter, r *http.Request) {
			lastHeaders = r.Header
			w.WriteHeader(http.StatusOK)
		},
	})

	tests := []struct {
		path     string
		opts     []Option
		validate func(*testing.T, string, int, string)
	}{
		{
			path: "/file1",
			validate: func(t *testing.T, contents string, statusCode int, _ string) {
				require.Equal(t, http.StatusOK, statusCode)
				require.Equal(t, "file1 content", contents)
			},
		},
		{
			path: "/file2",
			opts: []Option{Headers(map[string]string{
				"X-Custom-Header": "the-value",
			})},
			validate: func(t *testing.T, _ string, statusCode int, _ string) {
				require.Equal(t, http.StatusOK, statusCode)
				require.Equal(t, "the-value", lastHeaders.Get("X-Custom-Header"))
			},
		},
		{
			path: "/file3",
			validate: func(t *testing.T, _ string, statusCode int, _ string) {
				require.Equal(t, http.StatusNotFound, statusCode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			contents, statusCode, statusLine := Fetch(serverURL+tt.path, tt.opts...)
			tt.validate(t, contents, statusCode, statusLine)
		})
	}
}
