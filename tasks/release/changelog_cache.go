package release

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

const (
	// changelogCacheRegistry is the OCI registry hosting the pre-built changelog artifacts.
	changelogCacheRegistry = "ghcr.io"

	// changelogCacheNamespace is the repository namespace under which each project publishes
	// its changelog artifact, one repository per project tagged by full git commit SHA
	// (e.g. ghcr.io/anchore/changelog/syft:<commit>).
	changelogCacheNamespace = "anchore/changelog"

	// changelogMarkdownSuffix identifies the rendered markdown layer within the artifact
	// (matched against the org.opencontainers.image.title annotation).
	changelogMarkdownSuffix = ".md"

	// cacheFetchAttempts and cacheFetchBackoff control how aggressively transient registry
	// failures (5xx, non-auth 4xx, network blips) are retried before giving up and falling
	// back to generating the changelog locally.
	cacheFetchAttempts = 4
	cacheFetchBackoff  = 1 * time.Second

	// maxManifestDepth bounds index->manifest resolution so a pathological or self-referential
	// index returns an error (and falls back) instead of recursing forever.
	maxManifestDepth = 4
)

// manifestAccept advertises every manifest media type we know how to parse so the registry
// returns the artifact manifest (or an index pointing at it) rather than a 406.
const manifestAccept = "application/vnd.oci.image.manifest.v1+json," +
	"application/vnd.oci.image.index.v1+json," +
	"application/vnd.docker.distribution.manifest.v2+json," +
	"application/vnd.docker.distribution.manifest.list.v2+json"

// majorVersionSuffix matches a trailing major-version path segment like "/v2".
var majorVersionSuffix = regexp.MustCompile(`/v\d+$`)

// seams allowing tests to drive GetChangelog's fallback decision without a live registry or
// chronicle; production wiring points at the real implementations.
var (
	fetchCachedChangelogFn = fetchCachedChangelog
	fallbackChangelogFn    = fallbackChangelog
	writeAndShowFn         = writeAndShow
)

// GetChangelog resolves the release changelog for the current HEAD commit, preferring the
// pre-built artifact published to the OCI changelog cache over regenerating it in the pipeline.
// It returns the changelog file path and the computed release version.
//
// On any cache failure (a missing artifact, an auth problem, or transient registry errors that
// outlast the retries) it falls back to generating the changelog locally with chronicle, exactly
// as the pipeline did before the cache existed. Pass the release version when it is already known
// (CI publish jobs) so the fallback can scope to that tag; pass "" (the release trigger) to let
// chronicle compute the next version.
func GetChangelog(version string) (changelogFilePath, computedVersion string) {
	c, err := fetchCachedChangelogFn()
	switch {
	case err != nil:
		log.Debug("changelog cache unavailable, falling back to chronicle: %v", err)
		return fallbackChangelogFn(version)
	case version == "" && c.version == "":
		// cache hit lacked a version and the caller needs one; treat it as a miss so we
		// fall back to chronicle instead of handing back an empty version.
		log.Debug("changelog cache hit lacked a version, falling back to chronicle")
		return fallbackChangelogFn(version)
	}

	writeAndShowFn(c)
	// prefer the caller-supplied tag (authoritative) over the commit-keyed cached version,
	// which may differ from the tag actually being published.
	if version != "" {
		return changelogFile, version
	}
	return changelogFile, c.version
}

// fallbackChangelog generates the changelog locally with chronicle, exactly as the pipeline did
// before the cache existed, and notes the miss beneath the rendered changelog.
func fallbackChangelog(version string) (changelogFilePath, computedVersion string) {
	defer noteCacheMiss()
	if version == "" {
		_, versionFilePath := GenerateAndShowChangelog()
		return changelogFile, strings.TrimSpace(file.Read(versionFilePath))
	}
	return GenerateAndShowFromVersion(version), version
}

// noteCacheMiss prints a subtle, greyed-out note beneath the (already-displayed) changelog so it
// is clear the cache was skipped without drowning out the changelog itself.
func noteCacheMiss() {
	_, _ = fmt.Fprintln(os.Stderr, color.Grey("(changelog cache unavailable — generated locally)"))
}

// writeAndShow persists the cached changelog (and version, when present) to disk and prints the
// cached markdown directly to stderr. Unlike the chronicle path (which pretty-renders via
// md-pretty), this surfaces the markdown exactly as it was published to the cache.
func writeAndShow(c cachedChangelog) {
	file.Write(changelogFile, c.markdown)
	if c.version != "" {
		file.Write(versionFile, c.version)
	}
	_, _ = fmt.Fprintln(os.Stderr, c.markdown)
}

// cachedChangelog is the subset of the OCI changelog artifact we consume: the rendered markdown
// and the computed release version.
type cachedChangelog struct {
	markdown string
	version  string
}

