package file

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

func Ls(dir string) string {
	dir = lang.Return(filepath.Abs(dir))
	entries := lang.Return(os.ReadDir(dir))

	buf := bytes.Buffer{}
	for _, f := range entries {
		s := lang.Return(os.Stat(filepath.Join(dir, f.Name())))
		_, _ = fmt.Fprintf(&buf, "%v %8v %v\n", s.Mode(), HumanizeBytes(s.Size()), f.Name())
	}
	return buf.String()
}

func LogWorkdir() {
	if config.DebugEnabled {
		cwd := Cwd()
		log.Log("CWD: %s", cwd)
		for _, line := range strings.Split(Ls(cwd), "\n") {
			log.Log(line)
		}
	}
}

func HumanizeBytes[T int | int64](size T) any {
	units := ""
	value := size
	switch {
	case value < 1024:
	case value < 1024*1024:
		units = "KB"
		value /= 1024
	case value < 1024*1024*1024:
		units = "MB"
		value /= 1024 * 1024
	case value < 1024*1024*1024*1024:
		units = "GB"
		value /= 1024 * 1024 * 1024
	case value < 1024*1024*1024*1024*1024:
		units = "TB"
	}
	return fmt.Sprintf("%v%s", value, units)
}
