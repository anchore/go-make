package make

import (
	"os"
	"path/filepath"
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

func FileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func FindFile(glob string) string {
	dir := Get(os.Getwd())
	return findFile(dir, glob)
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
