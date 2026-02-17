package log

import (
	"fmt"
	"os"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/template"
)

var Prefix = ""

var Info = func(format string, args ...any) {
	if len(args) == 0 {
		_, _ = os.Stderr.WriteString(Prefix + template.Render(format) + "\n")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, Prefix+template.Render(format)+"\n", args...)
	}
}

var Debug = func(format string, args ...any) {}

var Trace = func(format string, args ...any) {}

var Warn = func(format string, args ...any) {
	Info(color.Yellow(format), args...)
}

// Error logs any non-nil error passed
var Error = func(err error, args ...any) {
	if err != nil {
		argString := ""
		for _, arg := range args {
			argString += fmt.Sprintf(" %v", arg)
		}
		Info("%v%s", err, argString)
	}
}

func init() {
	if config.Debug || config.Trace {
		Debug = debugLog
	}
	if config.Trace {
		Trace = traceLog
	}
}

func debugLog(format string, args ...any) {
	if len(args) == 0 {
		_, _ = fmt.Fprint(os.Stderr, Prefix+color.Grey(template.Render(format))+"\n")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, Prefix+color.Grey(template.Render(format))+"\n", args...)
	}
}

func traceLog(format string, args ...any) {
	if len(args) == 0 {
		_, _ = fmt.Fprint(os.Stderr, Prefix+color.Grey(template.Render(format))+"\n")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, Prefix+color.Grey(template.Render(format))+"\n", args...)
	}
}
