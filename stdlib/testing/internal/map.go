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
)

// MapAssertion encapsulates a map value and a test handler for making assertions on the map.
type MapAssertion[K, V comparable] struct {
	AssertionBase
	v map[K]V
}

// ThatMap returns a MapAssertion for the given testing object and map value.
func ThatMap[K, V comparable](t TestingT, v map[K]V, fatalOnFailure bool) *MapAssertion[K, V] {
	return &MapAssertion[K, V]{
		AssertionBase: AssertionBase{
			t:              t,
			fatalOnFailure: fatalOnFailure,
		},
		v: v,
	}
}

// Length asserts that the map has the expected length.
func (a *MapAssertion[K, V]) Length(length int, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) != length {
		str := fmt.Sprintf(`expected map to have length %d, but it has length %d
  actual: %v`, length, len(a.v), ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Nil asserts that the map is nil.
func (a *MapAssertion[K, V]) Nil(msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if a.v != nil {
		str := fmt.Sprintf(`expected map to be nil, but it is not
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotNil asserts that the map is not nil.
func (a *MapAssertion[K, V]) NotNil(msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if a.v == nil {
		str := fmt.Sprintf(`expected map not to be nil, but it is
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Empty asserts that the map is empty.
func (a *MapAssertion[K, V]) Empty(msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) != 0 {
		str := fmt.Sprintf(`expected map to be empty, but it is not
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotEmpty asserts that the map is not empty.
func (a *MapAssertion[K, V]) NotEmpty(msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) == 0 {
		str := fmt.Sprintf(`expected map to be non-empty, but it is empty
  actual: %v`, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Equal asserts that the map is equal to the expected map.
func (a *MapAssertion[K, V]) Equal(expect map[K]V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) != len(expect) {
		str := fmt.Sprintf(`expected maps to be equal, but their lengths are different
  actual: %v
expected: %v`, ToJSONString(a.v), ToJSONString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	for k, v := range a.v {
		if expectV, ok := expect[k]; !ok {
			str := fmt.Sprintf(`expected maps to be equal, but key '%v' is missing
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		} else if v != expectV {
			str := fmt.Sprintf(`expected maps to be equal, but values for key '%v' are different
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// NotEqual asserts that the map is not equal to the expected map.
func (a *MapAssertion[K, V]) NotEqual(expect map[K]V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) == len(expect) {
		equal := true
		for k, v := range a.v {
			if expectV, ok := expect[k]; !ok || v != expectV {
				equal = false
				break
			}
		}
		if equal {
			str := fmt.Sprintf(`expected maps to be different, but they are equal
  actual: %v`, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
		}
	}
	return a
}

// ContainsKey asserts that the map contains the expected key.
func (a *MapAssertion[K, V]) ContainsKey(key K, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if _, ok := a.v[key]; !ok {
		str := fmt.Sprintf(`expected map to contain key '%v', but it is missing
  actual: %v`, key, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotContainsKey asserts that the map does not contain the expected key.
func (a *MapAssertion[K, V]) NotContainsKey(key K, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if _, ok := a.v[key]; ok {
		str := fmt.Sprintf(`expected map not to contain key '%v', but it is found
  actual: %v`, key, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// ContainsValue asserts that the map contains the expected value.
func (a *MapAssertion[K, V]) ContainsValue(value V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for _, v := range a.v {
		if v == value {
			return a
		}
	}
	str := fmt.Sprintf(`expected map to contain value %+v, but it is missing
  actual: %v`, value, ToJSONString(a.v))
	Fail(a.t, a.fatalOnFailure, str, msg...)
	return a
}

// NotContainsValue asserts that the map does not contain the expected value.
func (a *MapAssertion[K, V]) NotContainsValue(value V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for _, v := range a.v {
		if v == value {
			str := fmt.Sprintf(`expected map not to contain value %+v, but it is found
  actual: %v`, value, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// ContainsKeyValue asserts that the map contains the expected key-value pair.
func (a *MapAssertion[K, V]) ContainsKeyValue(key K, value V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if v, ok := a.v[key]; !ok {
		str := fmt.Sprintf(`expected map to contain key '%v', but it is missing
  actual: %v`, key, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	} else if v != value {
		str := fmt.Sprintf(`expected value %+v for key '%v', but got %+v instead
  actual: %v`, value, key, v, ToJSONString(a.v))
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// ContainsKeys asserts that the map contains all the expected keys.
func (a *MapAssertion[K, V]) ContainsKeys(keys []K, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for _, key := range keys {
		if _, ok := a.v[key]; !ok {
			str := fmt.Sprintf(`expected map to contain key '%v', but it is missing
  actual: %v`, key, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// NotContainsKeys asserts that the map does not contain any of the expected keys.
func (a *MapAssertion[K, V]) NotContainsKeys(keys []K, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for _, key := range keys {
		if _, ok := a.v[key]; ok {
			str := fmt.Sprintf(`expected map not to contain key '%v', but it is found
  actual: %v`, key, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// ContainsValues asserts that the map contains all the expected values.
func (a *MapAssertion[K, V]) ContainsValues(values []V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for _, value := range values {
		found := false
		for _, v := range a.v {
			if v == value {
				found = true
				break
			}
		}
		if !found {
			str := fmt.Sprintf(`expected map to contain value %+v, but it is missing
  actual: %v`, value, ToJSONString(a.v))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// NotContainsValues asserts that the map does not contain any of the expected values.
func (a *MapAssertion[K, V]) NotContainsValues(values []V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for _, value := range values {
		for _, v := range a.v {
			if v == value {
				str := fmt.Sprintf(`expected map not to contain value %+v, but it is found
  actual: %v`, v, ToJSONString(a.v))
				Fail(a.t, a.fatalOnFailure, str, msg...)
				return a
			}
		}
	}
	return a
}

// SubsetOf asserts that the map is a subset of the expected map.
func (a *MapAssertion[K, V]) SubsetOf(expect map[K]V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for k, v := range a.v {
		if expectV, ok := expect[k]; !ok {
			str := fmt.Sprintf(`expected map to be a subset, but unexpected key '%v' is found
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		} else if v != expectV {
			str := fmt.Sprintf(`expected map to be a subset, but values for key '%v' are different
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// SupersetOf asserts that the map is a superset of the expected map.
func (a *MapAssertion[K, V]) SupersetOf(expect map[K]V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	for k, v := range expect {
		if aV, ok := a.v[k]; !ok {
			str := fmt.Sprintf(`expected map to be a superset, but key '%v' is missing
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		} else if aV != v {
			str := fmt.Sprintf(`expected map to be a superset, but values for key '%v' are different
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// HasSameKeys asserts that the map has the same keys as the expected map.
func (a *MapAssertion[K, V]) HasSameKeys(expect map[K]V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) != len(expect) {
		str := fmt.Sprintf(`expected maps to have the same keys, but their lengths are different
  actual: %v
expected: %v`, ToJSONString(a.v), ToJSONString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	for k := range a.v {
		if _, ok := expect[k]; !ok {
			str := fmt.Sprintf(`expected maps to have the same keys, but key '%v' is missing
  actual: %v
expected: %v`, k, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}

// HasSameValues asserts that the map has the same values as the expected map.
func (a *MapAssertion[K, V]) HasSameValues(expect map[K]V, msg ...string) *MapAssertion[K, V] {
	a.t.Helper()
	if len(a.v) != len(expect) {
		str := fmt.Sprintf(`expected maps to have the same values, but their lengths are different
  actual: %v
expected: %v`, ToJSONString(a.v), ToJSONString(expect))
		Fail(a.t, a.fatalOnFailure, str, msg...)
		return a
	}
	valueCount := make(map[V]int)
	for _, v := range a.v {
		valueCount[v]++
	}
	for _, v := range expect {
		valueCount[v]--
	}
	for _, count := range valueCount {
		if count != 0 {
			str := fmt.Sprintf(`expected maps to have the same values, but their values are different
  actual: %v
expected: %v`, ToJSONString(a.v), ToJSONString(expect))
			Fail(a.t, a.fatalOnFailure, str, msg...)
			return a
		}
	}
	return a
}
