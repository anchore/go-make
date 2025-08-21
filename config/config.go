package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	// ToolDir is the template to find the root tool directory
	ToolDir = "{{RootDir}}/.tool"
	// RootDir is the template to find the root directory when executing
	RootDir = "{{GitRoot}}"
	// TmpDir is the template to find the an alternate TempDir, if empty defaults to system temp dir
	TmpDir = ""

	// OS is the OS name to request for commands that require OS name
	OS = runtime.GOOS
	// Arch is the architecture to request for commands that require architecture
	Arch = runtime.GOARCH

	// Debug whether to output debug logging and perform additional diagnostic work
	Debug = false
	// Trace enables Debug and enables even more verbose logging
	Trace = false

	// CI indicates running in a CI environment
	CI = false
	// Cleanup whether to remove temporary files and downloads
	Cleanup = true
)

func init() {
	const maxLen = 10
	trunc := func(s string) string {
		if len(s) > 10 {
			return s[:10] + "..."
		}
		return s
	}
	stringify := func(m map[string]string) string {
		b, e := json.MarshalIndent(m, "", "  ")
		if b != nil {
			return string(b)
		}
		return "err: " + e.Error()
	}
	env := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		env[parts[0]] = trunc(parts[1])
	}
	_, _ = fmt.Fprintf(os.Stderr, "initializing config with env: %v\n", stringify(env))

	fullEnv := map[string]string{}
	filePath := filepath.Join(Env("GITHUB_WORKSPACE", ""), "full_env.json")
	f, err := os.Open(filePath)
	if err != nil {
		err = json.NewDecoder(f).Decode(&fullEnv)
	} else {
		defer f.Close()
	}
	for k, v := range fullEnv {
		fullEnv[k] = trunc(v)
	}
	if err != nil {
		_, _ = os.Stderr.WriteString("ERROR: " + err.Error() + "\n")
	}
	_, _ = fmt.Fprintf(os.Stderr, "initializing config from %v with full_env.json: %v\n", filePath, stringify(fullEnv))
	Trace, _ = strconv.ParseBool(Env("TRACE", "false"))
	Debug, _ = strconv.ParseBool(Env("DEBUG",
		Env("ACTIONS_RUNNER_DEBUG", strconv.FormatBool(Trace))))
	CI, _ = strconv.ParseBool(Env("CI", "false"))
	Cleanup = !Debug && !CI
}
