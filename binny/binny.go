package binny

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/fetch"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

const CMD = "binny"

// toolSpec captures the fields go-make needs from a single tool entry in a
// .binny.yaml. Version is the requested version string (e.g. "v0.13.0"). When
// LocalModule is non-empty, this entry refers to a checked-out source tree on
// disk (method: go-install + with.module: <relative path>) — go-make will shim
// it to `go run` instead of fetching or building a release binary. LocalModule
// is an absolute path; Entrypoint mirrors binny's with.entrypoint.
type toolSpec struct {
	Version     string
	LocalModule string
	Entrypoint  string
}

var (
	// binnyManaged holds tool specs from the project's local .binny.yaml.
	// Takes precedence over defaultSpecs when both define the same tool.
	binnyManaged = readRootBinnyYaml()

	// defaultSpecs holds tool specs from go-make's embedded .binny.yaml.
	// Populated by DefaultConfig() during go-make's init(). Used as fallback
	// when a tool isn't defined in the local .binny.yaml.
	defaultSpecs = map[string]toolSpec{}

	// defaultContents stores the raw embedded .binny.yaml bytes. Needed because
	// binny requires a config file on disk to read tool installation details.
	defaultContents []byte

	installed = map[string]string{}
)

func DefaultConfig(binnyConfig io.Reader) {
	defaultContents = lang.Return(io.ReadAll(binnyConfig))
	// embedded defaults never reference a local module on disk, so the base dir
	// passed here is irrelevant — pass "" and any local-module entries (which
	// shouldn't exist here) will be discarded.
	defaultSpecs = readBinnyYamlSpecs(bytes.NewReader(defaultContents), "")
}

func IsManagedTool(cmd string) bool {
	return isLocalSpec(binnyManaged[cmd]) || binnyManaged[cmd].Version != "" || defaultSpecs[cmd].Version != ""
}

// isLocalSpec is true when the spec points at a checked-out source tree rather
// than a released version.
func isLocalSpec(s toolSpec) bool {
	return s.LocalModule != ""
}

// ManagedToolPath returns the full path to a binny managed tool, installing or updating it before returning
// or returning empty string "" for non-managed tools
func ManagedToolPath(cmd string) string {
	if strings.HasPrefix(cmd, template.Render(config.ToolDir)) {
		return cmd
	}

	if out := installed[cmd]; out != "" {
		return out
	}

	// always prefer binny managed tools first
	if IsManagedTool(cmd) {
		fullPath := Install(cmd)
		installed[cmd] = fullPath
		return fullPath
	}

	// second, see if the user already has this tool on path
	fullPath, err := exec.LookPath(cmd)
	if fullPath != "" && err == nil {
		installed[cmd] = fullPath
		return fullPath
	}

	return ""
}

// Install installs the named executable and returns an absolute path to it
func Install(cmd string) string {
	binnyPath := ToolPath(CMD)
	binnySpec := binnyManaged[CMD]
	if installed[CMD] != binnyPath {
		switch {
		case isLocalSpec(binnySpec):
			// binny itself is being built from a local checkout; write (or refresh)
			// a thin shim at binnyPath that defers to `go run`. No version check —
			// the source is authoritative.
			installBinnyShim(binnyPath, binnySpec)
		case !file.Exists(binnyPath):
			installBinny(binnyPath, findBinnyVersion())
		case cmd != CMD && IsManagedTool(CMD):
			// we manage the binny updates here, because binny is not released for all platforms,
			// and we may have to build from source
			binnyVersion := lang.Return(run.Command(binnyPath, run.Args("--version"), run.Quiet()))
			binnyVersion = strings.TrimPrefix(binnyVersion, CMD)
			if !IsManagedTool(CMD) || !matchesVersion(binnyVersion, findVersion(CMD)) {
				// if binny needs to update, use our own install procedure since we may be on an unsupported platform
				installBinny(binnyPath, findBinnyVersion())
			}
		}
		installed[CMD] = binnyPath
	}

	// tool version inheritance: when a tool is in go-make's embedded defaults but
	// not in the project's local .binny.yaml, we need to give binny a config file
	// to read (it can't read from embedded bytes). We write the embedded config
	// to a temp file and pass it via -c flag.
	//
	// Priority: local .binny.yaml > embedded defaults
	var cfg []run.Option
	if !inLocalConfig(cmd) && defaultSpecs[cmd].Version != "" {
		tmpDir, err := os.MkdirTemp(template.Render(config.TmpDir), "binny-config")
		if err == nil {
			defer func() {
				log.Error(os.RemoveAll(tmpDir))
			}()
			configFile := lang.Continue(filepath.Abs(filepath.Join(tmpDir, "default.yaml")))
			if configFile != "" {
				log.Error(os.WriteFile(configFile, defaultContents, 0o600))
				cfg = append(cfg, run.Args("-c", configFile))
			}
		}
	}

	toolPath := ToolPath(cmd)
	toolDir := filepath.Dir(toolPath)

	out := bytes.Buffer{}
	lang.Return(run.Command(binnyPath, run.Options(cfg...), run.Args("install", cmd),
		run.Env("BINNY_LOG_LEVEL", "info"),
		run.Env("BINNY_ROOT", toolDir),
		run.Quiet(),
		run.Stderr(&out),
	))

	if !strings.Contains(out.String(), "already installed") {
		// check if binny has given us an executable without .exe on windows and copy it, if so
		nonExe := filepath.Join(toolDir, cmd)
		if config.Windows && nonExe != toolPath && file.Exists(nonExe) {
			log.Error(lang.Catch(func() {
				// older verions of binny do not create .exe files on windows
				// TODO: fix binny to handle windows executables properly, see the fix-freebsd branch
				file.Copy(nonExe, toolPath)
			}))
		}
		log.Info("binny installed: %v at %v", cmd, toolPath)
		log.Debug("    └─ output: %v", out.String())
	}

	return toolPath
}

