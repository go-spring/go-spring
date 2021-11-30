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

// Package assert 提供了一些常用的断言函数。
package assert

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// T testing.T 的简化接口。
type T interface {
	Helper()
	Fail()
	Log(args ...interface{})
}

type Cases = []struct {
	Condition bool
	Message   string
}

// Check 用于检查参数有效性。
func Check(cases Cases) error {
	buf := bytes.Buffer{}
	for _, c := range cases {
		if c.Condition {
			continue
		}
		buf.WriteString(c.Message)
		buf.WriteString("; ")
	}
	if buf.Len() == 0 {
		return nil
	}
	return errors.New(string(buf.Bytes()[:buf.Len()-2]))
}

// True asserts that got is true.
func True(t T, got bool, msg ...string) {
	t.Helper()
	if !got {
		fail(t, "got false but expect true", msg...)
	}
}

// False asserts that got is false.
func False(t T, got bool, msg ...string) {
	t.Helper()
	if got {
		fail(t, "got true but expect false", msg...)
	}
}

// Nil asserts that got is nil.
func Nil(t T, got interface{}, msg ...string) {
	t.Helper()
	if got != nil {
		str := fmt.Sprintf("got (%T) %v but expect nil", got, got)
		fail(t, str, msg...)
	}
}

// NotNil asserts that got is not nil.
func NotNil(t T, got interface{}, msg ...string) {
	t.Helper()
	if got == nil {
		fail(t, "got nil but expect not nil", msg...)
	}
}

// Equal asserts that got and expect are equal.
func Equal(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, expect) {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", got, got, expect, expect)
		fail(t, str, msg...)
	}
}

// NotEqual asserts that got and expect are not equal.
func NotEqual(t T, got interface{}, expect interface{}, msg ...string) {
	t.Helper()
	if reflect.DeepEqual(got, expect) {
		str := fmt.Sprintf("expect not (%T) %v", expect, expect)
		fail(t, str, msg...)
	}
}

// Panic asserts that function fn() would panic. It fails if the panic
// message does not match the regular expression.
func Panic(t T, fn func(), expr string, msg ...string) {
	// TODO 使用 util.Panic(err).When(err != nil) 时堆栈信息不对
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			fail(t, "did not panic", msg...)
		} else {
			var str string
			switch v := r.(type) {
			case error:
				str = v.Error()
			case string:
				str = v
			default:
				str = fmt.Sprint(r)
			}
			matches(t, str, expr, msg...)
		}
	}()
	fn()
}

// Matches asserts that a got value matches a given regular expression.
func Matches(t T, got string, expr string, msg ...string) {
	t.Helper()
	matches(t, got, expr, msg...)
}

// Error asserts that a got error string matches a given regular expression.
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

func fail(t T, str string, msg ...string) {
	t.Helper()
	args := append([]string{str}, msg...)
	t.Log(strings.Join(args, "; "))
	t.Fail()
}
