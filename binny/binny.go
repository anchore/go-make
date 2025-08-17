package binny

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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
	binnyManaged = readBinnyYamlVersions()
	installed    = map[string]string{}
)

func IsManagedTool(cmd string) bool {
	return binnyManaged[cmd] != ""
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

	if !IsManagedTool(cmd) {
		return ""
	}

	fullPath := Install(cmd)
	installed[cmd] = fullPath
	return fullPath
}

// Install installs the named executable and returns an absolute path to it
func Install(cmd string) string {
	binnyPath := ToolPath(CMD)
	if installed[CMD] != binnyPath {
		if !file.Exists(binnyPath) {
			installBinny(binnyPath)
		} else if cmd != CMD && IsManagedTool(CMD) {
			// we manage the binny updates here, because binny is not released for all platforms,
			// and we may have to build from source
			binnyVersion := run.Command(binnyPath, run.Args("--version"), run.Quiet())
			binnyVersion = strings.TrimPrefix(binnyVersion, CMD)
			if !IsManagedTool(CMD) || !isVersion(binnyVersion, binnyManaged[CMD]) {
				// if binny needs to update, use our own install procedure since we may be on an unsupported platform
				installBinny(binnyPath)
			}
		}
		installed[CMD] = binnyPath
	}

	toolPath := ToolPath(cmd)
	toolDir := filepath.Dir(toolPath)

	out := bytes.Buffer{}
	run.Command(binnyPath, run.Args("install", cmd),
		run.Env("BINNY_LOG_LEVEL", "info"),
		run.Env("BINNY_ROOT", toolDir),
		run.Quiet(),
		run.Stderr(&out),
	)

	if !strings.Contains(out.String(), "already installed") {
		// check if binny has given us an executable without .exe on windows and copy it, if so
		nonExe := filepath.Join(toolDir, cmd)
		if runtime.GOOS == "windows" && nonExe != toolPath && file.Exists(nonExe) {
			log.Error(lang.Catch(func() {
				// older verions of binny do not create .exe files on windows
				// TODO: fix binny to handle windows executables properly, see the fix-freebsd branch
				file.Copy(nonExe, toolPath)
			}))
		}
		log.Log("Binny installed: %v at %v", cmd, toolPath)
		log.Debug("Binny output: %v", out.String())
	}

	return toolPath
}

func installBinny(binnyPath string) {
	version := findBinnyVersion()

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

func readBinnyYamlVersions() map[string]string {
	out := map[string]string{}
	binnyConfig := file.FindParent(template.Render(config.RootDir), ".binny.yaml")
	if binnyConfig != "" {
		cfg := map[string]any{}
		f := lang.Return(os.Open(binnyConfig))
		defer lang.Close(f, binnyConfig)
		d := yaml.NewDecoder(f)
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

func findBinnyVersion() string {
	ver := readBinnyYamlVersions()["binny"]
	if ver != "" {
		return ver
	}
	// TODO: pin to floating tag? (e.g. v0)
	return "v0.9.0"
}

// isVersion indicates the versionRequest is satisfied
// by the versionToCheck
func isVersion(versionRequest, versionToCheck string) bool {
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
	log.Log("Building: %s@%s entrypoint: %s", module, version, entrypoint)
	git.InClone("https://"+module, version, func() {
		// go build <options> -o file <entrypoint>
		opts = append([]run.Option{run.Args("build"), run.Stderr(io.Discard)}, opts...)
		opts = append(opts, run.Args("-o", file, "./"+entrypoint))
		run.Command("go", opts...)
	})
}