// installBinnyShim ensures binnyPath is a thin wrapper that defers to the local
// source tree described by spec. On Unix-like systems it's a tiny sh script
// that execs `go run`; on Windows it's a `go build` artifact (Windows can't
// exec a shell script via a path ending in .exe, so we accept the per-source-
// change build cost there).
//
// The shim is written idempotently: if the on-disk contents already match
// what we'd produce, we skip the write so we don't churn mtimes.
func installBinnyShim(binnyPath string, spec toolSpec) {
	pkg := spec.LocalModule
	if spec.Entrypoint != "" {
		pkg = filepath.Join(spec.LocalModule, spec.Entrypoint)
	}

	if config.Windows {
		// no shim — build into binnyPath. `go build` is cheap on a warm cache.
		// remove first to avoid EACCES if a prior read-only release binary
		// occupies the path (same scenario the Unix branch guards against).
		// On Windows, os.Remove fails on read-only files because unlink
		// requires write access to the file itself (unlike Unix, where parent
		// dir perms govern); chmod to a writable mode first. Both calls are
		// best-effort against a possibly-missing file.
		_ = os.Chmod(binnyPath, 0o600)
		if err := os.Remove(binnyPath); err != nil && !os.IsNotExist(err) {
			lang.Throw(err)
		}
		log.Info("building local binny from %v", pkg)
		lang.Return(run.Command("go", run.Args("build", "-o", binnyPath, pkg)))
		installed[CMD] = binnyPath
		return
	}

	content := fmt.Sprintf("#!/bin/sh\nexec go run %s \"$@\"\n", shellQuote(pkg))
	if existing, err := os.ReadFile(binnyPath); err == nil && string(existing) == content {
		installed[CMD] = binnyPath
		return
	}
	lang.Throw(os.MkdirAll(filepath.Dir(binnyPath), 0o755))
	// any pre-existing file at binnyPath may be a previously-downloaded release
	// binary written read-only (-r-x------), which would cause WriteFile to fail
	// with EACCES even though we own the parent dir. Remove first so we can
	// write fresh perms. os.Remove tolerates missing files via the IsNotExist
	// guard below.
	if err := os.Remove(binnyPath); err != nil && !os.IsNotExist(err) {
		lang.Throw(err)
	}
	// shim must be executable for the caller to exec it; gosec defaults the
	// threshold to 0600 which is not viable for an entry-point script.
	lang.Throw(os.WriteFile(binnyPath, []byte(content), 0o755)) //nolint:gosec
	log.Info("binny shim installed at %v -> go run %v", binnyPath, pkg)
	installed[CMD] = binnyPath
}

// shellQuote wraps a path in single quotes for /bin/sh, escaping any embedded
// single quotes. The shim is not user-input, but module paths can contain
// spaces and we want the shim to remain robust.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func installBinny(binnyPath, version string) {
	err := fetch.BinaryRelease(binnyPath, fetch.ReleaseSpec{
		URL: "https://github.com/anchore/binny/releases/download/v{{.version}}/binny_{{.version}}_{{.os}}_{{.arch}}.{{.ext}}",
		Args: map[string]string{
			"ext":     "tar.gz",
			"version": strings.TrimPrefix(version, "v"),
		},
		Platform: map[string]map[string]string{
			"windows": {
				"ext": "zip",
			},
		},
	})

	if err != nil {
		log.Error(err)

		BuildFromGoSource(
			binnyPath,
			"github.com/anchore/binny",
			"cmd/binny",
			version,
			run.LDFlags("-w",
				"-s",
				"-extldflags '-static'",
				"-X main.version="+version))
	}

	installed["binny"] = binnyPath
}

func readRootBinnyYaml() map[string]toolSpec {
	rootDir := template.Render(config.RootDir)
	binnyYaml := file.FindParent(rootDir, ".binny.yaml")
	if binnyYaml == "" {
		log.Debug("no .binny.yaml found in %v or any parent directory", rootDir)
		return map[string]toolSpec{}
	}
	return readBinnyYamlSpecs(lang.Return(os.Open(binnyYaml)), filepath.Dir(binnyYaml))
}

