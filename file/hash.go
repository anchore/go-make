package file

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/anchore/go-make/lang"
)

// Sha256Hash computes the SHA-256 hash of the specified file and returns it
// as a lowercase hexadecimal string. Panics if the file cannot be read.
func Sha256Hash(file string) string {
	f := lang.Return(os.Open(file))
	defer lang.Close(f, file)
	sum := sha256.New()
	_ = lang.Return(io.Copy(sum, f))
	return fmt.Sprintf("%x", sum.Sum(nil))
}
