package require

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func Test(t *testing.T) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("failed due: %v", r)
		}
	}()
}

func True(t *testing.T, check bool) {
	t.Helper()
	if !check {
		t.Errorf("expected true value")
	}
}

type ValidationError func(t *testing.T, err error)

func (v ValidationError) Validate(t *testing.T, err error) {
	t.Helper()
	if v == nil {
		NoError(t, err)
	} else {
		v(t, err)
	}
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

func Equal(t *testing.T, expected, actual any) {
	t.Helper()
	v1 := reflect.ValueOf(expected)
	if !v1.Comparable() {
		if reflect.DeepEqual(expected, actual) {
			return
		}
		if fmt.Sprintf("%#v", expected) == fmt.Sprintf("%#v", actual) {
			return
		}
	} else if expected == actual {
		return
	}
	t.Errorf("not equal\nexpected: \"%v\"\n     got: \"%v\"", expected, actual)
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

func SetAndRestore[T any](t *testing.T, ptrToVar *T, newValue T) {
	t.Helper()
	origValue := *ptrToVar
	t.Cleanup(func() {
		*ptrToVar = origValue
	})
	*ptrToVar = newValue
}
