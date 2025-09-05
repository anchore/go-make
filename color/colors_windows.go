package color

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func init() {
	fd := windows.Handle(os.Stderr.Fd()) // os.Stdout?
	var originalMode uint32
	err := windows.GetConsoleMode(fd, &originalMode)
	if err == nil {
		err = windows.SetConsoleMode(fd, originalMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to set console mode: %v\n", err)
	}
}
