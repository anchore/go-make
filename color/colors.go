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

type colorFunc func(string) string

func (c colorFunc) And(color colorFunc) colorFunc {
	return func(s string) string {
		return c(color(s))
	}
}

func makeColor(c int) colorFunc {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("NOCOLOR") != "" {
		return func(s string) string { return s }
	}
	prefix := fmt.Sprintf("\033[%vm", c)
	return func(s string) string {
		return prefix + s + Reset
	}
}