// fetchCachedChangelog pulls the changelog artifact for HEAD from the OCI cache, retrying only
// on transient errors (5xx, rate limiting, network blips). Definitive failures (a missing
// artifact, auth problems) return immediately so the caller can fall back without delay.
func fetchCachedChangelog() (cachedChangelog, error) {
	// resolve setup inputs without panicking so a missing go.mod or git failure degrades to
	// the chronicle fallback rather than aborting the release.
	name, err := projectName()
	if err != nil {
		return cachedChangelog{}, err
	}
	repo := changelogCacheNamespace + "/" + name

	commit, err := run.Command("git", run.Args("rev-parse", "HEAD"))
	if err != nil {
		return cachedChangelog{}, fmt.Errorf("resolving HEAD commit: %w", err)
	}
	commit = strings.TrimSpace(commit)

	c := &ociClient{
		scheme:   "https",
		registry: changelogCacheRegistry,
		repo:     repo,
		ghToken:  githubToken(),
		http:     &http.Client{Timeout: 30 * time.Second},
	}

	log.Info("getting changelog")
	log.Debug("fetching changelog from cache: %s/%s:%s", c.registry, repo, commit)

	ctx := context.Background()
	var lastErr error
	backoff := cacheFetchBackoff
	for attempt := 1; attempt <= cacheFetchAttempts; attempt++ {
		cl, err := c.pull(ctx, commit)
		if err == nil {
			return cl, nil
		}
		if !isRetryable(err) {
			return cachedChangelog{}, err
		}

		lastErr = err
		log.Debug("changelog cache fetch attempt %d/%d failed: %v", attempt, cacheFetchAttempts, err)
		if attempt < cacheFetchAttempts {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return cachedChangelog{}, fmt.Errorf("changelog cache unavailable after %d attempts: %w", cacheFetchAttempts, lastErr)
}

// ociClient is a minimal read-only OCI distribution client, just enough to resolve an artifact
// manifest and pull a single named blob from it.
type ociClient struct {
	scheme   string // "https" in production; overridable for tests
	registry string
	repo     string
	ghToken  string
	bearer   string
	http     *http.Client
}

// pull authenticates, resolves the artifact manifest at ref, and returns its changelog markdown
// (required) and version (best-effort: the version layer may be absent and callers that need it
// validate separately).
func (c *ociClient) pull(ctx context.Context, ref string) (cachedChangelog, error) {
	if err := c.authenticate(ctx); err != nil {
		return cachedChangelog{}, err
	}

	m, err := c.getManifest(ctx, ref, 0)
	if err != nil {
		return cachedChangelog{}, err
	}

	mdLayer, ok := findLayer(m.Layers, isMarkdownLayer)
	if !ok {
		// a resolved manifest without a markdown layer is a structural miss, not a transient
		// error, so fall back immediately rather than retrying.
		return cachedChangelog{}, &cacheMissError{reason: fmt.Sprintf("no markdown layer found in changelog artifact %s:%s", c.repo, ref)}
	}
	md, err := c.getBlob(ctx, mdLayer.Digest)
	if err != nil {
		return cachedChangelog{}, err
	}
	if len(strings.TrimSpace(md)) == 0 {
		// an empty changelog blob would silently produce empty release notes; treat it as a
		// structural miss so we fall back to chronicle.
		return cachedChangelog{}, &cacheMissError{reason: fmt.Sprintf("empty changelog markdown in artifact %s:%s", c.repo, ref)}
	}

	out := cachedChangelog{markdown: md}

	if verLayer, ok := findLayer(m.Layers, isVersionLayer); ok {
		ver, err := c.getBlob(ctx, verLayer.Digest)
		if err != nil {
			return cachedChangelog{}, err
		}
		out.version = strings.TrimSpace(ver)
	}

	return out, nil
}

func (c *ociClient) getBlob(ctx context.Context, digest string) (string, error) {
	blobURL := fmt.Sprintf("%s://%s/v2/%s/blobs/%s", c.scheme, c.registry, c.repo, digest)
	body, err := c.get(ctx, blobURL, "")
	return string(body), err
}

// authenticate exchanges the GitHub token (or anonymous access) for a pull-scoped bearer token
// from the registry's token endpoint.
func (c *ociClient) authenticate(ctx context.Context) error {
	scope := "repository:" + c.repo + ":pull"
	tokenURL := fmt.Sprintf("%s://%s/token?service=%s&scope=%s", c.scheme, c.registry, c.registry, url.QueryEscape(scope))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return err
	}
	if c.ghToken != "" {
		// any username paired with the token works for ghcr token exchange
		req.Header.Set("Authorization", "Basic "+basicAuth("x-access-token", c.ghToken))
	}

	body, err := c.do(req, "token endpoint")
	if err != nil {
		return err
	}

	var t struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return fmt.Errorf("decoding token response: %w", err)
	}

	c.bearer = t.Token
	if c.bearer == "" {
		c.bearer = t.AccessToken
	}
	if c.bearer == "" {
		return fmt.Errorf("empty token from %s", c.registry)
	}
	return nil
}

