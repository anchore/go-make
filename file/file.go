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

// Delete removes the given file or directory, first verifying it is a subdirectory of RootDir
func Delete(path string) {
	dirToRm := lang.Return(filepath.Abs(path))
	rootDir := lang.Return(filepath.Abs(template.Render(config.RootDir)))

	if strings.HasPrefix(dirToRm, rootDir) {
		log.Log(color.Red(`deleting: %v`), dirToRm)
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

// WithTempDir creates a temporary directory, provided for the duration of the function call, removing all contents upon completion
func WithTempDir(fn func(dir string)) {
	tmp := lang.Return(os.MkdirTemp(config.TmpDir, "buildtools-tmp-"))
	defer func() {
		log.Error(os.RemoveAll(tmp))
	}()
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

// EnsureDir checks if the directory exists. create if not, including any subdirectories needed
func EnsureDir(dir string) {
	s, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		lang.Throw(os.MkdirAll(dir, 0o755))
		s, err = os.Stat(dir)
	}
	if s == nil || !s.IsDir() {
		panic(fmt.Errorf("path '%s' is not a directory", dir))
	}
	lang.Throw(err)
}

// IsRegular indicates the provided file exists and is a regular file, not a directory or symlink
func IsRegular(name string) bool {
	s, err := os.Lstat(name)
	if err != nil {
		return false
	}
	return !s.IsDir() && s.Mode()&os.ModeSymlink == 0
}

// Fingerprint hashes all files and provides a stable hash of all the contents
func Fingerprint(globs ...string) string {
	var files []string
	for _, glob := range globs {
		files = append(files, FindAll(glob)...)
	}

	sort.Strings(files)

	hasher := md5.New() //nolint: gosec
	for _, file := range files {
		if IsDir(file) {
			log.Debug("fingerprinting: %s", file)
			continue
		}
		log.Debug("fingerprinting: %s", file)
		_ = lang.Return(io.Copy(hasher, lang.Return(os.Open(file))))
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// Require panics if the provided file does not exist
func Require(file string) {
	if !Exists(file) {
		panic(fmt.Errorf("file does not exist: %s", file))
	}
}

// Find finds the first matching file given a glob expression in the CWD
func Find(glob string) string {
	matches := FindAll(glob)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// FindAll finds all matching files given a glob expression
func FindAll(glob string) []string {
	return lang.Return(doublestar.FilepathGlob(glob))
}

// FindParent finds the first matching file in the specified directory or any parent directory
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
