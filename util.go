package make

import (
	"fmt"
	"io"
	"iter"
	"maps"
	"os"
	"reflect"
	"slices"
)

var NewLine = fmt.Sprintln()

var LogPrefix = ""

var Log = func(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, LogPrefix+Tpl(format)+NewLine, args...)
}

func LogErr(err error) {
	if err != nil {
		Log("%v", err)
	}
}

func NoErr(e error) {
	if e != nil {
		Throw(e)
	}
}

func Get[T any](t T, e error) T {
	NoErr(e)
	return t
}

func All[T any](values ...T) []T {
	return values
}

func AllNotNil[T any](values ...T) []T {
	for i := 0; i < len(values); i++ {
		if reflect.ValueOf(values[i]).IsNil() {
			values = append(values[0:i], values[i+1:]...)
			i--
		}
	}
	return values
}

func remove[T comparable](values []T, toRemove T) []T {
	for i := 0; i < len(values); i++ {
		if values[i] == toRemove {
			values = append(values[0:i], values[i+1:]...)
		}
	}
	return values
}

func sortedMapIter[K ~string, V any](values map[K]V) iter.Seq2[K, V] {
	var keys []K
	for k := range maps.Keys(values) {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return func(yield func(K, V) bool) {
		for _, k := range keys {
			if !yield(k, values[k]) {
				return
			}
		}
	}
}

func Close(closeable io.Closer) {
	if closeable != nil {
		LogErr(closeable.Close())
	}
}
