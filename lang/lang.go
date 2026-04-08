package lang

import (
	"io"
	"reflect"

	"github.com/anchore/go-make/log"
)

// Default returns the first non-zero value from the provided values. Useful for
// providing fallback values in a chain.
//
// Example:
//
//	name := Default(os.Getenv("NAME"), config.DefaultName, "anonymous")
func Default[T comparable](values ...T) T {
	var def T
	for _, v := range values {
		if v != def {
			return v
		}
	}
	return def
}

// Continue returns the value regardless of any error. If there's an error, it's
// logged but execution continues. Use when errors are acceptable and shouldn't
// halt the build.
func Continue[T any](t T, e error) T {
	log.Error(e)
	return t
}

// Return returns the value if error is nil, otherwise panics with the error.
// This is the standard pattern for error handling in go-make tasks.
//
// Example:
//
//	contents := lang.Return(os.ReadFile("config.yaml"))
func Return[T any](t T, e error) T {
	Throw(e)
	return t
}

// List returns a slice containing all non-empty values. Values that are nil,
// zero-length strings, empty slices/maps, or other "empty" values are filtered out.
// Useful for building lists where some values may be conditionally present.
//
// Example:
//
//	args := List("build", verbose && "-v", "-o", output)  // filters out false
func List[T any](values ...T) []T {
	for i := 0; i < len(values); i++ {
		if isEmpty(reflect.ValueOf(values[i])) {
			values = append(values[0:i], values[i+1:]...)
			i--
		}
	}
	return values
}

// Remove returns a new slice with values removed based on true returns from shouldRemove
func Remove[T comparable](values []T, shouldRemove func(T) bool) []T {
	var out []T
	for i := 0; i < len(values); i++ {
		if shouldRemove(values[i]) {
			continue
		}
		out = append(out, values[i])
	}
	return out
}

// Map returns a new slice with values mapped from incoming to outgoing in mapFunc
func Map[From, To any](values []From, mapFunc func(From) To) []To {
	out := make([]To, len(values))
	for i := 0; i < len(values); i++ {
		out[i] = mapFunc(values[i])
	}
	return out
}

// isEmpty returns true if the value seems to be an empty value: default, invalid, nil, 0-element slice, etc.
func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface, reflect.Func:
		return v.IsNil()
	default:
		return !v.IsValid() || v.IsZero()
	}
}

// for go1.23+
// func sortedMapIter[K ~string, V any](values map[K]V) iter.Seq2[K, V] {
//	var keys []K
//	for k := range maps.Keys(values) {
//		keys = append(keys, k)
//	}
//	slices.Sort(keys)
//	return func(yield func(K, V) bool) {
//		for _, k := range keys {
//			if !yield(k, values[k]) {
//				return
//			}
//		}
//	}
//}

// Close safely closes an io.Closer and logs any error. Unlike a bare defer close(),
// this doesn't lose the error and provides context for debugging. Use in place of
// defer file.Close() patterns.
//
// Example:
//
//	f := lang.Return(os.Open(path))
//	defer lang.Close(f, path)
func Close(closeable io.Closer, context ...any) {
	if closeable != nil {
		log.Error(closeable.Close(), context...)
	}
}