// getManifest resolves the manifest at ref, following an index to its first child manifest. depth
// bounds that recursion (see maxManifestDepth) so a self-referential index errors out.
func (c *ociClient) getManifest(ctx context.Context, ref string, depth int) (*ociManifest, error) {
	if depth >= maxManifestDepth {
		return nil, fmt.Errorf("manifest resolution exceeded depth %d for %s:%s", maxManifestDepth, c.repo, ref)
	}

	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", c.scheme, c.registry, c.repo, ref)
	body, err := c.get(ctx, manifestURL, manifestAccept)
	if err != nil {
		return nil, err
	}

	var m ociManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	// if we got an index, resolve to the first child manifest it points at
	if len(m.Manifests) > 0 {
		return c.getManifest(ctx, m.Manifests[0].Digest, depth+1)
	}
	return &m, nil
}

func (c *ociClient) get(ctx context.Context, urlString, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlString, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	return c.do(req, urlString)
}

// do performs the request and normalizes the response into ([]byte, error), surfacing non-2xx
// responses as a statusError so the caller can decide whether to retry.
func (c *ociClient) do(req *http.Request, what string) ([]byte, error) {
	log.Debug("changelog cache %s %s", req.Method, what)

	// the request host and scheme are fixed constants (https + ghcr.io); only the URL path is
	// derived from project/commit metadata, so gosec's SSRF taint warning is a false positive here.
	rsp, err := c.http.Do(req) //nolint:gosec // G704: destination host/scheme are constant, not attacker-controllable
	if err != nil {
		return nil, err
	}
	defer func() { _ = rsp.Body.Close() }()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		// a mid-read transport drop must not pass partial bytes off as a complete body; surface
		// it as a (retryable) transport error so we retry and then fall back.
		return nil, err
	}

	if rsp.StatusCode >= 200 && rsp.StatusCode < 300 {
		return body, nil
	}
	return nil, &statusError{status: rsp.StatusCode, what: what, body: truncate(body)}
}

type ociManifest struct {
	MediaType string          `json:"mediaType"`
	Manifests []ociDescriptor `json:"manifests"` // populated when the response is an index
	Layers    []ociDescriptor `json:"layers"`    // populated for an image/artifact manifest
}

type ociDescriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Annotations map[string]string `json:"annotations"`
}

// findLayer returns the first layer matching the predicate, which receives the (lowercased)
// title annotation and media type of each layer.
func findLayer(layers []ociDescriptor, match func(title, mediaType string) bool) (ociDescriptor, bool) {
	for _, l := range layers {
		title := strings.ToLower(l.Annotations["org.opencontainers.image.title"])
		if match(title, strings.ToLower(l.MediaType)) {
			return l, true
		}
	}
	return ociDescriptor{}, false
}

// isMarkdownLayer identifies the rendered changelog layer by filename suffix or media type.
func isMarkdownLayer(title, mediaType string) bool {
	return strings.HasSuffix(title, changelogMarkdownSuffix) || strings.Contains(mediaType, "markdown")
}

// isVersionLayer identifies the computed-version layer (a bare "version"/"VERSION" file).
func isVersionLayer(title, _ string) bool {
	base := title
	if i := strings.LastIndex(title, "/"); i >= 0 {
		base = title[i+1:]
	}
	return base == "version" || strings.HasSuffix(title, ".version")
}

// statusError represents a non-2xx HTTP response from the registry.
type statusError struct {
	status int
	what   string
	body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s: unexpected status %d: %s", e.what, e.status, e.body)
}

// cacheMissError marks a structural cache miss (a resolved artifact that is unusable: missing
// markdown layer or empty markdown) so the caller falls back to chronicle without burning the
// retry budget.
type cacheMissError struct {
	reason string
}

func (e *cacheMissError) Error() string { return e.reason }

// isRetryable reports whether an error is worth retrying. Transport errors and transient server
// responses (5xx, rate limiting) are; definitive client responses (a missing artifact, auth
// problems) and structural misses are not, so the caller falls back to chronicle without burning
// the retry budget.
func isRetryable(err error) bool {
	var miss *cacheMissError
	if errors.As(err, &miss) {
		return false
	}
	var s *statusError
	if errors.As(err, &s) {
		return s.status >= 500 || s.status == http.StatusTooManyRequests
	}
	return true
}

// projectName derives the changelog artifact's project name from the module path, e.g.
// github.com/anchore/syft -> syft (ignoring any trailing major-version suffix like /v2).
func projectName() (name string, err error) {
	// gomod.Read panics if go.mod can't be read or parsed; recover so a setup failure degrades
	// to the chronicle fallback instead of aborting the release.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("unable to determine project name: %v", r)
		}
	}()

	m := gomod.Read()
	if m == nil || m.Module == nil {
		return "", fmt.Errorf("unable to determine project name: no go.mod module path found")
	}
	path := majorVersionSuffix.ReplaceAllString(m.Module.Mod.Path, "")
	return path[strings.LastIndex(path, "/")+1:], nil
}

func githubToken() string {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t
	}
	return strings.TrimSpace(Run("gh auth token", run.NoFail()))
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

// truncate trims and shortens a registry error body to a bounded length, counting runes so it
// never splits a multi-byte UTF-8 sequence before the ellipsis.
func truncate(b []byte) string {
	const max = 256
	s := strings.TrimSpace(string(b))
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}
