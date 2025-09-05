package github

import (
	"fmt"
	"os"
	"strings"

	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

func SetEnv(key, value string) {
	log.Trace("set github env: %s=%s", key, value)

	f := lang.Return(os.OpenFile(os.Getenv("GITHUB_ENV"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644))
	defer lang.Close(f, os.Getenv("GITHUB_ENV"))
	if strings.Contains(value, "\n") {
		delim := "EOF"
		for strings.Contains(value, delim) {
			delim += "F"
		}
		lang.Return(fmt.Fprintf(f, key+"<<"+delim+"\n"+value+"\n"+delim+"\n"))
	} else {
		lang.Return(fmt.Fprintf(f, key+"="+value+"\n"))
	}
}
