package run

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/template"
)

const opRefPrefix = "op://"

var (
	// pointer so tests can swap in a fresh Once without copying a used one
	dotEnvOnce  = &sync.Once{}
	dotEnvCache map[string]string

	// opInject is overridable in tests. Receives the raw .env content and
	// returns the rendered content with op:// references resolved.
	opInject = runOpInject

	// gitCheckIgnore is overridable in tests. Returns true when path is
	// gitignored; on indeterminate results (git missing, not a repo, etc.)
	// returns true to avoid blocking legitimate non-git workflows.
	gitCheckIgnore = runGitCheckIgnore
)

// loadDotEnv resolves and parses <RootDir>/.env exactly once per process.
// Missing files are silent; op-resolution failures fall back to raw values
// with a warning. See readDotEnv for details.
func loadDotEnv() map[string]string {
	dotEnvOnce.Do(func() {
		dotEnvCache = readDotEnv(dotEnvPath())
	})
	return dotEnvCache
}

// dotEnvPath returns <RootDir>/.env, tolerating a template render failure
// (e.g., GitRoot not registered when the git package isn't imported).
func dotEnvPath() (path string) {
	defer func() {
		// tolerate template render panics by treating as no project root
		_ = recover()
	}()
	return filepath.Join(template.Render(config.RootDir), ".env")
}

// readDotEnv reads and parses the .env file at the given path. A missing
// path returns an empty map (no error). If the file contains op:// refs,
// op inject is invoked over its contents; on failure the raw content is
// parsed instead and a warning is logged.
func readDotEnv(path string) map[string]string {
	if path == "" {
		return map[string]string{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn("dotenv: cannot read %s: %v", path, err)
		}
		return map[string]string{}
	}

	// safety gate: refuse to load if the file is not gitignored. catches the
	// common foot-gun of committing secrets. indeterminate results (no git,
	// not a repo) are treated as allowed so non-git workflows still function.
	if !gitCheckIgnore(path) {
		log.Warn("dotenv: %s is not gitignored; refusing to load (add it to .gitignore to enable)", path)
		return map[string]string{}
	}

	content := string(raw)
	if strings.Contains(content, opRefPrefix) {
		rendered, opErr := opInject(content)
		if opErr != nil {
			log.Warn("dotenv: op inject failed: %v; using raw values", opErr)
		} else {
			content = rendered
		}
	}
	return parseDotEnv(content)
}

// runGitCheckIgnore asks git whether path is gitignored. Returns true if
// ignored. On any error other than git's "not ignored" exit code (1) —
// e.g. git missing, not a repo — returns true to remain permissive.
func runGitCheckIgnore(path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", "--", path)
	cmd.Dir = filepath.Dir(path)
	cmd.Stderr = nil // suppress "fatal: not a git repository" noise
	err := cmd.Run()
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false
	}
	// could not determine (git not installed, not a repo, etc.) — be permissive
	return true
}

// runOpInject pipes content through `op inject` and returns the rendered
// output. Uses stdlib os/exec directly (not run.Command) to avoid recursing
// back into dotenv loading.
func runOpInject(content string) (string, error) {
	cmd := exec.Command("op", "inject")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stderr = os.Stderr // surface op prompts and errors directly
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// parseDotEnv parses a .env-style document into a map. Supports:
//   - blank lines and full-line comments starting with #
//   - optional `export ` prefix
//   - optional surrounding single or double quotes on values
//
// Does not perform ${VAR} interpolation.
func parseDotEnv(content string) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		out[key] = val
	}
	return out
}

// mergeDotEnv appends entries from dotEnv to env only for keys not already
// present (process env wins). Returns the merged slice and any keys that
// were skipped due to existing values, for trace logging.
func mergeDotEnv(env []string, dotEnv map[string]string) (merged []string, skipped []string) {
	existing := map[string]struct{}{}
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i > 0 {
			existing[e[:i]] = struct{}{}
		}
	}
	merged = env
	for k, v := range dotEnv {
		if _, ok := existing[k]; ok {
			skipped = append(skipped, k)
			continue
		}
		merged = append(merged, fmt.Sprintf("%s=%s", k, v))
	}
	return merged, skipped
}
