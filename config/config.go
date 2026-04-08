package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

var (
	// ToolDir is a template string for the directory where managed tools are installed.
	// Defaults to "{{RootDir}}/.tool". Can be overridden before Makefile() is called.
	ToolDir = "{{RootDir}}/.tool"

	// RootDir is a template string for the project root directory.
	// Defaults to "{{GitRoot}}" which resolves to the directory containing .git.
	RootDir = "{{GitRoot}}"

	// TmpDir specifies an alternate temporary directory. If empty, uses the system
	// default temp directory. Useful for CI environments with specific temp paths.
	TmpDir = ""

	// OS is the target operating system (runtime.GOOS). Used in template rendering
	// and tool downloads.
	OS = runtime.GOOS

	// Arch is the target architecture (runtime.GOARCH). Used in template rendering
	// and tool downloads.
	Arch = runtime.GOARCH

	// Debug enables debug logging and additional diagnostics like periodic stack traces.
	// Set via DEBUG=true or RUNNER_DEBUG=1 environment variables.
	Debug = false

	// Trace enables even more verbose logging than Debug. Implies Debug=true.
	// Set via TRACE=true environment variable.
	Trace = false

	// CI indicates running in a continuous integration environment.
	// Set via CI=true environment variable (automatically set by most CI systems).
	CI = false

	// Windows is true when running on Windows (runtime.GOOS == "windows").
	Windows = runtime.GOOS == "windows"

	// Cleanup controls whether temporary files are deleted. Automatically disabled
	// when Debug or CI is true to aid in debugging.
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
