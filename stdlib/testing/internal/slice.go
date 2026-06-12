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
	"fmt"
	"slices"
)

// SliceAssertion encapsulates a slice value and a test handler for making assertions on the slice.
type SliceAssertion[T comparable] struct {
	AssertionBase
	v []T
}

// ThatSlice returns a SliceAssertion for the given testing object and slice value.
func ThatSlice[T comparable](t TestingT, v []T, fatalOnFailure bool) *SliceAssertion[T] {
	return &SliceAssertion[T]{
		AssertionBase: AssertionBase{
			t:              t,
			fatalOnFailure: fatalOnFailure,
		},
		v: v,
	}
}

// Length asserts that the slice has the expected length.
func (a *SliceAssertion[T]) Length(length int, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(a.v) != length {
		str := fmt.Sprintf(`expected slice to have length %d, but it has length %d
  actual: %v`, length, len(a.v), ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Nil asserts that the slice is nil.
func (a *SliceAssertion[T]) Nil(msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if a.v != nil {
		str := fmt.Sprintf(`expected slice to be nil, but it is not
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotNil asserts that the slice is not nil.
func (a *SliceAssertion[T]) NotNil(msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if a.v == nil {
		str := fmt.Sprintf(`expected slice not to be nil, but it is
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Empty asserts that the slice is empty.
func (a *SliceAssertion[T]) Empty(msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(a.v) != 0 {
		str := fmt.Sprintf(`expected slice to be empty, but it is not
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotEmpty asserts that the slice is not empty.
func (a *SliceAssertion[T]) NotEmpty(msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(a.v) == 0 {
		str := fmt.Sprintf(`expected slice not to be empty, but it is
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Equal asserts that the slice is equal to the expected slice.
func (a *SliceAssertion[T]) Equal(expect []T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(a.v) != len(expect) {
		str := fmt.Sprintf(`expected slices to be equal, but their lengths are different
  actual: %v
expected: %v`, ToJSONString(a.v), ToJSONString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	for i := range a.v {
		if a.v[i] != expect[i] {
			str := fmt.Sprintf(`expected slices to be equal, but values at index %d are different
  actual: %v
expected: %v`, i, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// NotEqual asserts that the slice is not equal to the expected slice.
func (a *SliceAssertion[T]) NotEqual(expect []T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(a.v) == len(expect) {
		equal := true
		for i := range a.v {
			if a.v[i] != expect[i] {
				equal = false
				break
			}
		}
		if equal {
			str := fmt.Sprintf(`expected slices to be different, but they are equal
  actual: %v`, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
		}
	}
	return a
}

// Contains asserts that the slice contains the expected element.
func (a *SliceAssertion[T]) Contains(element T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if slices.Contains(a.v, element) {
		return a
	}
	str := fmt.Sprintf(`expected slice to contain element %s, but it is missing
  actual: %v`, ToPrettyString(element), ToJSONString(a.v))
	Fail(a.t, a.fatalOnFailure, str, msg...)
	return a
}

// NotContains asserts that the slice does not contain the expected element.
func (a *SliceAssertion[T]) NotContains(element T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if slices.Contains(a.v, element) {
		str := fmt.Sprintf(`expected slice not to contain element %+v, but it is found
  actual: %v`, element, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	return a
}

// ContainsSlice asserts that the slice contains the expected sub-slice.
func (a *SliceAssertion[T]) ContainsSlice(sub []T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(sub) == 0 {
		return a
	}
	for i := 0; i <= len(a.v)-len(sub); i++ {
		match := true
		for j := range len(sub) {
			if a.v[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return a
		}
	}
	str := fmt.Sprintf(`expected slice to contain sub-slice, but it is not
  actual: %v
     sub: %v`, ToJSONString(a.v), ToJSONString(sub))
	Fail(a.t, a.fatalOnFailure, str, msg...)
	return a
}

// NotContainsSlice asserts that the slice does not contain the expected sub-slice.
func (a *SliceAssertion[T]) NotContainsSlice(sub []T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(sub) == 0 {
		return a
	}
	for i := 0; i <= len(a.v)-len(sub); i++ {
		match := true
		for j := range len(sub) {
			if a.v[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			str := fmt.Sprintf(`expected slice not to contain sub-slice, but it is
  actual: %v
     sub: %v`, ToJSONString(a.v), ToJSONString(sub))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// HasPrefix asserts that the slice starts with the specified prefix.
func (a *SliceAssertion[T]) HasPrefix(prefix []T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(prefix) > len(a.v) {
		str := fmt.Sprintf(`expected slice to start with prefix, but it is not
  actual: %v
  prefix: %v`, ToJSONString(a.v), ToJSONString(prefix))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	for i := range prefix {
		if a.v[i] != prefix[i] {
			str := fmt.Sprintf(`expected slice to start with prefix, but it is not
  actual: %v
  prefix: %v`, ToJSONString(a.v), ToJSONString(prefix))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// HasSuffix asserts that the slice ends with the specified suffix.
func (a *SliceAssertion[T]) HasSuffix(suffix []T, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if len(suffix) > len(a.v) {
		str := fmt.Sprintf(`expected slice to end with suffix, but it is not
  actual: %v
  suffix: %v`, ToJSONString(a.v), ToJSONString(suffix))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	offset := len(a.v) - len(suffix)
	for i := range suffix {
		if a.v[offset+i] != suffix[i] {
			str := fmt.Sprintf(`expected slice to end with suffix, but it is not
  actual: %v
  suffix: %v`, ToJSONString(a.v), ToJSONString(suffix))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// AllUnique asserts that all elements in the slice are unique.
func (a *SliceAssertion[T]) AllUnique(msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	seen := make(map[T]bool)
	for _, v := range a.v {
		if seen[v] {
			str := fmt.Sprintf(`expected all elements in the slice to be unique, but duplicate element %+v is found
  actual: %v`, v, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
		seen[v] = true
	}
	return a
}

// AllMatches asserts that all elements in the slice satisfy the given condition.
func (a *SliceAssertion[T]) AllMatches(fn func(T) bool, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	for _, v := range a.v {
		if !fn(v) {
			str := fmt.Sprintf(`expected all elements in the slice to satisfy the condition, but element %s does not
  actual: %v`, ToPrettyString(v), ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// AnyMatches asserts that at least one element in the slice satisfies the given condition.
func (a *SliceAssertion[T]) AnyMatches(fn func(T) bool, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	if slices.ContainsFunc(a.v, fn) {
		return a
	}
	str := fmt.Sprintf(`expected at least one element in the slice to satisfy the condition, but none do
  actual: %v`, ToJSONString(a.v))
	Fail(a.t, a.fatalOnFailure, str, msg...)
	return a
}

// NoneMatches asserts that no element in the slice satisfies the given condition.
func (a *SliceAssertion[T]) NoneMatches(fn func(T) bool, msg ...string) *SliceAssertion[T] {
	a.t.Helper()
	for _, v := range a.v {
		if fn(v) {
			str := fmt.Sprintf(`expected no element in the slice to satisfy the condition, but element %s does
  actual: %v`, ToPrettyString(v), ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}
