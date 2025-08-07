package color

import (
	"fmt"
	"os"
)

var (
	Bold      = makeColor(1)
	Underline = makeColor(4)

	Black = makeColor(30)
	Red   = makeColor(31)
	Green = makeColor(32)
	White = makeColor(37)
	Grey  = makeColor(90)

	BgRed   = makeColor(41)
	BgGreen = makeColor(42)

	Reset = "\033[0m"
)

// colorFunc automatically switch to format if args provided, directly output string otherwise
type colorFunc func(s string, args ...any) string

func (c colorFunc) And(color colorFunc) colorFunc {
	return func(s string, args ...any) string {
		return c(color(s), args...)
	}
}

func makeColor(c int) colorFunc {
	render := func(s string, args ...any) string {
		if len(args) > 0 {
			return fmt.Sprintf(s, args...)
		}
		return s
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("NOCOLOR") != "" {
		return render
	}
	prefix := fmt.Sprintf("\033[%vm", c)
	return func(s string, args ...any) string {
		return prefix + render(s, args...) + Reset
	}
}
