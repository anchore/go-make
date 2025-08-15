package file

import (
	"bytes"
	"os"

	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

func DosToUnix(glob string) {
	for _, file := range FindAll(glob) {
		perms := lang.Return(os.Stat(file))
		contents := lang.Return(os.ReadFile(file))
		orig := len(contents)
		contents = bytes.ReplaceAll(contents, []byte("\r"), []byte(""))
		if len(contents) != orig {
			log.Debug("dos2unix: ", file)
			lang.Throw(os.WriteFile(file, contents, perms.Mode()))
		} else {
			log.Trace("dos2unix skipping: ", file)
		}
	}
}
