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

package assert

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

// Equal asserts that expect and got are equal as defined by reflect.DeepEqual.
func Equal(t *testing.T, expect interface{}, got interface{}) {
	if !reflect.DeepEqual(expect, got) {
		fail(t, 1, "expect %v but got %v", expect, got)
	}
}

// Panic asserts that function fn() would panic. It fails if the panic message
// does not match the regular expression in 'expr'.
func Panic(t *testing.T, fn func(), expr string) {
	defer func() {
		if r := recover(); r == nil {
			fail(t, 1, "did not panic")
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
			matches(t, 1, expr, v)
		}
	}()
	fn()
}

// Matches asserts that a got value matches a given regular expression.
func Matches(t *testing.T, expr string, got string) {
	matches(t, 1, expr, got)
}

func matches(t *testing.T, skip int, expr string, got string) {
	if ok, err := regexp.MatchString(expr, got); err != nil {
		fail(t, skip+1, "invalid pattern %q %s", expr, err.Error())
	} else if !ok {
		fail(t, skip+1, "got %s which does not match %s", got, expr)
	}
}

func fail(t *testing.T, skip int, format string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(skip + 1)
	fmt.Printf("\t%s:%d: %s\n", filepath.Base(file), line, fmt.Sprintf(format, args...))
	t.Fail()
}