// readBinnyYamlSpecs decodes a binny config and returns the subset go-make
// cares about per tool. baseDir is the directory containing the .binny.yaml,
// used to resolve relative `with.module` paths into absolute paths so the
// shim continues to work when go-make is invoked from a subdirectory.
func readBinnyYamlSpecs(binnyConfig io.Reader, baseDir string) map[string]toolSpec {
	out := map[string]toolSpec{}
	if binnyConfig == nil {
		return out
	}
	if closer, _ := binnyConfig.(io.Closer); closer != nil {
		defer lang.Close(closer, ".binny.yaml")
	}
	cfg := map[string]any{}
	d := yaml.NewDecoder(binnyConfig)
	lang.Throw(d.Decode(&cfg))
	tools, _ := cfg["tools"].([]any)
	for _, tool := range tools {
		m, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		name := toString(m["name"])
		if name == "" {
			continue
		}
		out[name] = toSpec(m, baseDir)
	}
	return out
}

// toSpec extracts the version (and, if applicable, a local-module pointer)
// from a single tool entry. A local module is recognized when method is
// "go-install" and `with.module` looks like a filesystem path (starts with
// "." or is absolute) — the same shape used for self-development of binny.
func toSpec(m map[string]any, baseDir string) toolSpec {
	spec := toolSpec{Version: extractVersion(m["version"])}

	method := toString(m["method"])
	with, _ := m["with"].(map[string]any)
	module := toString(with["module"])
	if method == "go-install" && isLocalPath(module) && baseDir != "" {
		spec.LocalModule = lang.Return(filepath.Abs(filepath.Join(baseDir, module)))
		spec.Entrypoint = toString(with["entrypoint"])
	}
	return spec
}

func extractVersion(v any) string {
	if m, ok := v.(map[string]any); ok {
		return toString(m["want"])
	}
	return toString(v)
}

func isLocalPath(p string) bool {
	// a leading "/" is recognized as local on every OS even though Windows's
	// filepath.IsAbs requires a drive letter — a portable .binny.yaml uses
	// forward slashes and rooted paths should still parse as local there.
	return p == "." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") || filepath.IsAbs(p)
}

// inLocalConfig is true when the project's local .binny.yaml mentions cmd.
// Used to decide whether embedded defaults should be layered in via a temp
// config file when binny is asked to install cmd.
func inLocalConfig(cmd string) bool {
	s := binnyManaged[cmd]
	return s.Version != "" || isLocalSpec(s)
}

// findVersion returns the version for a tool. Local .binny.yaml takes precedence
// over embedded defaults (lang.Default returns first non-empty value).
func findVersion(name string) string {
	return lang.Default(binnyManaged[name].Version, defaultSpecs[name].Version)
}

func findBinnyVersion() string {
	// TODO: pin to floating tag? (e.g. v0)
	return lang.Default(findVersion("binny"), "v0.13.0")
}

// matchesVersion indicates the versionRequest is satisfied
// by the versionToCheck
func matchesVersion(versionRequest, versionToCheck string) bool {
	if versionRequest == "" || versionToCheck == "" {
		return false // empty versions are considered unknown
	}
	for _, ptr := range []*string{&versionRequest, &versionToCheck} {
		*ptr = strings.TrimSpace(*ptr)
		*ptr = strings.TrimPrefix(*ptr, "v")
	}
	remover := regexp.MustCompile(`^[-._]`)
	splitter := regexp.MustCompile(`((^|[-._+~a-zA-Z])[a-zA-Z]*\d+)`)
	parts1 := splitter.FindAllString(versionRequest, -1)
	parts2 := splitter.FindAllString(versionToCheck, -1)
	// when either side has no digit-bearing tokens (e.g. "main", "feature-x"), the
	// component-wise comparison below has nothing to align against; fall back to a
	// direct string comparison so non-numeric refs still produce sensible results.
	if len(parts1) == 0 || len(parts2) == 0 {
		return versionRequest == versionToCheck
	}
	for i, part := range parts1 {
		part = remover.ReplaceAllString(part, "")
		if i < len(parts2) {
			part2 := remover.ReplaceAllString(parts2[i], "")
			int1, err := strconv.Atoi(part)
			if err == nil {
				var int2 int
				int2, err = strconv.Atoi(part2)
				if err == nil {
					if int1 != int2 {
						return false
					}
					continue // equal
				}
			}
			// fall back to a string comparison
			if part != part2 {
				return false
			}
		}
	}
	return true
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

func BuildFromGoSource(file string, module, entrypoint, version string, opts ...run.Option) {
	if version == "" {
		panic(fmt.Errorf("no version specified for: %s %s %s", file, module, entrypoint))
	}
	log.Info("Building: %s@%s entrypoint: %s", module, version, entrypoint)
	git.InClone("https://"+module, version, func() {
		// go build <options> -o file <entrypoint>
		lang.Return(run.Command("go", run.Args("build"), run.Stderr(io.Discard), run.Options(opts...), run.Args("-o", file, "./"+entrypoint)))
	})
}
