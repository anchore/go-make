package release

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/anchore/go-make/require"
)

func TestFindMarkdownLayer(t *testing.T) {
	tests := []struct {
		name       string
		layers     []ociDescriptor
		wantDigest string
		wantFound  bool
	}{
		{
			name: "match by title annotation",
			layers: []ociDescriptor{
				{Digest: "sha256:json", Annotations: map[string]string{"org.opencontainers.image.title": "changelog.json"}},
				{Digest: "sha256:md", Annotations: map[string]string{"org.opencontainers.image.title": "CHANGELOG.md"}},
				{Digest: "sha256:ver", Annotations: map[string]string{"org.opencontainers.image.title": "VERSION"}},
			},
			wantDigest: "sha256:md",
			wantFound:  true,
		},
		{
			name: "fall back to markdown media type",
			layers: []ociDescriptor{
				{Digest: "sha256:json", MediaType: "application/json"},
				{Digest: "sha256:md", MediaType: "text/markdown"},
			},
			wantDigest: "sha256:md",
			wantFound:  true,
		},
		{
			name: "no markdown layer",
			layers: []ociDescriptor{
				{Digest: "sha256:json", Annotations: map[string]string{"org.opencontainers.image.title": "changelog.json"}},
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := findLayer(tt.layers, isMarkdownLayer)
			require.Equal(t, tt.wantFound, found)
			if !found {
				return
			}
			require.Equal(t, tt.wantDigest, got.Digest)
		})
	}
}

