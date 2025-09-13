package color

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"

	"github.com/anchore/go-make/config"
)

func init() {
	var originalMode uint32
	fd := windows.Handle(os.Stderr.Fd()) // also do this for os.Stdout?
	err := windows.GetConsoleMode(fd, &originalMode)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to get windows console mode: %v\n", err)
		// we may be running in bash or some other shell that doesn't SetConsoleMode
		return
	}
	if originalMode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		// already set
		return
	}
	err = windows.SetConsoleMode(fd, originalMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to set console mode: %v\n", err)
	} else {
		config.OnExit(func() {
			if originalMode != 0 {
				_ = windows.SetConsoleMode(windows.Handle(os.Stderr.Fd()), originalMode)
			}
		})
	}
}
