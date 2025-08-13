package log

import (
	"fmt"
	"os"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/template"
)

var Prefix = ""

var Log = func(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, Prefix+template.Render(format)+"\n", args...)
}

var Debug = func(format string, args ...any) {}

var Trace = func(format string, args ...any) {}

// Error logs any non-nil error passed
var Error = func(err error, args ...any) {
	if err != nil {
		argString := ""
		for _, arg := range args {
			argString += fmt.Sprintf(" %v", arg)
		}
		Log("%v%s", err, argString)
	}
}

func init() {
	if config.DebugEnabled || config.TraceEnabled {
		Debug = debugLog
	}
	if config.TraceEnabled {
		Trace = traceLog
	}
}

func debugLog(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, Prefix+color.Grey(template.Render(format))+"\n", args...)
}

func traceLog(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, Prefix+color.Grey(template.Render(format))+"\n", args...)
}
