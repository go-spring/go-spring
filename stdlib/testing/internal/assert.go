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

package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// TestingT is the minimum interface of *testing.T.
// It provides basic methods for reporting test errors or failures.
type TestingT interface {
	Helper()
	Error(args ...any)
	Fatal(args ...any)
}

// MockTestingT simulates *testing.T for testing purposes.
// It records output in a buffer for verification during tests.
type MockTestingT struct {
	buf bytes.Buffer
}

func (m *MockTestingT) Helper() {}

// Error writes error messages to the internal buffer.
func (m *MockTestingT) Error(args ...any) {
	m.buf.WriteString("error# ")
	for _, arg := range args {
		_, _ = fmt.Fprint(&m.buf, fmt.Sprint(arg))
	}
}

// Fatal writes fatal messages to the internal buffer.
func (m *MockTestingT) Fatal(args ...any) {
	m.buf.WriteString("fatal# ")
	for _, arg := range args {
		_, _ = fmt.Fprint(&m.buf, fmt.Sprint(arg))
	}
}

// Reset clears the internal buffer.
func (m *MockTestingT) Reset() {
	m.buf.Reset()
}

// String returns the current content of the buffer.
func (m *MockTestingT) String() string {
	return m.buf.String()
}

// Fail reports an assertion failure using the provided TestingT.
// If fatalOnFailure is true, it calls `t.Fatal`; otherwise, it calls `t.Error`.
func Fail(t TestingT, fatalOnFailure bool, str string, msg ...string) {
	t.Helper()
	if len(msg) > 0 {
		str += fmt.Sprintf("\n message: %q", strings.Join(msg, ", "))
	}
	if fatalOnFailure {
		t.Fatal("Assertion failed: " + str)
	} else {
		t.Error("Assertion failed: " + str)
	}
}

// recovery executes the given function and recovers from any panic.
// Returns the recovered value as a string if a panic occurs.
func recovery(fn func()) (str string) {
	defer func() {
		if r := recover(); r != nil {
			str = fmt.Sprint(r)
		}
	}()
	fn()
	return "<<SUCCESS>>"
}

// Panic asserts that fn panics and the panic message matches expr.
// It reports an error if fn does not panic or if the recovered message does not satisfy expr.
func Panic(t TestingT, fatalOnFailure bool, fn func(), expr string, msg ...string) {
	t.Helper()
	if got := recovery(fn); got == "<<SUCCESS>>" {
		Fail(t, fatalOnFailure, "did not panic", msg...)
	} else {
		if ok, err := regexp.MatchString(expr, got); err != nil {
			Fail(t, fatalOnFailure, "invalid pattern", msg...)
		} else if !ok {
			str := fmt.Sprintf("got %q which does not match %q", got, expr)
			Fail(t, fatalOnFailure, str, msg...)
		}
	}
}

// AssertionBase provides common functionality for `Assertion`.
type AssertionBase struct {
	t TestingT

	fatalOnFailure bool
}

// ToJSONString converts the given value to a JSON string.
func ToJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

// ToPrettyString converts the given value to a pretty string.
func ToPrettyString(v any) string {
	fv := reflect.ValueOf(v)
	if v == nil || isNil(fv) {
		return "nil"
	}

	switch fv.Kind() {
	case reflect.Func:
		return fmt.Sprintf("(%v)", v)
	default: // for linter
	}

	s := fmt.Sprintf("%#v", v)
	s = strings.TrimLeft(s, "&")
	s = strings.TrimLeft(s, "*")
	if strings.HasPrefix(s, "(") {
		s = s[strings.Index(s, ")")+1:]
	}
	s = strings.TrimSpace(s)

	typ := reflect.TypeOf(v).String()
	typ = strings.TrimLeft(typ, "*")
	s, _ = strings.CutPrefix(s, typ)
	return s
}

// Assertion wraps a test context and a value for fluent assertions.
type Assertion struct {
	AssertionBase
	v any
}

// That creates an Assertion for the given value v and test context t.
func That(t TestingT, v any, fatalOnFailure bool) *Assertion {
	return &Assertion{
		AssertionBase: AssertionBase{
			t:              t,
			fatalOnFailure: fatalOnFailure,
		},
		v: v,
	}
}

