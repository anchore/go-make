package file

import (
	"crypto/md5" //nolint: gosec
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/template"
)

// Cd changes the current working directory to the provided relative or absolute directory
func Cd(dir string) {
	lang.Throw(os.Chdir(dir))
}

// Cwd returns the current working directory
func Cwd() string {
	return lang.Return(os.Getwd())
}

// Copy copies the source file to the destination file, preserving permissions
func Copy(src, dst string) {
	perms := lang.Return(os.Stat(src)).Mode()
	contents := lang.Return(os.ReadFile(src))
	EnsureDir(filepath.Dir(dst))
	lang.Throw(os.WriteFile(dst, contents, perms)) //nolint:gosec // G703: dst is caller-controlled build utility path
}

// Delete removes the given file or directory recursively. As a safety measure,
// it verifies the path is under RootDir before deletion to prevent accidental
// deletion of files outside the project. Panics if the path is outside RootDir.
func Delete(path string) {
	dirToRm := lang.Return(filepath.Abs(path))
	rootDir := lang.Return(filepath.Abs(template.Render(config.RootDir)))

	if strings.HasPrefix(dirToRm, rootDir) {
		log.Info(color.Yellow(`delete: %v`), dirToRm)
		lang.Throw(os.RemoveAll(dirToRm))
	} else {
		panic(fmt.Errorf("directory '%s' not in RootDir: '%s'", dirToRm, rootDir))
	}
}

// InDir executes the given function in the provided directory, returning to the current working directory upon completion
func InDir(dir string, run func()) {
	cwd := Cwd()
	log.Debug("pushd %s", dir)
	Cd(dir)
	defer func() {
		log.Debug("popd %s", cwd)
		Cd(cwd)
	}()
	run()
}

// WithTempDir creates a temporary directory and passes it to the provided function.
// The directory is automatically deleted when the function returns, unless config.Cleanup
// is false (which happens in Debug mode or CI environments for debugging purposes).
func WithTempDir(fn func(dir string)) {
	tmp := lang.Return(os.MkdirTemp(config.TmpDir, "buildtools-tmp-"))
	if config.Cleanup {
		defer func() {
			log.Error(os.RemoveAll(tmp))
		}()
	}
	fn(tmp)
}

// InTempDir executes with the current working directory in a new temporary directory, restoring cwd and removing all contents of the temp directory upon completion
func InTempDir(fn func()) {
	WithTempDir(func(tmp string) {
		InDir(tmp, fn)
	})
}

// Exists indicates a file of any type (regular, directory, symlink, etc.) exists and is readable
func Exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// IsDir indicates the provided directory exists and is a directory
func IsDir(dir string) bool {
	s, err := os.Stat(dir)
	if err != nil || s == nil {
		return false
	}
	return s.IsDir()
}

// IsEmpty indicates the provided directory is empty or does not exist
func IsEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	return len(entries) == 0
}

// EnsureDir checks if the directory exists. create if not, including any subdirectories needed
// and returns the absolute path to the directory
func EnsureDir(dir string) string {
	dir = lang.Return(filepath.Abs(dir))
	s, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		lang.Throw(os.MkdirAll(dir, 0o755))
		s, err = os.Stat(dir)
	}
	if s == nil || !s.IsDir() {
		panic(fmt.Errorf("path '%s' is not a directory", dir))
	}
	lang.Throw(err)
	return dir
}

// IsRegular indicates the provided file exists and is a regular file, not a directory or symlink
func IsRegular(name string) bool {
	s, err := os.Lstat(name)
	if err != nil {
		return false
	}
	return !s.IsDir() && s.Mode()&os.ModeSymlink == 0
}

// Fingerprint computes an MD5 hash of all files matching the given glob patterns.
// Files are sorted before hashing to ensure a stable, reproducible result. Useful
// for cache invalidation or detecting changes in a set of files.
//
// Supports doublestar glob syntax (e.g., "**/*.go" for recursive matching).
func Fingerprint(globs ...string) string {
	var files []string
	for _, glob := range globs {
		files = append(files, FindAll(glob)...)
	}

	sort.Strings(files)

	hasher := md5.New() //nolint: gosec
	for _, file := range files {
		if IsDir(file) {
			log.Trace("fingerprinting: %s", file)
			continue
		}
		log.Trace("fingerprinting: %s", file)
		streamFile(file, hasher)
	}

	fingerprint := fmt.Sprintf("%x", hasher.Sum(nil))
	log.Debug("fingerprinted globs %v: %s", globs, fingerprint)
	return fingerprint
}

// Require panics if the provided file does not exist
func Require(file string) {
	if !Exists(file) {
		panic(fmt.Errorf("file does not exist: %s", file))
	}
}

// FindAll returns all files matching the given glob pattern. Uses the doublestar
// library, supporting "**" for recursive directory matching.
//
// Example:
//
//	FindAll("**/*.go")           // all Go files recursively
//	FindAll("cmd/**/main.go")    // main.go files under cmd/
func FindAll(glob string) []string {
	return lang.Return(doublestar.FilepathGlob(glob, doublestar.WithFilesOnly()))
}

// FindParent traverses up the directory tree from dir, looking for a file matching
// the glob pattern. Returns the first match found, or an empty string if none found.
// Useful for finding config files like .binny.yaml or .git in parent directories.
func FindParent(dir string, glob string) string {
	for {
		f := filepath.Join(dir, glob)
		matches, _ := doublestar.FilepathGlob(f)
		if len(matches) > 0 {
			return matches[0]
		}
		if dir == filepath.Dir(dir) {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}

// Read reads the file and returns the contents as a string
func Read(file string) string {
	b, err := os.ReadFile(file)
	lang.Throw(err)
	return string(b)
}

// Contains indicates the given file contains the provided substring
func Contains(file, substr string) bool {
	return strings.Contains(Read(file), substr)
}

// Write writes the provided contents to a file
func Write(path, contents string) {
	lang.Throw(os.WriteFile(path, []byte(contents), 0o600))
}

// JoinPaths joins paths together using an OS-appropriate separator
func JoinPaths(paths ...string) string {
	return filepath.Join(paths...)
}

func streamFile(file string, writer io.Writer) {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer lang.Close(f, file)
	_ = lang.Return(io.Copy(writer, f))
}
