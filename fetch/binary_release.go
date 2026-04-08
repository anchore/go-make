package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/template"
)

const GB = 1024 * 1024 * 1024

// MaxFileSize is the maximum allowed size for extracted files (default 2GB).
// This is a safety limit to prevent zip bombs or accidental extraction of huge files.
var MaxFileSize = 2 * GB

// ReleaseSpec defines how to download and extract a binary release from a URL.
// The URL is a Go template that can reference variables from Args, Platform,
// and the built-in {{os}} and {{arch}} values.
type ReleaseSpec struct {
	// URL is the URL template for downloading. Built-in variables include:
	//   - {{os}}: runtime.GOOS (e.g., "linux", "darwin", "windows")
	//   - {{arch}}: runtime.GOARCH (e.g., "amd64", "arm64")
	// Additional variables can be defined in Args and Platform.
	URL string

	// Args provides default template values for all platforms.
	Args map[string]string

	// Platform provides platform-specific overrides for Args. Keys can be:
	//   - OS only: "linux", "darwin", "windows"
	//   - Arch only with wildcard: "*/arm64", "*/amd64"
	//   - OS/Arch pair: "darwin/arm64", "linux/amd64"
	// More specific keys take precedence over less specific ones.
	Platform map[string]map[string]string
}

// BinaryRelease downloads a binary archive from the URL defined in spec, extracts
// the file matching toolPath's basename, and writes it to toolPath with executable
// permissions (0500). Supports .zip and .tar.gz archives.
func BinaryRelease(toolPath string, spec ReleaseSpec) error {
	url := spec.render(runtime.GOOS, runtime.GOARCH)

	buf := bytes.Buffer{}
	_, err := Fetch(url, Writer(&buf))
	if err != nil {
		return err
	}
	contents := buf.Bytes()
	contents = getArchiveFileContents(contents, filepath.Base(toolPath))
	if contents == nil {
		return fmt.Errorf("unable to read archive from: %v", url)
	}
	dir := filepath.Dir(toolPath)
	if !file.Exists(dir) {
		lang.Throw(os.MkdirAll(dir, 0o700|os.ModeDir))
	}
	return os.WriteFile(toolPath, contents, 0o500) //nolint:gosec // needs read + execute permissions
}

func getArchiveFileContents(archive []byte, file string) []byte {
	var errs []error

	contents, err := getZipArchiveFileContents(archive, file)
	if err == nil && len(contents) > 0 {
		return contents
	}
	errs = append(errs, err)

	contents, err = getTarGzArchiveFileContents(archive, file)
	if err == nil && len(contents) > 0 {
		return contents
	}
	errs = append(errs, err)

	panic(fmt.Errorf("unable to read archive after attempting readers: %w", errors.Join(errs...)))
}

func getZipArchiveFileContents(archive []byte, file string) ([]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	f, err := zipReader.Open(file)
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func getTarGzArchiveFileContents(archive []byte, fileName string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err == nil && gzipReader != nil {
		t := tar.NewReader(gzipReader)
		for {
			hdr, err := t.Next()
			if err != nil {
				return nil, err
			}
			if hdr.Name == fileName {
				if hdr.Size > int64(MaxFileSize) {
					return nil, fmt.Errorf("refusing to extract file %v larger than %s, declared size: %v", fileName, file.HumanizeBytes(MaxFileSize), file.HumanizeBytes(hdr.Size))
				}
				return io.ReadAll(t)
			}
		}
	}
	return nil, fmt.Errorf("file not found: %v", fileName)
}

func (s ReleaseSpec) render(os, arch string) string {
	args := map[string]any{
		"os":   os,
		"arch": arch,
	}
	for k, v := range s.Args {
		args[k] = v
	}
	if s.Platform != nil {
		for k, v := range s.Platform[os] {
			args[k] = v
		}
		for k, v := range s.Platform["*/"+arch] {
			args[k] = v
		}
		for k, v := range s.Platform[os+"/"+arch] {
			args[k] = v
		}
	}
	return template.Render(s.URL, args)
}
