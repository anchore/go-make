package lang

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/log"
)

// Throw panics if the provided error is non-nil. This is the fundamental error
// handling primitive in go-make - errors cause immediate task failure with a
// stack trace pointing to the source.
func Throw(e error) {
	if e != nil {
		panic(e)
	}
}

// OkError is a sentinel error that indicates successful completion. When caught
// by HandleErrors, it exits cleanly without printing an error message.
type OkError struct{}

func (o *OkError) Error() string {
	return "OK"
}

// StackTraceError wraps an error with additional context for better error reporting.
// It captures the stack trace at creation time and can include additional log output.
type StackTraceError struct {
	// Err is the underlying error.
	Err error
	// ExitCode is the exit code to use when this error causes program termination.
	ExitCode int
	// Stack contains the filtered stack trace lines.
	Stack []string
	// Log contains additional output (e.g., stdout/stderr) to display with the error.
	Log string
}

func (s *StackTraceError) Unwrap() error {
	return s.Err
}

func (s *StackTraceError) Error() string {
	return fmt.Sprintf("%v\n%v", s.Err, strings.Join(s.Stack, "\n"))
}

func (s *StackTraceError) WithExitCode(exitCode int) *StackTraceError {
	s.ExitCode = exitCode
	return s
}

func (s *StackTraceError) WithLog(log string) *StackTraceError {
	s.Log = log
	return s
}

var _ error = (*StackTraceError)(nil)

// HandleErrors is the main panic recovery handler for go-make. It should be deferred
// at the start of task execution to catch panics and display formatted error messages.
// This is automatically called by Makefile() - you typically don't need to call it directly.
//
// Behavior:
//   - OkError: exits cleanly without error output
//   - StackTraceError: prints formatted error with stack trace, exits with ExitCode
//   - Other panics: prints error with stack trace, exits with code 1
func HandleErrors() {
	v := recover()
	if v == nil {
		return
	}
	switch v := v.(type) {
	case OkError:
		return
	case *StackTraceError:
		errText := strings.TrimSpace(fmt.Sprintf("ERROR: %v", v.Err))
		log.Info("\n" + formatError(errText) + "\n\n" + strings.TrimSpace(v.Log) + "\n\n" + color.Grey("\n\n"+strings.Join(v.Stack, "\n")))
		if v.ExitCode > 0 {
			os.Exit(v.ExitCode)
		}
	default:
		log.Info(formatError("ERROR: %v", v) + color.Grey("\n"+strings.Join(stackTraceLines(), "\n")))
	}
	os.Exit(1)
}

func formatError(format string, args ...any) string {
	line := "\n"
	if config.Windows {
		line = "\r\n"
	}
	format = line + line + " " + format + " " + line
	return color.BgRed(color.White(format+" ", args...))
}

// Catch executes fn and recovers from any panic, returning the panic value as an error.
// Use this to attempt operations that might fail without stopping the entire build.
//
// Example:
//
//	err := lang.Catch(func() {
//	    Run(`optional-command`)
//	})
//	if err != nil {
//	    log.Debug("optional command failed: %v", err)
//	}
func Catch(fn func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", v)
			}
		}
	}()
	fn()
	return nil
}

// NewStackTraceError helps to capture nicer stack trace information
func NewStackTraceError(err error) *StackTraceError {
	return &StackTraceError{
		Err:   err,
		Stack: stackTraceLines(),
	}
}

func AppendStackTraceToPanics() {
	if err := recover(); err != nil {
		var out *StackTraceError
		switch e := err.(type) {
		case *StackTraceError:
			out = e
		case error:
			out = &StackTraceError{
				Err: e,
			}
		default:
			out = &StackTraceError{
				Err: fmt.Errorf("%v", err),
			}
		}
		out.Stack = append(out.Stack, stackTraceLines()...)
		panic(out)
	}
}

func stackTraceLines() []string {
	var out []string
	stack := string(debug.Stack())
	lines := strings.Split(stack, "\n")
	// start at 1, skip goroutine line
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if skipTraceLine(line) {
			i++
			continue
		}
		out = append(out, line)
	}
	return out
}

func skipTraceLine(line string) bool {
	if config.Debug {
		return false
	}
	line = strings.TrimSpace(line)
	// keep go-make internal frames (binny, run, template, etc.) so that runtime
	// panics inside go-make show their actual origin instead of just the user-task
	// call site. only filter pure noise: the runtime panic machinery, the test
	// harness, and the main entry.
	return strings.HasPrefix(line, "panic(") ||
		strings.HasPrefix(line, "runtime/") ||
		strings.Contains(line, "testing.") ||
		strings.HasPrefix(line, "main.main()")
}
