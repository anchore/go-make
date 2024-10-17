package make

import (
	"crypto/md5" //nolint: gosec
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RootDir is a function to return the root directory which builds run from
var RootDir = func() string {
	return Tpl("{{RepoRoot}}")
}

// TmpDirRoot is a directory to use as the base for temporary directories, defaults to system temp dir if empty
var TmpDirRoot = ""

// Cd changes the current working directory to the provided relative or absolute directory
func Cd(dir string) {
	NoErr(os.Chdir(dir))
}

// Cwd returns the current working directory
func Cwd() string {
	return Get(os.Getwd())
}

// InDir executes the given function in the provided directory, returning to the current working directory upon completion
func InDir(dir string, run func()) {
	cwd := Cwd()
	Log("pushd %s", dir)
	Cd(dir)
	defer func() {
		Log("popd %s", cwd)
		Cd(cwd)
	}()
	run()
}

// WithTempDir creates a temporary directory, provided for the duration of the function call, removing all contents upon completion
func WithTempDir(fn func(dir string)) {
	tmp := Get(os.MkdirTemp(TmpDirRoot, "buildtools-tmp-"))
	defer func() {
		LogErr(os.RemoveAll(tmp))
	}()
	fn(tmp)
}

// InTempDir executes with current working directory in a new temporary directory, restoring cwd and removing all contents of the temp directory upon completion
func InTempDir(fn func()) {
	WithTempDir(func(tmp string) {
		InDir(tmp, fn)
	})
}

// IsDir indicates the provided directory exists and is a directory
func IsDir(dir string) bool {
	s, err := os.Stat(dir)
	if err != nil || s == nil {
		return false
	}
	return s.IsDir()
}

// IsRegularFile indicates the provided file exists and is a regular file, not a directory or symlink
func IsRegularFile(name string) bool {
	s, err := os.Lstat(name)
	if err != nil {
		return false
	}
	return !s.IsDir() && s.Mode()&os.ModeSymlink == 0
}

// FingerprintFiles hashes all files and provides a stable hash of all the contents
func FingerprintFiles(files ...string) string {
	sort.Strings(files)

	hasher := md5.New() //nolint: gosec
	for _, file := range files {
		data := ReadFile(file)
		hasher.Write([]byte(data))
	}
	return string(hasher.Sum(nil))
}

// FingerprintGlobs fingerprints  all files matching the provided glob expression, providing a stable hash of all contents
func FingerprintGlobs(globs ...string) string {
	var files []string
	for _, glob := range globs {
		files = append(files, FindFiles(glob)...)
	}
	return FingerprintFiles(files...)
}

// FileExists indicates a file of any type (regular, directory, symlink, etc.) exists and is readable
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// EnsureFileExists halts execution if the provided file does not exist
func EnsureFileExists(file string) {
	if !FileExists(file) {
		Throw(fmt.Errorf("file does not exist: %s", file))
	}
}

// FindFile finds the first matching file given a glob expression
func FindFile(glob string) string {
	dir := Get(os.Getwd())
	return findFile(dir, glob)
}

// FindFiles finds all matching files given a glob expression
func FindFiles(glob string) []string {
	dir := Get(os.Getwd())
	f := filepath.Join(dir, glob)
	return Get(filepath.Glob(f))
}

// ReadFile reads the file and returns the contents as a string
func ReadFile(file string) string {
	b, err := os.ReadFile(file)
	NoErr(err)
	return string(b)
}

// FileContains indicates the given file contains the provided substring
func FileContains(file, substr string) bool {
	return strings.Contains(ReadFile(file), substr)
}

// WriteFile writes the provided contents to a file
func WriteFile(path, contents string) {
	NoErr(os.WriteFile(path, []byte(contents), 0600))
}

// PathJoin joins paths together using OS-appropriate separator
func PathJoin(paths ...string) string {
	return filepath.Join(paths...)
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
