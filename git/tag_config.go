package git

import (
	"fmt"
	"regexp"
	"strings"
)

// CreateTagConfig contains configuration for creating a local git tag.
type CreateTagConfig struct {
	Tag          string
	TagMessage   string
	GitUserName  string
	GitUserEmail string
}

// PushTagConfig contains configuration for pushing a tag to a remote.
type PushTagConfig struct {
	Tag        string
	DeployKey  string
	Repository string
}

// validation regex patterns
var (
	// tag must start with alphanumeric and can contain alphanumeric, ., _, /, -
	tagPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,255}$`)

	// safe string pattern for git user name/email (no shell injection chars)
	safeStringPattern = regexp.MustCompile(`^[a-zA-Z0-9@._\-\[\] ]{1,256}$`)

	// repository part pattern (owner and repo name)
	repoPartPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

// validate validates all CreateTagConfig fields.
func (c CreateTagConfig) validate() {
	if err := validateTag(c.Tag); err != nil {
		panic(err)
	}
	if err := validateSafeString(c.TagMessage, "tag message"); err != nil {
		panic(err)
	}
	if err := validateSafeString(c.GitUserName, "git user name"); err != nil {
		panic(err)
	}
	if err := validateSafeString(c.GitUserEmail, "git user email"); err != nil {
		panic(err)
	}
}

// validate validates all PushTagConfig fields.
func (c PushTagConfig) validate() {
	if err := validateTag(c.Tag); err != nil {
		panic(err)
	}
	if err := validateDeployKey(c.DeployKey); err != nil {
		panic(err)
	}
	if err := validateRepository(c.Repository); err != nil {
		panic(err)
	}
}

// validateTag validates the git tag name format.
// Rejects spaces, shell metacharacters, .., .lock, leading -, and tags > 256 chars.
func validateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("invalid tag format: %q", tag)
	}
	if strings.Contains(tag, "..") {
		return fmt.Errorf("tag cannot contain '..': %q", tag)
	}
	if strings.HasSuffix(tag, ".lock") {
		return fmt.Errorf("tag cannot end with '.lock': %q", tag)
	}
	return nil
}

// validateDeployKey validates the SSH private key format.
// Requires PEM markers, rejects public keys, null bytes, and keys > 16KB.
func validateDeployKey(key string) error {
	if key == "" {
		return fmt.Errorf("deploy key cannot be empty")
	}
	if len(key) > 16*1024 {
		return fmt.Errorf("deploy key exceeds maximum size of 16KB")
	}
	if strings.Contains(key, "\x00") {
		return fmt.Errorf("deploy key contains null bytes")
	}

	// must have both BEGIN and END markers for a private key
	hasBegin := strings.Contains(key, "-----BEGIN")
	hasEnd := strings.Contains(key, "-----END")
	hasPrivateKey := strings.Contains(key, "PRIVATE KEY")

	if !hasBegin || !hasEnd || !hasPrivateKey {
		return fmt.Errorf("deploy key must be a PEM-formatted private key")
	}

	// extract the key type from BEGIN marker and verify END marker matches
	beginIdx := strings.Index(key, "-----BEGIN ")
	if beginIdx == -1 {
		return fmt.Errorf("deploy key must be a PEM-formatted private key")
	}
	afterBegin := key[beginIdx+len("-----BEGIN "):]
	endOfType := strings.Index(afterBegin, "-----")
	if endOfType == -1 {
		return fmt.Errorf("deploy key has malformed BEGIN marker")
	}
	keyType := afterBegin[:endOfType]

	// verify the END marker has the same key type
	expectedEnd := "-----END " + keyType + "-----"
	if !strings.Contains(key, expectedEnd) {
		return fmt.Errorf("deploy key has mismatched BEGIN/END markers")
	}

	// reject if it looks like a public key or certificate
	if strings.Contains(key, "PUBLIC KEY") {
		return fmt.Errorf("deploy key appears to be a public key, not a private key")
	}
	if strings.Contains(key, "CERTIFICATE") {
		return fmt.Errorf("deploy key appears to be a certificate, not a private key")
	}

	return nil
}

// validateSafeString validates user-provided strings (name, email) to prevent shell injection.
// Rejects ; $ \ ` ( ) and other shell metacharacters.
func validateSafeString(val, field string) error {
	if val == "" {
		return fmt.Errorf("%s cannot be empty", field)
	}
	if !safeStringPattern.MatchString(val) {
		return fmt.Errorf("%s contains invalid characters: %q", field, val)
	}
	return nil
}

// validateRepository validates the GitHub repository format (owner/repo).
// Requires single /, alphanumeric+._- only.
func validateRepository(repo string) error {
	if repo == "" {
		return fmt.Errorf("repository cannot be empty")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("repository must be in format 'owner/repo': %q", repo)
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("repository owner and name cannot be empty: %q", repo)
		}
		if !repoPartPattern.MatchString(part) {
			return fmt.Errorf("repository contains invalid characters: %q", repo)
		}
	}
	return nil
}
