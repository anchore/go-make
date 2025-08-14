package binny

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/fetch"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

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
	binnyPath := ToolPath("binny")
	if !file.Exists(binnyPath) {
		installBinny(binnyPath)
		if IsManagedTool("binny") {
			Install("binny") // this will cause binny to update .tool/.binny.state.json with itself as installed
		}
	} else if cmd != "binny" && IsManagedTool("binny") {
		// if binny itself is managed, install it to update itself
		// using ManagedToolPath will only execute binny managing its own install once, in case it needs to update
		ManagedToolPath("binny")
	}

	toolDir := lang.Return(filepath.Abs(template.Render(config.ToolDir)))

	out := bytes.Buffer{}
	run.Command(binnyPath, run.Args("install", cmd),
		run.Env("BINNY_LOG_LEVEL", "info"),
		run.Env("BINNY_ROOT", toolDir),
		run.Stdout(&out),
		run.Stderr(&out),
		run.Quiet(),
	)

	if !strings.Contains(out.String(), "already installed") {
		log.Log("Binny installed: %v", cmd)
	}

	return filepath.Join(toolDir, cmd)
}

func installBinny(binnyPath string) {
	installed["binny"] = binnyPath

	version := findBinnyVersion()

	err := downloadPrebuiltBinary(binnyPath, downloadSpec{
		url: "https://github.com/anchore/binny/releases/download/v{{.version}}/binny_{{.version}}_{{.os}}_{{.arch}}.{{.ext}}",
		args: map[string]string{
			"ext":     "tar.gz",
			"version": version,
		},
		platform: map[string]map[string]string{
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
			LDFlags("-w",
				"-s",
				"-extldflags '-static'",
				"-X main.version="+gomod.GoDepVersion("github.com/anchore/binny")))
	}
}

//nolint:gocognit
func readBinnyYamlVersions() map[string]string {
	out := map[string]string{}
	binnyConfig := file.FindParent(git.Root(), ".binny.yaml")
	if binnyConfig != "" {
		cfg := map[string]any{}
		f, err := os.Open(binnyConfig)
		defer lang.Close(f, binnyConfig)
		if err == nil {
			d := yaml.NewDecoder(f)
			err = d.Decode(&cfg)
			if err == nil {
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
							out[toString(m["name"])] = regexp.MustCompile("^v").ReplaceAllString(toString(version), "")
						}
					}
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

func toString(v any) string {
	s, _ := v.(string)
	return s
}

func downloadPrebuiltBinary(toolPath string, spec downloadSpec) error {
	tplArgs := spec.currentArgs()
	url := template.Render(spec.url, tplArgs)

	log.Log("Downloading: %v", url)

	buf := bytes.Buffer{}
	_, code, status := fetch.Fetch(url, fetch.Writer(&buf))
	contents := buf.Bytes()
	if code > 300 || len(contents) == 0 {
		return fmt.Errorf("error downloading %v: http %v %v", url, code, status)
	}
	contents = getArchiveFileContents(contents, filepath.Base(toolPath))
	if contents == nil {
		return fmt.Errorf("unable to read archive from downloading %v: http %v %v", url, code, status)
	}
	dir := filepath.Dir(toolPath)
	if !file.Exists(dir) {
		lang.Throw(os.MkdirAll(dir, 0o700|os.ModeDir))
	}
	return os.WriteFile(toolPath, contents, 0o500) //nolint:gosec // needs read + execute permissions
}

func getArchiveFileContents(archive []byte, file string) []byte {
	var errs []error

	contents, err := getZipArchiveFileContents(archive, file)
	if err == nil && len(contents) > 0 {
		return contents
	}
	errs = append(errs, err)

	contents, err = getTarGzArchiveFileContents(archive, file)
	if err == nil && len(contents) > 0 {
		return contents
	}
	errs = append(errs, err)

	panic(fmt.Errorf("unable to read archive after attempting readers: %w", errors.Join(errs...)))
}

func getZipArchiveFileContents(archive []byte, file string) ([]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	f, err := zipReader.Open(file)
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func getTarGzArchiveFileContents(archive []byte, file string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err == nil && gzipReader != nil {
		t := tar.NewReader(gzipReader)
		for {
			hdr, err := t.Next()
			if err != nil {
				return nil, err
			}
			if hdr.Name == file {
				const GB = 1024 * 1024 * 1024
				if hdr.Size > 2*GB {
					return nil, fmt.Errorf("refusing to extract file %v larger than 2 GB, declared size: %v", file, hdr.Size)
				}
				return io.ReadAll(t)
			}
		}
	}
	return nil, fmt.Errorf("file not found: %v", file)
}

func LDFlags(flags ...string) run.Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		for i, arg := range cmd.Args {
			// append to existing ldflags arg
			if arg == "-ldflags" {
				if i+1 >= len(cmd.Args) {
					cmd.Args = append(cmd.Args, "")
				} else {
					cmd.Args[i+1] += " "
				}
				cmd.Args[i+1] += strings.Join(flags, " ")
				return nil
			}
		}
		cmd.Args = append(cmd.Args, "-ldflags", strings.Join(flags, " "))
		return nil
	}
}

func BuildFromGoSource(file string, module, entrypoint, version string, opts ...run.Option) {
	log.Log("Building: %s", module)
	git.InClone("https://"+module, version, func() {
		run.Command("go build", append(opts, run.Args("-o", file, "./"+entrypoint))...)
	})
}

type downloadSpec struct {
	url      string
	args     map[string]string
	platform map[string]map[string]string
}

func (d downloadSpec) currentArgs() map[string]any {
	out := map[string]any{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	}
	for k, v := range d.args {
		out[k] = v
	}
	if d.platform != nil {
		for k, v := range d.platform[runtime.GOOS] {
			out[k] = v
		}
	}
	return out
}
