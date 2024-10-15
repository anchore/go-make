package make

import (
	"fmt"
	"io"
	"os"
)

var NewLine = fmt.Sprintln()

var TmpDirRoot = ""

func WithTempDir(fn func(dir string)) {
	tmp := Get(os.MkdirTemp(TmpDirRoot, "buildtools-tmp-"))
	defer func() {
		LogErr(os.RemoveAll(tmp))
	}()
	fn(tmp)
}

func InTempDir(fn func()) {
	WithTempDir(func(tmp string) {
		cwd := Cwd()
		defer Cd(cwd)
		Cd(tmp)
		fn()
	})
}

func InGitClone(repo, branch string, fn func()) {
	InTempDir(func() {
		Run("git clone --depth 1 --branch", branch, repo, "repo")
		Cd("repo")
		fn()
	})
}

var Log = func(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, Tpl(format)+NewLine, args...)
}

func LogErr(err error) {
	if err != nil {
		Log("%v", err)
	}
}

func NoErr(e error) {
	if e != nil {
		Throw(e)
	}
}

func Get[T any](t T, e error) T {
	NoErr(e)
	return t
}

func All[T any](values ...T) []T {
	return values
}

func Close(closeable io.Closer) {
	if closeable != nil {
		LogErr(closeable.Close())
	}
}
