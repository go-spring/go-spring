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
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

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
func True(t *testing.T, got bool, msg ...string) {
	if !got {
		str := "got false but expect true"
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
	}
}

// False asserts that got is false.
func False(t *testing.T, got bool, msg ...string) {
	if got {
		str := "got true but expect false"
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
	}
}

// Nil asserts that got is nil.
func Nil(t *testing.T, got interface{}, msg ...string) {
	if got != nil {
		str := fmt.Sprintf("got (%T) %v but expect nil", got, got)
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
	}
}

// NotNil asserts that got is not nil.
func NotNil(t *testing.T, got interface{}, msg ...string) {
	if got == nil {
		str := "got nil but expect not nil"
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
	}
}

// Equal asserts that got and expect are equal.
func Equal(t *testing.T, got interface{}, expect interface{}, msg ...string) {
	if !reflect.DeepEqual(got, expect) {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", got, got, expect, expect)
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
	}
}

// NotEqual asserts that got and expect are not equal.
func NotEqual(t *testing.T, got interface{}, expect interface{}, msg ...string) {
	if reflect.DeepEqual(got, expect) {
		str := fmt.Sprintf("expect not (%T) %v", expect, expect)
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
	}
}

// Panic asserts that function fn() would panic. It fails if the panic
// message does not match the regular expression.
func Panic(t *testing.T, fn func(), expr string, msg ...string) {
	// TODO 使用 util.Panic(err).When(err != nil) 时堆栈信息不对
	defer func() {
		if r := recover(); r == nil {
			str := "did not panic"
			args := append([]string{str}, msg...)
			fail(t, 2, strings.Join(args, "; "))
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
			matches(t, 1, str, expr, msg...)
		}
	}()
	fn()
}

// Matches asserts that a got value matches a given regular expression.
func Matches(t *testing.T, got string, expr string, msg ...string) {
	matches(t, 1, got, expr, msg...)
}

// Error asserts that a got error string matches a given regular expression.
func Error(t *testing.T, got error, expr string, msg ...string) {
	if got == nil {
		str := fmt.Sprintf("got nil error but expect %q", expr)
		args := append([]string{str}, msg...)
		fail(t, 1, strings.Join(args, "; "))
		return
	}
	matches(t, 1, got.Error(), expr)
}

func matches(t *testing.T, skip int, got string, expr string, msg ...string) {
	if ok, err := regexp.MatchString(expr, got); err != nil {
		str := fmt.Sprintf("invalid pattern %q %s", expr, err.Error())
		fail(t, skip+1, str)
	} else if !ok {
		str := fmt.Sprintf("got %q which does not match %q", got, expr)
		args := append([]string{str}, msg...)
		fail(t, skip+1, strings.Join(args, "; "))
	}
}

func fail(t *testing.T, skip int, msg string) {
	_, file, line, _ := runtime.Caller(skip + 1)
	fmt.Printf("\t%s:%d: %s\n", filepath.Base(file), line, msg)
	t.Fail()
}
