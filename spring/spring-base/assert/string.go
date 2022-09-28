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

// StringAssertion assertion for type string.
type StringAssertion struct {
	t T
	v string
}

// String returns an assertion for type string.
func String(t T, v string) *StringAssertion {
	return &StringAssertion{
		t: t,
		v: v,
	}
}

// IsEqualFold assertion failed when v doesn't equal to `s` under Unicode case-folding.
func (a *StringAssertion) IsEqualFold(s string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.EqualFold(a.v, s) {
		fail(a.t, fmt.Sprintf("'%s' doesn't equal fold to '%s'", a.v, s), msg...)
	}
	return a
}

// HasPrefix assertion failed when v doesn't have prefix `prefix`.
func (a *StringAssertion) HasPrefix(prefix string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.HasPrefix(a.v, prefix) {
		fail(a.t, fmt.Sprintf("'%s' doesn't have prefix '%s'", a.v, prefix), msg...)
	}
	return a
}

// HasSuffix assertion failed when v doesn't have suffix `suffix`.
func (a *StringAssertion) HasSuffix(suffix string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.HasSuffix(a.v, suffix) {
		fail(a.t, fmt.Sprintf("'%s' doesn't have suffix '%s'", a.v, suffix), msg...)
	}
	return a
}

// HasSubString assertion failed when v doesn't contain substring `substr`.
func (a *StringAssertion) HasSubString(substr string, msg ...string) *StringAssertion {
	a.t.Helper()
	if !strings.Contains(a.v, substr) {
		fail(a.t, fmt.Sprintf("'%s' doesn't contain substr '%s'", a.v, substr), msg...)
	}
	return a
}
