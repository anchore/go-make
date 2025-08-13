package script

import (
	"fmt"
	"strings"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// Confirm prompts the user for a single keypress of y for yes or cancels
func Confirm(format string, args ...any) {
loop:
	for {
		log.Log(format+" [y/n]", args...)
		var response string
		lang.Return(fmt.Scan(&response))
		switch strings.ToLower(response) {
		case "y":
			break loop
		case "n":
			panic(fmt.Errorf("CANCELLED: "+format, args))
		default:
			log.Log(color.Red("Please answer 'y' or 'n'"))
		}
	}
}
