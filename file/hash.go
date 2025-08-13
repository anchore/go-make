package file

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/anchore/go-make/lang"
)

func Sha256Hash(file string) string {
	f := lang.Return(os.Open(file))
	sum := sha256.New()
	_ = lang.Return(io.Copy(sum, f))
	return fmt.Sprintf("%x", sum.Sum(nil))
}
