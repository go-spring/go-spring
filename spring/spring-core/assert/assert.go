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

// Package assert 提供了一些断言函数。
package assert

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

// True asserts that got is true.
func True(t *testing.T, got bool) {
	if !got {
		fail(t, 1, "got false but expect true")
	}
}

// False asserts that got is false.
func False(t *testing.T, got bool) {
	if got {
		fail(t, 1, "got true but expect false")
	}
}

// Nil asserts that got is nil.
func Nil(t *testing.T, got interface{}) {
	if got != nil {
		fail(t, 1, "got not nil but expect nil")
	}
}

// NotNil asserts that got is not nil.
func NotNil(t *testing.T, got interface{}) {
	if got == nil {
		fail(t, 1, "got nil but expect not nil")
	}
}

// Equal asserts that got and expect are equal as defined by reflect.DeepEqual.
func Equal(t *testing.T, got interface{}, expect interface{}) {
	if !reflect.DeepEqual(got, expect) {
		fail(t, 1, "got %v but expect %v", got, expect)
	}
}

// NotEqual asserts that got and expect are not equal as defined by reflect.DeepEqual.
func NotEqual(t *testing.T, got interface{}, expect interface{}) {
	if reflect.DeepEqual(got, expect) {
		fail(t, 1, "got %v but expect %v", got, expect)
	}
}

// Panic asserts that function fn() would panic. It fails if the panic message
// does not match the regular expression in 'expr'.
func Panic(t *testing.T, fn func(), expr string) {
	defer func() {
		if r := recover(); r == nil {
			fail(t, 2, "did not panic")
		} else {
			var v string
			switch r.(type) {
			case error:
				v = r.(error).Error()
			case string:
				v = r.(string)
			default:
				v = fmt.Sprint(r)
			}
			matches(t, 1, v, expr)
		}
	}()
	fn()
}

// Matches asserts that a got value matches a given regular expression.
func Matches(t *testing.T, got string, expr string) {
	matches(t, 1, got, expr)
}

func matches(t *testing.T, skip int, got string, expr string) {
	if ok, err := regexp.MatchString(expr, got); err != nil {
		fail(t, skip+1, "invalid pattern %s %s", expr, err.Error())
	} else if !ok {
		fail(t, skip+1, "got %s which does not match %s", got, expr)
	}
}

func fail(t *testing.T, skip int, format string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(skip + 1)
	fmt.Printf("\t%s:%d: %s\n", filepath.Base(file), line, fmt.Sprintf(format, args...))
	t.Fail()
}

// Error asserts that a got error string matches a given regular expression.
func Error(t *testing.T, got error, expr string) {
	if got == nil {
		fail(t, 1, "err is nil")
		return
	}
	matches(t, 1, got.Error(), expr)
}
