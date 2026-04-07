package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// GitHubMeta represents the response from the GitHub meta API.
// See: https://docs.github.com/en/rest/meta/meta#get-github-meta-information
type GitHubMeta struct {
	SSHKeyFingerprints map[string]string `json:"ssh_key_fingerprints"`
	SSHKeys            []string          `json:"ssh_keys"`
}

const (
	metaURL    = "https://api.github.com/meta"
	outputFile = "known_hosts"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Printf("fetching GitHub SSH keys from %s\n", metaURL)

	req, err := http.NewRequest(http.MethodGet, metaURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching meta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var meta GitHubMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if len(meta.SSHKeys) == 0 {
		return fmt.Errorf("no SSH keys found in response")
	}

	// sort keys for deterministic output
	keys := make([]string, len(meta.SSHKeys))
	copy(keys, meta.SSHKeys)
	sort.Strings(keys)

	// format as known_hosts entries (github.com <key>)
	var lines []string
	for _, key := range keys {
		// keys from API are just the key part, need to prefix with hostname
		lines = append(lines, fmt.Sprintf("github.com %s", key))
	}

	content := strings.Join(lines, "\n") + "\n"

	// find the directory containing this source file
	// when run via "go run ./internal/cmd/gen-github-ssh-keys" from the release package,
	// we want to write to internal/cmd/gen-github-ssh-keys/known_hosts
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("unable to determine source file location")
	}
	outputPath := filepath.Join(filepath.Dir(thisFile), outputFile)

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil { //nolint:gosec // G306: known_hosts contains public keys, 0644 is appropriate
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	fmt.Printf("wrote %d SSH keys to %s\n", len(keys), outputPath)

	// also print fingerprints for verification
	if len(meta.SSHKeyFingerprints) > 0 {
		fmt.Println("\nSSH key fingerprints (for verification):")
		var fps []string
		for algo, fp := range meta.SSHKeyFingerprints {
			fps = append(fps, fmt.Sprintf("  %s: %s", algo, fp))
		}
		sort.Strings(fps)
		for _, fp := range fps {
			fmt.Println(fp)
		}
		fmt.Println("\nVerify these match: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints")
	}

	return nil
}
