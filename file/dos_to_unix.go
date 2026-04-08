package file

import (
	"bytes"
	"os"

	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// DosToUnix converts all files matching the glob pattern from DOS/Windows line
// endings (CRLF) to Unix line endings (LF). Files that don't contain CRLF are
// skipped. Preserves file permissions.
func DosToUnix(glob string) {
	for _, file := range FindAll(glob) {
		perms := lang.Return(os.Stat(file))
		contents := lang.Return(os.ReadFile(file))
		orig := len(contents)
		contents = bytes.ReplaceAll(contents, []byte("\r"), []byte(""))
		if len(contents) != orig {
			log.Debug("dos2unix: ", file)
			lang.Throw(os.WriteFile(file, contents, perms.Mode())) //nolint:gosec // G703: path comes from glob expansion, not user input
		} else {
			log.Trace("dos2unix skipping: ", file)
		}
	}
}
