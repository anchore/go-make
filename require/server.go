package require

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anchore/go-make/lang"
)

func Server(t *testing.T, routes map[string]any) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := routes[r.URL.Path]
		switch handler := handler.(type) {
		case func(w http.ResponseWriter, r *http.Request):
			handler(w, r)
		case []byte:
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(handler)
			NoError(t, err)
		case string:
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(handler))
			NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func Gzip(contents []byte) []byte {
	out := bytes.Buffer{}

	gzw := gzip.NewWriter(&out)

	_ = lang.Return(gzw.Write(contents))

	lang.Throw(gzw.Close())
	return out.Bytes()
}

func Tar(files map[string][]byte) []byte {
	out := bytes.Buffer{}
	w := tar.NewWriter(&out)

	for fileName, content := range files {
		// create the tar file entry
		hdr := &tar.Header{
			Name: fileName,
			Mode: 0755,
			Size: int64(len(content)),
		}
		lang.Throw(w.WriteHeader(hdr))
		lang.Return(w.Write(content))
	}

	lang.Throw(w.Close())
	return out.Bytes()
}

func Zip(files map[string][]byte) []byte {
	out := bytes.Buffer{}
	w := zip.NewWriter(&out)

	for fileName, content := range files {
		// create the zip file entry
		f := lang.Return(w.Create(fileName))
		lang.Return(f.Write(content))
	}

	lang.Throw(w.Close())
	return out.Bytes()
}
