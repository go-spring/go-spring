/*
 * Copyright 2012-2019 the original author or authors.
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

//go:generate mockgen -build_flags="-mod=mod" -package=assert -source=assert.go -destination=assert_mock.go

// Package assert provides some useful assertion methods.
package assert

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-spring/spring-base/json"
)

// T is the minimum interface of *testing.T.
type T interface {
	Helper()
	Error(args ...interface{})
}

func fail(t T, str string, msg ...string) {
	t.Helper()
	args := append([]string{str}, msg...)
	t.Error(strings.Join(args, "; "))
}

// True assertion failed when got is false.
func True(t T, got bool, msg ...string) {
	t.Helper()
	if !got {
		fail(t, "got false but expect true", msg...)
	}
}

// False assertion failed when got is true.
func False(t T, got bool, msg ...string) {
	t.Helper()
	if got {
		fail(t, "got true but expect false", msg...)
	}
}

// isNil reports v is nil, but will not panic.
func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Ptr,
		reflect.Slice,
		reflect.UnsafePointer:
		return v.IsNil()
	}
	return !v.IsValid()
}

// Nil assertion failed when got is not nil.
func Nil(t T, got interface{}, msg ...string) {
	t.Helper()
	// Why can't we use got==nil to judge？Because if
	// a := (*int)(nil)        // %T == *int
	// b := (interface{})(nil) // %T == <nil>
	// then a==b is false, because they are different types.
	if !isNil(reflect.ValueOf(got)) {
		str := fmt.Sprintf("got (%T) %v but expect nil", got, got)
		fail(t, str, msg...)
	}
}

// NotNil assertion failed when got is nil.
func NotNil(t T, got interface{}, msg ...string) {
	t.Helper()
	if isNil(reflect.ValueOf(got)) {
		fail(t, "got nil but expect not nil", msg...)
	}
}

// Equal assertion failed when got and expect are not `deeply equal`.
func Equal(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, expect) {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", got, got, expect, expect)
		fail(t, str, msg...)
	}
}

// NotEqual assertion failed when got and expect are `deeply equal`.
func NotEqual(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()
	if reflect.DeepEqual(got, expect) {
		str := fmt.Sprintf("got (%T) %v but expect not (%T) %v", got, got, expect, expect)
		fail(t, str, msg...)
	}
}

// JsonEqual assertion failed when got and expect are not `json equal`.
func JsonEqual(t T, got string, expect string, msg ...string) {
	t.Helper()
	var gotJson interface{}
	if err := json.Unmarshal([]byte(got), &gotJson); err != nil {
		fail(t, err.Error(), msg...)
		return
	}
	var expectJson interface{}
	if err := json.Unmarshal([]byte(expect), &expectJson); err != nil {
		fail(t, err.Error(), msg...)
		return
	}
	if !reflect.DeepEqual(gotJson, expectJson) {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", got, got, expect, expect)
		fail(t, str, msg...)
	}
}

// Same assertion failed when got and expect are not same.
func Same(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()
	if got != expect {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", got, got, expect, expect)
		fail(t, str, msg...)
	}
}

// NotSame assertion failed when got and expect are same.
func NotSame(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()
	if got == expect {
		str := fmt.Sprintf("expect not (%T) %v", expect, expect)
		fail(t, str, msg...)
	}
}

// Panic assertion failed when fn doesn't panic or not match expr expression.
func Panic(t T, fn func(), expr string, msg ...string) {
	t.Helper()
	str := recovery(fn)
	if str == "<<SUCCESS>>" {
		fail(t, "did not panic", msg...)
	} else {
		matches(t, str, expr, msg...)
	}
}

func recovery(fn func()) (str string) {
	defer func() {
		if r := recover(); r != nil {
			str = fmt.Sprint(r)
		}
	}()
	fn()
	return "<<SUCCESS>>"
}

// Matches assertion failed when got doesn't match expr expression.
func Matches(t T, got string, expr string, msg ...string) {
	t.Helper()
	matches(t, got, expr, msg...)
}

// Error assertion failed when got `error` doesn't match expr expression.
func Error(t T, got error, expr string, msg ...string) {
	t.Helper()
	if got == nil {
		fail(t, "expect not nil error", msg...)
		return
	}
	matches(t, got.Error(), expr, msg...)
}

func matches(t T, got string, expr string, msg ...string) {
	t.Helper()
	if ok, err := regexp.MatchString(expr, got); err != nil {
		fail(t, "invalid pattern", msg...)
	} else if !ok {
		str := fmt.Sprintf("got %q which does not match %q", got, expr)
		fail(t, str, msg...)
	}
}

// TypeOf assertion failed when got and expect are not same type.
func TypeOf(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()

	e2 := reflect.TypeOf(expect)
	if e2.Kind() == reflect.Ptr {
		if e2.Elem().Kind() == reflect.Interface {
			e2 = e2.Elem()
		}
	}

	e1 := reflect.TypeOf(got)
	if !e1.AssignableTo(e2) {
		str := fmt.Sprintf("got type (%s) but expect type (%s)", e1, e2)
		fail(t, str, msg...)
	}
}

// Implements assertion failed when got doesn't implement expect.
func Implements(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()

	e2 := reflect.TypeOf(expect)
	if e2.Kind() == reflect.Ptr {
		if e2.Elem().Kind() == reflect.Interface {
			e2 = e2.Elem()
		} else {
			fail(t, "expect should be interface", msg...)
			return
		}
	}

	e1 := reflect.TypeOf(got)
	if !e1.Implements(e2) {
		str := fmt.Sprintf("got type (%s) but expect type (%s)", e1, e2)
		fail(t, str, msg...)
	}
}
