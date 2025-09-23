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

// Throw panics if the provided error is non-nil
func Throw(e error) {
	if e != nil {
		panic(e)
	}
}

// OkError is used to proceed normally
type OkError struct{}

func (o *OkError) Error() string {
	return "OK"
}

// StackTraceError provides nicer stack information
type StackTraceError struct {
	Err      error
	ExitCode int
	Stack    []string
	Log      string
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

// HandleErrors is a utility to make errors and error codes handled prettier
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

// Catch handles panic values and returns any error caught
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
	return strings.HasPrefix(line, "panic(") ||
		strings.HasPrefix(line, "runtime/") ||
		strings.Contains(line, "testing.") ||
		strings.HasPrefix(line, "main.main()") ||
		strings.HasPrefix(line, "github.com/anchore/go-make.") ||
		(strings.HasPrefix(line, "github.com/anchore/go-make/") &&
			!strings.HasPrefix(line, "github.com/anchore/go-make/tasks/"))
}