// True asserts that got is true. It reports an error if the value is false.
func (a *Assertion) True(msg ...string) *Assertion {
	a.t.Helper()
	if b, _ := a.v.(bool); !b {
		str := `expected value to be true, but it is false`
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// False asserts that got is false. It reports an error if the value is true.
func (a *Assertion) False(msg ...string) *Assertion {
	a.t.Helper()
	if b, _ := a.v.(bool); b {
		str := `expected value to be false, but it is true`
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

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

// Nil asserts that got is nil. It reports an error if the value is not nil.
func (a *Assertion) Nil(msg ...string) *Assertion {
	a.t.Helper()
	// Why can't we use got==nil to judge？Because if
	// a := (*int)(nil) // %T == *int
	// b := (any)(nil)  // %T == <nil>
	// then a==b is false, because they are different types.
	if !isNil(reflect.ValueOf(a.v)) {
		str := fmt.Sprintf(`expected value to be nil, but it is not
  actual: (%T) %s`, a.v, ToPrettyString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotNil asserts that got is not nil. It reports an error if the value is nil.
func (a *Assertion) NotNil(msg ...string) *Assertion {
	a.t.Helper()
	if isNil(reflect.ValueOf(a.v)) {
		str := `expected value to be non-nil, but it is nil`
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Equal asserts that the wrapped value v is `reflect.DeepEqual` to expect.
// It reports an error if the values are not deeply equal.
func (a *Assertion) Equal(expect any, msg ...string) *Assertion {
	a.t.Helper()
	if !reflect.DeepEqual(a.v, expect) {
		str := fmt.Sprintf(`expected values to be equal, but they are different
  actual: (%T) %s
expected: (%T) %s`, a.v, ToPrettyString(a.v), expect, ToPrettyString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotEqual asserts that the wrapped value v is not deeply equal to expect.
// It reports an error if the values are deeply equal.
func (a *Assertion) NotEqual(expect any, msg ...string) *Assertion {
	a.t.Helper()
	if reflect.DeepEqual(a.v, expect) {
		str := fmt.Sprintf(`expected values to be different, but they are equal
  actual: (%T) %s`, a.v, ToPrettyString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Same asserts that the wrapped value v and expect are the same (using Go ==).
// It reports an error if v != expect.
func (a *Assertion) Same(expect any, msg ...string) *Assertion {
	a.t.Helper()
	if a.v != expect {
		str := fmt.Sprintf(`expected values to be same, but they are different
  actual: (%T) %s
expected: (%T) %s`, a.v, ToPrettyString(a.v), expect, ToPrettyString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotSame asserts that the wrapped value v and expect are not the same (using Go !=).
// It reports an error if v == expect.
func (a *Assertion) NotSame(expect any, msg ...string) *Assertion {
	a.t.Helper()
	if a.v == expect {
		str := fmt.Sprintf(`expected values to be different, but they are same
  actual: (%T) %s`, a.v, ToPrettyString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// TypeOf asserts that the type of the wrapped value v is assignable to the type of expect.
// It supports pointer to interface types.
// It reports an error if the types are not assignable.
func (a *Assertion) TypeOf(expect any, msg ...string) *Assertion {
	a.t.Helper()

	e1 := reflect.TypeOf(a.v)
	e2 := reflect.TypeOf(expect)
	if e2.Kind() == reflect.Pointer && e2.Elem().Kind() == reflect.Interface {
		e2 = e2.Elem()
	}

	if !e1.AssignableTo(e2) {
		str := fmt.Sprintf(`expected type to be assignable to target, but it does not
  actual: %s
expected: %s`, e1.String(), e2.String())
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Implements asserts that the type of the wrapped value v implements the interface type of expect.
// The expect parameter must be an interface or pointer to interface.
// It reports an error if v does not implement the interface.
func (a *Assertion) Implements(expect any, msg ...string) *Assertion {
	a.t.Helper()

	e1 := reflect.TypeOf(a.v)
	e2 := reflect.TypeOf(expect)
	if e2.Kind() == reflect.Pointer {
		if e2.Elem().Kind() == reflect.Interface {
			e2 = e2.Elem()
		} else {
			Fail(a.t, a.fatalOnFailure, "expected target to implement should be interface", msg...)
			return a
		}
	}

	if !e1.Implements(e2) {
		str := fmt.Sprintf(`expected type to implement target interface, but it does not
  actual: %s
expected: %s`, e1.String(), e2.String())
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Has asserts that the wrapped value v has a method named 'Has' that returns true when passed expect.
// It reports an error if the method does not exist or returns false.
func (a *Assertion) Has(expect any, msg ...string) *Assertion {
	a.t.Helper()

	if isNil(reflect.ValueOf(a.v)) {
		str := `method 'Has' not found on type <nil>`
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}

	m := reflect.ValueOf(a.v).MethodByName("Has")
	if !m.IsValid() {
		str := fmt.Sprintf("method 'Has' not found on type %T", a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}

	if m.Type().NumOut() != 1 || m.Type().Out(0).Kind() != reflect.Bool {
		str := fmt.Sprintf("method 'Has' on type %T should return only a bool, but it does not", a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}

	ret := m.Call([]reflect.Value{reflect.ValueOf(expect)})
	if !ret[0].Bool() {
		str := fmt.Sprintf(`method 'Has' on type %T should return true when using param %s, but it does not`, a.v, ToPrettyString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Contains asserts that the wrapped value v has a method named 'Contains' that returns true when passed expect.
// It reports an error if the method does not exist or returns false.
func (a *Assertion) Contains(expect any, msg ...string) *Assertion {
	a.t.Helper()

	if isNil(reflect.ValueOf(a.v)) {
		str := `method 'Contains' not found on type <nil>`
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}

	m := reflect.ValueOf(a.v).MethodByName("Contains")
	if !m.IsValid() {
		str := fmt.Sprintf("method 'Contains' not found on type %T", a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}

	if m.Type().NumOut() != 1 || m.Type().Out(0).Kind() != reflect.Bool {
		str := fmt.Sprintf("method 'Contains' on type %T should return only a bool, but it does not", a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}

	ret := m.Call([]reflect.Value{reflect.ValueOf(expect)})
	if !ret[0].Bool() {
		str := fmt.Sprintf(`method 'Contains' on type %T should return true when using param %s, but it does not`, a.v, ToPrettyString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}
