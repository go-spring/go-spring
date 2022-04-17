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
	"reflect"
	"strings"
)

type Assertion struct {
	t T
	v interface{}
}

func That(t T, v interface{}) *Assertion {
	return &Assertion{
		t: t,
		v: v,
	}
}

// IsNil 返回 v 的值是否为 nil，但是不会 panic 。
func IsNil(v reflect.Value) bool {
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

func (a *Assertion) IsNil(msg ...string) *Assertion {
	a.t.Helper()
	// 为什么不能使用 got == nil 进行判断呢？因为如果
	// a := (*int)(nil)        // %T == *int
	// b := (interface{})(nil) // %T == <nil>
	// 那么 a==b 的结果是 false，因为二者类型不一致。
	if !IsNil(reflect.ValueOf(a.v)) {
		str := fmt.Sprintf("got (%T) %v but expect nil", a.v, a.v)
		fail(a.t, str, msg...)
	}
	return a
}

func (a *Assertion) IsNotNil(msg ...string) *Assertion {
	a.t.Helper()
	if IsNil(reflect.ValueOf(a.v)) {
		fail(a.t, "got nil but expect not nil", msg...)
	}
	return a
}

func (a *Assertion) IsTrue(msg ...string) *Assertion {
	a.t.Helper()
	v := a.v.(bool)
	if !v {
		fail(a.t, "got false but expect true", msg...)
	}
	return a
}

func (a *Assertion) IsFalse(msg ...string) *Assertion {
	a.t.Helper()
	v := a.v.(bool)
	if v {
		fail(a.t, "got true but expect false", msg...)
	}
	return a
}

func (a *Assertion) IsEqualTo(expect interface{}, msg ...string) *Assertion {
	a.t.Helper()
	if !reflect.DeepEqual(a.v, expect) {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", a.v, a.v, expect, expect)
		fail(a.t, str, msg...)
	}
	return a
}

func (a *Assertion) IsNotEqualTo(expect interface{}, msg ...string) *Assertion {
	a.t.Helper()
	if reflect.DeepEqual(a.v, expect) {
		str := fmt.Sprintf("expect not (%T) %v", expect, expect)
		fail(a.t, str, msg...)
	}
	return a
}

func (a *Assertion) IsSame(expect interface{}, msg ...string) *Assertion {
	a.t.Helper()
	if a.v != expect {
		str := fmt.Sprintf("got (%T) %v but expect (%T) %v", a.v, a.v, expect, expect)
		fail(a.t, str, msg...)
	}
	return a
}

func (a *Assertion) IsNotSame(expect interface{}, msg ...string) *Assertion {
	a.t.Helper()
	if a.v == expect {
		str := fmt.Sprintf("expect not (%T) %v", expect, expect)
		fail(a.t, str, msg...)
	}
	return a
}

func (a *Assertion) HasPrefix(prefix string, msg ...string) *Assertion {
	a.t.Helper()
	v := a.v.(string)
	if !strings.HasPrefix(v, prefix) {
		fail(a.t, fmt.Sprintf("'%s' doesn't have prefix '%s'", v, prefix), msg...)
	}
	return a
}

type BoolAssertion struct {
	t T
	v bool
}

func ThatBool(t T, v bool) *BoolAssertion {
	return &BoolAssertion{
		t: t,
		v: v,
	}
}

func (a *BoolAssertion) IsTrue(msg ...string) *BoolAssertion {
	a.t.Helper()
	if !a.v {
		fail(a.t, "got false but expect true", msg...)
	}
	return a
}

func (a *BoolAssertion) IsFalse(msg ...string) *BoolAssertion {
	a.t.Helper()
	if a.v {
		fail(a.t, "got true but expect false", msg...)
	}
	return a
}

type StringAssertion struct {
	t T
	v string
}

func ThatString(t T, v string) *StringAssertion {
	return &StringAssertion{
		t: t,
		v: v,
	}
}

func (a *StringAssertion) IsEqualFold(s string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.EqualFold(a.v, s) {
		fail(a.t, fmt.Sprintf("'%s' doesn't equal fold '%s'", a.v, s), msg...)
	}
	return a
}

func (a *StringAssertion) HasPrefix(prefix string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.HasPrefix(a.v, prefix) {
		fail(a.t, fmt.Sprintf("'%s' doesn't have prefix '%s'", a.v, prefix), msg...)
	}
	return a
}

func (a *StringAssertion) HasSuffix(suffix string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.HasSuffix(a.v, suffix) {
		fail(a.t, fmt.Sprintf("'%s' doesn't have suffix '%s'", a.v, suffix), msg...)
	}
	return a
}

func (a *StringAssertion) HasSubString(substr string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.Contains(a.v, substr) {
		fail(a.t, fmt.Sprintf("'%s' doesn't contain substr '%s'", a.v, substr), msg...)
	}
	return a
}