func TestOCIClientPullMarkdown(t *testing.T) {
	const (
		repo    = "anchore/changelog/test"
		ref     = "abcdef"
		blob    = "sha256:deadbeef"
		verBlob = "sha256:cafe"
		body    = "# Changelog\n\n- a great change\n"
		verBody = "v1.2.3\n"
	)

	tests := []struct {
		name          string
		handler       http.HandlerFunc
		want          cachedChangelog
		wantErr       require.ValidationError
		wantRetryable bool
	}{
		{
			name: "happy path with markdown and version layers",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/token"):
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case strings.HasSuffix(r.URL.Path, "/manifests/"+ref):
					require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
					_ = json.NewEncoder(w).Encode(ociManifest{
						Layers: []ociDescriptor{
							{Digest: blob, Annotations: map[string]string{"org.opencontainers.image.title": "CHANGELOG.md"}},
							{Digest: verBlob, Annotations: map[string]string{"org.opencontainers.image.title": "VERSION"}},
						},
					})
				case strings.HasSuffix(r.URL.Path, "/blobs/"+blob):
					_, _ = w.Write([]byte(body))
				case strings.HasSuffix(r.URL.Path, "/blobs/"+verBlob):
					_, _ = w.Write([]byte(verBody))
				default:
					http.Error(w, "not found", http.StatusNotFound)
				}
			},
			want: cachedChangelog{markdown: body, version: "v1.2.3"},
		},
		{
			name: "index is resolved to child manifest",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/token"):
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case strings.HasSuffix(r.URL.Path, "/manifests/"+ref):
					_ = json.NewEncoder(w).Encode(ociManifest{
						MediaType: "application/vnd.oci.image.index.v1+json",
						Manifests: []ociDescriptor{{Digest: "sha256:child"}},
					})
				case strings.HasSuffix(r.URL.Path, "/manifests/sha256:child"):
					_ = json.NewEncoder(w).Encode(ociManifest{
						Layers: []ociDescriptor{{
							Digest:      blob,
							Annotations: map[string]string{"org.opencontainers.image.title": "CHANGELOG.md"},
						}},
					})
				case strings.HasSuffix(r.URL.Path, "/blobs/"+blob):
					_, _ = w.Write([]byte(body))
				default:
					http.Error(w, "not found", http.StatusNotFound)
				}
			},
			want: cachedChangelog{markdown: body},
		},
		{
			name: "self-referential index errors at the depth limit",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/token"):
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				default:
					// every manifest is an index pointing back at another manifest, so
					// resolution would loop forever without the depth guard.
					_ = json.NewEncoder(w).Encode(ociManifest{
						MediaType: "application/vnd.oci.image.index.v1+json",
						Manifests: []ociDescriptor{{Digest: "sha256:loop"}},
					})
				}
			},
			wantErr:       require.Error,
			wantRetryable: true,
		},
		{
			name: "auth failure at token endpoint is not retryable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			},
			wantErr:       require.Error,
			wantRetryable: false,
		},
		{
			name: "auth failure pulling manifest is not retryable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/token") {
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
					return
				}
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			wantErr:       require.Error,
			wantRetryable: false,
		},
		{
			name: "not found is not retryable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/token") {
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
					return
				}
				http.Error(w, "no such artifact", http.StatusNotFound)
			},
			wantErr:       require.Error,
			wantRetryable: false,
		},
		{
			name: "server error is retryable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/token") {
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
					return
				}
				http.Error(w, "boom", http.StatusServiceUnavailable)
			},
			wantErr:       require.Error,
			wantRetryable: true,
		},
		{
			name: "transport drop mid-read is retryable, not a structural miss",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/token"):
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case strings.HasSuffix(r.URL.Path, "/manifests/"+ref):
					_ = json.NewEncoder(w).Encode(ociManifest{
						Layers: []ociDescriptor{{
							Digest:      blob,
							Annotations: map[string]string{"org.opencontainers.image.title": "CHANGELOG.md"},
						}},
					})
				case strings.HasSuffix(r.URL.Path, "/blobs/"+blob):
					// promise more bytes than we send, then drop the connection so the client's
					// read fails mid-stream instead of returning a complete body.
					w.Header().Set("Content-Length", "4096")
					_, _ = w.Write([]byte("# partial changelog"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					if hj, ok := w.(http.Hijacker); ok {
						conn, _, err := hj.Hijack()
						if err == nil {
							_ = conn.Close()
						}
					}
				default:
					http.Error(w, "not found", http.StatusNotFound)
				}
			},
			wantErr:       require.Error,
			wantRetryable: true,
		},
		{
			name: "missing markdown layer is a non-retryable miss",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/token"):
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				default:
					_ = json.NewEncoder(w).Encode(ociManifest{
						Layers: []ociDescriptor{{
							Digest:      "sha256:json",
							Annotations: map[string]string{"org.opencontainers.image.title": "changelog.json"},
						}},
					})
				}
			},
			wantErr:       require.Error,
			wantRetryable: false,
		},
		{
			name: "empty markdown is a non-retryable miss",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/token"):
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case strings.HasSuffix(r.URL.Path, "/manifests/"+ref):
					_ = json.NewEncoder(w).Encode(ociManifest{
						Layers: []ociDescriptor{{
							Digest:      blob,
							Annotations: map[string]string{"org.opencontainers.image.title": "CHANGELOG.md"},
						}},
					})
				case strings.HasSuffix(r.URL.Path, "/blobs/"+blob):
					_, _ = w.Write([]byte("   \n\t  "))
				default:
					http.Error(w, "not found", http.StatusNotFound)
				}
			},
			wantErr:       require.Error,
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			u, err := url.Parse(srv.URL)
			require.NoError(t, err)

			c := &ociClient{
				scheme:   u.Scheme,
				registry: u.Host,
				repo:     repo,
				ghToken:  "fake-token",
				http:     srv.Client(),
			}

			got, err := c.pull(context.Background(), ref)
			tt.wantErr.Validate(t, err)

			if err != nil {
				require.Equal(t, tt.wantRetryable, isRetryable(err))
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

// TestGetChangelogFallback exercises the fallback decision and version precedence: a cache hit
// lacking a version degrades to chronicle when the caller needs one, a usable cache hit is served
// from the cache, and a caller-supplied tag (the publish path) always wins over the cached version.
func TestGetChangelogFallback(t *testing.T) {
	tests := []struct {
		name         string
		cached       cachedChangelog
		fetchErr     error
		inputVersion string
		wantFallback bool
		wantShown    bool
		wantVersion  string
	}{
		{
			name:         "cache hit without version falls back when caller needs one",
			cached:       cachedChangelog{markdown: "md"},
			inputVersion: "",
			wantFallback: true,
			wantVersion:  "fell-back",
		},
		{
			name:         "cache hit with version is served from the cache",
			cached:       cachedChangelog{markdown: "md", version: "v1.2.3"},
			inputVersion: "",
			wantShown:    true,
			wantVersion:  "v1.2.3",
		},
		{
			name:         "cache miss falls back",
			fetchErr:     errors.New("boom"),
			inputVersion: "",
			wantFallback: true,
			wantVersion:  "fell-back",
		},
		{
			name:         "caller-supplied version wins when cache lacks one",
			cached:       cachedChangelog{markdown: "md"},
			inputVersion: "v9.9.9",
			wantShown:    true,
			wantVersion:  "v9.9.9",
		},
		{
			name:         "caller-supplied version wins over a differing cached version",
			cached:       cachedChangelog{markdown: "md", version: "v1.2.3"},
			inputVersion: "v9.9.9",
			wantShown:    true,
			wantVersion:  "v9.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fellBack, shown bool

			require.SetAndRestore(t, &fetchCachedChangelogFn, func() (cachedChangelog, error) {
				return tt.cached, tt.fetchErr
			})
			require.SetAndRestore(t, &fallbackChangelogFn, func(string) (string, string) {
				fellBack = true
				return changelogFile, "fell-back"
			})
			require.SetAndRestore(t, &writeAndShowFn, func(cachedChangelog) {
				shown = true
			})

			path, version := GetChangelog(tt.inputVersion)
			require.Equal(t, changelogFile, path)
			require.Equal(t, tt.wantVersion, version)
			require.Equal(t, tt.wantFallback, fellBack)
			require.Equal(t, tt.wantShown, shown)
		})
	}
}

func TestIsVersionLayer(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{title: "version", want: true},
		{title: "VERSION", want: true}, // matched case-insensitively via findLayer, but verify lowercased form
		{title: "changelog.version", want: true},
		{title: "path/to/version", want: true},
		{title: "changelog.md", want: false},
		{title: "changelog.json", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			require.Equal(t, tt.want, isVersionLayer(strings.ToLower(tt.title), ""))
		})
	}
}

func TestBasicAuth(t *testing.T) {
	// "user:pass" base64-encoded
	require.Equal(t, "dXNlcjpwYXNz", basicAuth("user", "pass"))
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "short is unchanged", in: "  hello  ", want: "hello"},
		{name: "long is truncated", in: strings.Repeat("x", 300), want: strings.Repeat("x", 256) + "..."},
		{name: "multi-byte runes are not split", in: strings.Repeat("é", 300), want: strings.Repeat("é", 256) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, truncate([]byte(tt.in)))
		})
	}
}
