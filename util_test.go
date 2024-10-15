package make_test

import (
	"os"
	"path/filepath"
	"testing"
)

func inDir(t *testing.T, dir string, fn func()) {
	cwd, err := os.Getwd()
	requireNoErr(t, err)
	requireNoErr(t, os.Chdir(filepath.Join(cwd, filepath.ToSlash(dir))))
	defer func() {
		requireNoErr(t, os.Chdir(cwd))
	}()
	fn()
}

func requireNoErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func requireEqual(t *testing.T, expected, actual any) {
	if expected != actual {
		t.Errorf("not equal\nexpected: %v\n     got: %v", expected, actual)
	}
}

func requireEqualElements[T comparable](t *testing.T, expected, actual []T) {
	if len(expected) != len(actual) {
		t.Errorf("not equal\nexpected: %v\n     got: %v", expected, actual)
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Errorf("not equal\nexpected: %v in idx %v %v\n     got: %v in %v", expected[i], i, expected, actual[i], actual)
		}
	}
}
