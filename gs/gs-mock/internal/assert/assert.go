/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package assert

import (
	"fmt"
	"reflect"
	"regexp"
)

// T is the minimal interface that *testing.T satisfies.
type T interface {
	Helper()
	Errorf(format string, args ...any)
}

// isNil reports whether the given reflect.Value is nil.
func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.UnsafePointer:
		return v.IsNil()
	default:
		return !v.IsValid()
	}
}

// Nil asserts that got is nil.
// It fails if got is not nil.
//
// Why not just use `got == nil`?
// Because in Go, typed nils behave differently:
//
//	a := (*int)(nil)   // type is *int
//	b := (any)(nil)    // type is <nil>
//
// a == b is false, even though both are nil in a sense.
func Nil(t T, got any) {
	t.Helper()
	if !isNil(reflect.ValueOf(got)) {
		t.Errorf("got (%T) %v but expect nil", got, got)
	}
}

// Equal asserts that got and expect are deeply equal.
// It fails if they are not equal according to reflect.DeepEqual.
func Equal(t T, got any, expect any) {
	t.Helper()
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("got (%T) %v but expect (%T) %v", got, got, expect, expect)
	}
}

// recovery runs fn and captures any panic.
// If fn panics, it returns the panic message string and recovered=true.
// If fn does not panic, it returns an empty string and recovered=false.
func recovery(fn func()) (str string, recovered bool) {
	defer func() {
		if r := recover(); r != nil {
			str = fmt.Sprint(r)
			recovered = true
		}
	}()
	fn()
	return "", false
}

// Panic asserts that fn panics.
// If fn does not panic, it fails.
// If expr is non-empty, it must be a valid regexp that matches the panic message.
func Panic(t T, fn func(), expr string) {
	t.Helper()
	if str, ok := recovery(fn); !ok {
		t.Errorf("did not panic")
	} else {
		matches(t, str, expr)
	}
}

// matches asserts that got matches the given regular expression expr.
// If expr is empty, it fails.
// If expr is invalid or does not match, it fails.
func matches(t T, got string, expr string) {
	t.Helper()
	if expr == "" {
		t.Errorf("empty pattern")
		return
	}
	if ok, err := regexp.MatchString(expr, got); err != nil {
		t.Errorf("invalid pattern")
	} else if !ok {
		t.Errorf("got %q which does not match %q", got, expr)
	}
}
