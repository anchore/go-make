// Package redact masks credential-bearing values so they don't reach logs,
// error messages, or anywhere else go-make prints. It centralizes the rules so
// the name denylist and the secret-shape patterns live in exactly one place
// rather than drifting across the run, fetch, and github packages.
package redact

import (
	"regexp"
	"strings"
)

// Mask is the placeholder substituted for a redacted value.
const Mask = "***"

// sensitiveNameSubstrings are uppercased substrings whose presence in a NAME
// (env var, CLI flag, HTTP header, or config key) marks the associated value as
// a credential to mask. The list is deliberately conservative: a false positive
// only costs log readability, while a false negative leaks a secret to anyone
// with debug/trace logging enabled.
var sensitiveNameSubstrings = []string{
	"TOKEN",
	"SECRET",
	"PASSWORD",
	"PASSWD",
	"PASSPHRASE",
	"CREDENTIAL",
	"_KEY", // DEPLOY_KEY, PRIVATE_KEY, API_KEY, AWS_SECRET_ACCESS_KEY, ...
	"KEY_",
	"AUTH",   // Authorization, Proxy-Authorization, *_AUTH_*, ...
	"COOKIE", // Cookie / Set-Cookie carry session credentials
}

// IsSensitiveName reports whether name looks like it holds a credential.
func IsSensitiveName(name string) bool {
	upper := strings.ToUpper(name)
	for _, s := range sensitiveNameSubstrings {
		if strings.Contains(upper, s) {
			return true
		}
	}
	return false
}

// Value returns Mask when name looks sensitive, otherwise value unchanged. Use
// it wherever a value is keyed by a name (env entry, header, config key).
func Value(name, value string) string {
	if IsSensitiveName(name) {
		return Mask
	}
	return value
}

// Args returns a copy of args with credential-looking values masked, suitable
// for logging a command line. It handles two shapes:
//
//   - "name=value" / "--flag=value": the value is masked when the key looks
//     sensitive AND the value is non-empty (an empty value is not a secret, so
//     e.g. "credential.helper=" is left readable).
//   - "--flag" followed by a separate value token: the following token is masked
//     when the flag looks sensitive (e.g. "--token", "secret").
//
// A bare positional secret cannot be detected by name and passes through
// unchanged — callers must avoid putting raw secrets in positional arguments.
func Args(args []string) []string {
	out := make([]string, len(args))
	maskNext := false
	for i, a := range args {
		switch {
		case maskNext:
			out[i] = Mask
			maskNext = false
		case strings.Contains(a, "="):
			key, value, _ := strings.Cut(a, "=")
			if value != "" && IsSensitiveName(key) {
				out[i] = key + "=" + Mask
			} else {
				out[i] = a
			}
		case strings.HasPrefix(a, "-") && IsSensitiveName(a):
			out[i] = a
			maskNext = true
		default:
			out[i] = a
		}
	}
	return out
}

// githubToken matches the GitHub token families (PAT, OAuth, user-to-server,
// server-to-server, refresh, and fine-grained PAT) by their fixed prefixes.
var githubToken = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}`)

// authzHeader masks the credential in an "Authorization: <scheme> <secret>"
// header line while preserving the scheme (basic/bearer/token) for diagnostics.
var authzHeader = regexp.MustCompile(`(?i)((?:proxy-)?authorization:\s*\S+\s+)\S+`)

// urlUserinfo masks the user:password embedded in a URL (e.g.
// https://user:token@host/...).
var urlUserinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)[^/?#\s@:]+:[^/?#\s@]+@`)

// Secrets scrubs known credential shapes out of an arbitrary string. Use it for
// content that may contain secrets but isn't keyed by name — command output
// captured on failure, fetched URLs, and the like. It is best-effort by design:
// it masks the shapes go-make actually handles (GitHub tokens, Authorization
// headers, URL userinfo) and cannot catch an arbitrary opaque secret.
func Secrets(s string) string {
	s = authzHeader.ReplaceAllString(s, `${1}`+Mask)
	s = urlUserinfo.ReplaceAllString(s, `${1}`+Mask+`:`+Mask+`@`)
	s = githubToken.ReplaceAllString(s, Mask)
	return s
}
