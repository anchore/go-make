package require

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func InDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	cwd, err := os.Getwd()
	NoError(t, err)
	NoError(t, os.Chdir(filepath.Join(cwd, filepath.ToSlash(dir))))
	defer func() {
		NoError(t, os.Chdir(cwd))
	}()
	fn()
}

func Error(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("error was expected")
	}
}

func NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func Contains(t *testing.T, values any, value any) {
	t.Helper()
	if value, ok := value.(string); ok {
		if values, ok := values.(string); ok && strings.Contains(values, value) {
			return
		}
		if values, ok := values.([]string); ok && slices.Contains(values, value) {
			return
		}
	}
	t.Errorf("error: %v not contained in %v", value, values)
}

func Equal[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Errorf("not equal\nexpected: %v\n     got: %v", expected, actual)
	}
}

func EqualElements[T comparable](t *testing.T, expected, actual []T) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("not equal\nexpected: %v\n     got: %v", expected, actual)
	}
	for i := range expected {
		found := false
		for j := range actual {
			if expected[i] == actual[j] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("not equal\nexpected: %v in idx %v %v\n     got: %v in %v", expected[i], i, expected, actual[i], actual)
		}
	}
}
