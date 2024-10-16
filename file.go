package make

import (
	"crypto/md5" //nolint: gosec
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var RootDir = func() string {
	return Tpl("{{RepoRoot}}")
}

func Cd[path string](dir path) {
	NoErr(os.Chdir(string(dir)))
}

func Cwd() string {
	return Get(os.Getwd())
}

func PushPopd[path string](dir path, run func()) {
	cwd := Cwd()
	Log("pushd %s", dir)
	Cd(dir)
	defer func() {
		Log("popd %s", cwd)
		Cd(cwd)
	}()
	run()
}

func IsDir(dir string) bool {
	s, err := os.Stat(dir)
	if err != nil || s == nil {
		return false
	}
	return s.IsDir()
}

func IsRegularFile(name string) bool {
	s, err := os.Lstat(name)
	if err != nil {
		return false
	}
	return !s.IsDir() && s.Mode()&os.ModeSymlink == 0
}

func FingerprintFiles(files ...string) string {
	sort.Strings(files)

	hasher := md5.New() //nolint: gosec
	for _, file := range files {
		data := ReadFile(file)
		hasher.Write([]byte(data))
	}
	return string(hasher.Sum(nil))
}

func FingerprintGlobs(globs ...string) string {
	var files []string
	for _, glob := range globs {
		files = append(files, FindFiles(glob)...)
	}
	return FingerprintFiles(files...)
}

func FileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func EnsureFileExists(file string) {
	if !FileExists(file) {
		Throw(fmt.Errorf("file does not exist: %s", file))
	}
}

func FindFile(glob string) string {
	dir := Get(os.Getwd())
	return findFile(dir, glob)
}

func FindFiles(glob string) []string {
	dir := Get(os.Getwd())
	f := filepath.Join(dir, glob)
	return Get(filepath.Glob(f))
}

func ReadFile(file string) string {
	b, err := os.ReadFile(file)
	NoErr(err)
	return string(b)
}

func FileContains(file, substr string) bool {
	return strings.Contains(ReadFile(file), substr)
}

func WriteFile(contents, path string) {
	NoErr(os.WriteFile(path, []byte(contents), 0600))
}

func findFile(dir string, glob string) string {
	for {
		f := filepath.Join(dir, glob)
		matches, _ := filepath.Glob(f)
		if len(matches) > 0 {
			return matches[0]
		}
		if dir == filepath.Dir(dir) {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}

func PathJoin(paths ...string) string {
	return filepath.Join(pathStrings(paths...)...)
}

func pathStrings[From string](from ...From) []string {
	var out []string
	for _, v := range from {
		out = append(out, string(v))
	}
	return out
}
