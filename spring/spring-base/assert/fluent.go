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

func (a *Assertion) IsTrue(msg ...string) *Assertion {
	a.t.Helper()
	v := a.v.(bool)
	if !v {
		fail(a.t, "got false but expect true", msg...)
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

func (a *StringAssertion) EqualFold(s string, msg ...string) *StringAssertion {
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

func (a *StringAssertion) Contains(substr string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.Contains(a.v, substr) {
		fail(a.t, fmt.Sprintf("'%s' doesn't contain substr '%s'", a.v, substr), msg...)
	}
	return a
}
