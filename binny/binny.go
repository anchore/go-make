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

var (
	binnyManaged    = readRootBinnyYaml()
	defaultVersions = map[string]string{}
	defaultContents []byte
	installed       = map[string]string{}
)

func DefaultConfig(binnyConfig io.Reader) {
	defaultContents = lang.Return(io.ReadAll(binnyConfig))
	defaultVersions = readBinnyYamlVersions(bytes.NewReader(defaultContents))
}

func IsManagedTool(cmd string) bool {
	return binnyManaged[cmd] != "" || defaultVersions[cmd] != ""
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

	// first, check if we have the tool in the path already, such as `gh` on a GitHub Actions runner;
	// we may need to find a way to force use of the managed version
	fullPath, err := exec.LookPath(cmd)
	if fullPath != "" && err == nil {
		installed[cmd] = fullPath
		return fullPath
	}

	if !IsManagedTool(cmd) {
		return ""
	}

	fullPath = Install(cmd)
	installed[cmd] = fullPath
	return fullPath
}

// Install installs the named executable and returns an absolute path to it
func Install(cmd string) string {
	binnyPath := ToolPath(CMD)
	if installed[CMD] != binnyPath {
		if !file.Exists(binnyPath) {
			installBinny(binnyPath, findBinnyVersion())
		} else if cmd != CMD && IsManagedTool(CMD) {
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

	// to support default versions inherited from the go-make repo itself, these need
	// to have a config file on disk for binny to read to get versions, etc.
	var cfg []run.Option
	if binnyManaged[cmd] == "" && defaultVersions[cmd] != "" {
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

func readRootBinnyYaml() map[string]string {
	rootDir := template.Render(config.RootDir)
	binnyYaml := file.FindParent(rootDir, ".binny.yaml")
	if binnyYaml == "" {
		log.Debug("no .binny.yaml found in %v or any parent directory", rootDir)
		return map[string]string{}
	}
	return readBinnyYamlVersions(lang.Return(os.Open(binnyYaml)))
}

func readBinnyYamlVersions(binnyConfig io.Reader) map[string]string {
	out := map[string]string{}
	if binnyConfig != nil {
		if closer, _ := binnyConfig.(io.Closer); closer != nil {
			defer lang.Close(closer, ".binny.yaml")
		}
		cfg := map[string]any{}
		d := yaml.NewDecoder(binnyConfig)
		lang.Throw(d.Decode(&cfg))
		tools := cfg["tools"]
		if tools, ok := tools.([]any); ok {
			for _, tool := range tools {
				if m, ok := tool.(map[string]any); ok {
					version := m["version"]
					if v, ok := version.(map[string]any); ok {
						if want, ok := v["want"].(string); ok {
							version = want
						}
					}
					out[toString(m["name"])] = toString(version)
				}
			}
		}
	}
	return out
}

func findVersion(name string) string {
	return lang.Default(binnyManaged[name], defaultVersions[name])
}

func findBinnyVersion() string {
	// TODO: pin to floating tag? (e.g. v0)
	return lang.Default(findVersion("binny"), "v0.9.0")
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
	for i, part := range parts1 {
		part = remover.ReplaceAllString(part, "")
		if i <= len(parts2) {
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
