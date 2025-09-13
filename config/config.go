package config

import (
	"os"
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
	// Windows indicates running on Windows
	Windows = runtime.GOOS == "windows"
	// Cleanup whether to remove temporary files and downloads
	Cleanup = true
)

func init() {
	Trace, _ = strconv.ParseBool(Env("TRACE", "false"))
	Debug, _ = strconv.ParseBool(Env("DEBUG", strconv.FormatBool(runnerDebug() || Trace)))
	CI, _ = strconv.ParseBool(Env("CI", "false"))
	Cleanup = !Debug && !CI
}

func runnerDebug() bool {
	debug := os.Getenv("RUNNER_DEBUG")
	return debug == "1" || strings.EqualFold(debug, "true")
}
