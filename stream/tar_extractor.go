package stream

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/log"
)

// TarExtractor returns a readerFn that can be passed to a WriterPipe to extract
// the contents of a tar stream to a directory
func TarExtractor(destDir string) func(r io.Reader) {
	return func(r io.Reader) {
		destDir = filepath.Clean(destDir)
		destDirPrefix := filepath.Dir(destDir) + string(os.PathSeparator)
		tr := tar.NewReader(r)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break // End of archive
			}
			if err != nil {
				return
			}

			// Security: Prevent ZipSlip (directory traversal attacks)
			target := filepath.Join(destDir, filepath.FromSlash(filepath.Clean(header.Name)))

			if !strings.HasPrefix(target, destDirPrefix) {
				log.Warn("refusing to write file outside of root dir: %s", target)
				continue
			}

			switch header.Typeflag {
			case tar.TypeDir:
				err = os.MkdirAll(target, 0755)
				if err != nil {
					log.Warn("error creating directory %s: %v", target, err)
				}
			case tar.TypeReg:
				// Ensure parent directory exists
				err = os.MkdirAll(filepath.Dir(target), 0755)
				if err != nil {
					log.Warn("unable to create parent directory for %s: %v", target, err)
					continue
				}

				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					log.Warn("unable to create file at %s: %v", target, err)
					continue
				}

				// Stream the file content directly to disk
				const gb = 1024 * 1024 * 1024
				_, err = io.CopyN(f, tr, gb)
				if err != nil {
					log.Warn("error writing file %s: %v", target, err)
				}
				err = f.Close()
				if err != nil {
					log.Debug("error writing file %s: %v", target, err)
				}
			}
		}
	}
}
