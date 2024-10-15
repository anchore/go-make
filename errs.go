package make

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/anchore/go-make/color"
)

// OkError is used to proceed normally
type OkError struct{}

// StackTraceError provides nicer stack information
type StackTraceError struct {
	Err      any
	ExitCode int
	Stack    []string
}

func (s *StackTraceError) Error() string {
	return fmt.Sprintf(color.Red("ERROR: %v")+"%s%s%v", s.Err, NewLine, NewLine, strings.Join(s.Stack, NewLine))
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
		Log("%v", v)
		if v.ExitCode > 0 {
			os.Exit(v.ExitCode)
		}
		os.Exit(1)
	default:
		panic(v)
	}
}

// Throw panics with a stack trace
func Throw(err error) {
	panic(err)
}

// Catch handles panic values and returns any error caught
func Catch(fn func()) (err error) {
	defer func() {
		if recoverVal := recover(); recoverVal != nil {
			if e, ok := recoverVal.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", recoverVal)
			}
		}
	}()
	fn()
	return nil
}

// appendStackOnPanic helps to capture nicer stack trace information
func appendStackOnPanic() {
	v := recover()
	if v == nil {
		return
	}
	var out *StackTraceError
	switch v := v.(type) {
	case OkError:
		return
	case *StackTraceError:
		out = v
	default:
		out = &StackTraceError{
			Err: v,
		}
	}
	out.Stack = append(out.Stack, stackTraceLines()...)
	panic(out)
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
	return strings.HasPrefix(line, "panic(") ||
		strings.HasPrefix(line, "runtime/") ||
		strings.HasPrefix(line, "github.com/anchore/go-make.")
}
