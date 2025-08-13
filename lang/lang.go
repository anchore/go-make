package lang

import (
	"io"
	"reflect"

	"github.com/anchore/go-make/log"
)

// Default returns the first value that does not equal the type's default value
func Default[T comparable](value T, defaultValue T) T {
	var def T
	if value == def {
		return defaultValue
	}
	return value
}

// Return returns the provided value, panicking if a non-nil error is provided
func Return[T any](t T, e error) T {
	Throw(e)
	return t
}

// Return2 returns the provided values, panicking if a non-nil error is provided
func Return2[T1, T2 any](t1 T1, t2 T2, e error) (T1, T2) {
	Throw(e)
	return t1, t2
}

// List returns a slice containing all the provided values, removing any nil or "empty" values
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

func Close(closeable io.Closer, context ...any) {
	if closeable != nil {
		log.Error(closeable.Close(), context...)
	}
}
