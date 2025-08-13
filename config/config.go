package config

import (
	"context"
	"runtime"
	"strconv"
)

var (
	DebugEnabled = false
	TraceEnabled = false
)

func init() {
	DebugEnabled, _ = strconv.ParseBool(Env("DEBUG",
		Env("ACTIONS_RUNNER_DEBUG", "false")))
	TraceEnabled, _ = strconv.ParseBool(Env("TRACE", "false"))
}

var (
	Context, Cancel = context.WithCancel(context.Background())
)

var (
	ToolDir  = "{{RootDir}}/.tool"
	RootDir  = "{{GitRoot}}"
	TmpDir   = ""
	Platform = "{{OS}}/{{Arch}}"
	OS       = runtime.GOOS
	Arch     = runtime.GOARCH
)
