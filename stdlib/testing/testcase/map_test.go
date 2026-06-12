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

package testcase_test

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/internal"
	"go-spring.org/stdlib/testing/require"
)

func TestMap_Length(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case
	m.Reset()
	assert.Map(m, testMap).Length(1)
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.Map(m, testMap).Length(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to have length 0, but it has length 1
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).Length(0, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to have length 0, but it has length 1
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).Length(0)
	assert.String(t, m.String()).Equal("")

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).Length(0)
	assert.String(t, m.String()).Equal("")

	// Test failure with empty map
	m.Reset()
	assert.Map(m, emptyMap).Length(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to have length 1, but it has length 0
  actual: {}`)

	// Test with custom message
	m.Reset()
	assert.Map(m, testMap).Length(3, "custom message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to have length 3, but it has length 1
  actual: {"a":1}
 message: "custom message"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, testMap).Length(3, "fatal message")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to have length 3, but it has length 1
  actual: {"a":1}
 message: "fatal message"`)
}

func TestMap_Nil(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with nil map
	m.Reset()
	assert.Map(m, map[string]int(nil)).Nil()
	assert.String(t, m.String()).Equal("")

	// Test failure case with non-empty map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).Nil()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be nil, but it is not
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, map[string]int{"a": 1}).Nil("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be nil, but it is not
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map (not nil)
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).Nil()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be nil, but it is not
  actual: {}`)

	// Test with custom message
	m.Reset()
	testMap := map[string]int{"key": 42}
	assert.Map(m, testMap).Nil("custom error message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be nil, but it is not
  actual: {"key":42}
 message: "custom error message"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, testMap).Nil("fatal error")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be nil, but it is not
  actual: {"key":42}
 message: "fatal error"`)
}

func TestMap_NotNil(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with non-empty map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).NotNil()
	assert.String(t, m.String()).Equal("")

	// Test failure case with nil map
	m.Reset()
	assert.Map(m, map[string]int(nil)).NotNil()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to be nil, but it is
  actual: null`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, map[string]int(nil)).NotNil("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to be nil, but it is
  actual: null
 message: "index is 0"`)

	// Test with empty map (not nil)
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).NotNil()
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).NotNil("map should not be nil")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to be nil, but it is
  actual: null
 message: "map should not be nil"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, nilMap).NotNil("required: map must not be nil")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to be nil, but it is
  actual: null
 message: "required: map must not be nil"`)
}

func TestMap_IsEmpty(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with nil map
	m.Reset()
	assert.Map(m, map[string]int(nil)).Empty()
	assert.String(t, m.String()).Equal("")

	// Test failure case with non-empty map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).Empty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be empty, but it is not
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, map[string]int{"a": 1}).Empty("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be empty, but it is not
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map (non-nil)
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).Empty()
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	testMap := map[string]int{"key": 100}
	assert.Map(m, testMap).Empty("map should be empty")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be empty, but it is not
  actual: {"key":100}
 message: "map should be empty"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, testMap).Empty("required: map must be empty")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be empty, but it is not
  actual: {"key":100}
 message: "required: map must be empty"`)
}

func TestMap_IsNotEmpty(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with non-empty map
	testMap := map[string]int{"a": 1}
	assert.Map(m, testMap).NotEmpty()
	assert.String(t, m.String()).Equal("")

	// Test failure case with nil map
	m.Reset()
	assert.Map(m, map[string]int(nil)).NotEmpty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be non-empty, but it is empty
  actual: null`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, map[string]int{}).NotEmpty("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be non-empty, but it is empty
  actual: {}
 message: "index is 0"`)

	// Test with empty non-nil map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).NotEmpty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be non-empty, but it is empty
  actual: {}`)

	// Test with custom message
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).NotEmpty("map should not be empty")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be non-empty, but it is empty
  actual: null
 message: "map should not be empty"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, emptyMap).NotEmpty("required: map must not be empty")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be non-empty, but it is empty
  actual: {}
 message: "required: map must not be empty"`)
}

func TestMap_Equal(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with equal maps
	m.Reset()
	assert.Map(m, testMap).Equal(map[string]int{"a": 1})
	assert.String(t, m.String()).Equal("")

	// Test failure case with nil map
	m.Reset()
	assert.Map(m, testMap).Equal(nil)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be equal, but their lengths are different
  actual: {"a":1}
expected: null`)

	// Test failure case with different keys
	m.Reset()
	assert.Map(m, testMap).Equal(map[string]int{"b": 2})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be equal, but key 'a' is missing
  actual: {"a":1}
expected: {"b":2}`)

	// Test fatal failure with different values
	m.Reset()
	require.Map(m, testMap).Equal(map[string]int{"a": 2}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to be equal, but values for key 'a' are different
  actual: {"a":1}
expected: {"a":2}
 message: "index is 0"`)

	// Test with empty maps
	m.Reset()
	emptyMap1 := map[string]int{}
	emptyMap2 := map[string]int{}
	assert.Map(m, emptyMap1).Equal(emptyMap2)
	assert.String(t, m.String()).Equal("")

	// Test with maps of different lengths
	m.Reset()
	map1 := map[string]int{"a": 1, "b": 2}
	map2 := map[string]int{"a": 1, "b": 2, "c": 3}
	assert.Map(m, map1).Equal(map2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be equal, but their lengths are different
  actual: {"a":1,"b":2}
expected: {"a":1,"b":2,"c":3}`)

	// Test with maps with same keys but different values
	m.Reset()
	map5 := map[string]int{"a": 1, "b": 2}
	map6 := map[string]int{"a": 1, "b": 3}
	assert.Map(m, map5).Equal(map6)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be equal, but values for key 'b' are different
  actual: {"a":1,"b":2}
expected: {"a":1,"b":3}`)

	// Test with nil maps
	m.Reset()
	var nilMap1 map[string]int
	var nilMap2 map[string]int
	assert.Map(m, nilMap1).Equal(nilMap2)
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	map7 := map[string]int{"x": 10}
	map8 := map[string]int{"x": 20}
	assert.Map(m, map7).Equal(map8, "maps should be equal")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be equal, but values for key 'x' are different
  actual: {"x":10}
expected: {"x":20}
 message: "maps should be equal"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, map7).Equal(map8, "required: maps must be equal")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to be equal, but values for key 'x' are different
  actual: {"x":10}
expected: {"x":20}
 message: "required: maps must be equal"`)
}

func TestMap_NotEqual(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with different maps
	m.Reset()
	assert.Map(m, testMap).NotEqual(map[string]int{"b": 2})
	assert.String(t, m.String()).Equal("")

	// Test failure case with equal maps
	m.Reset()
	assert.Map(m, testMap).NotEqual(testMap)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be different, but they are equal
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).NotEqual(testMap, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to be different, but they are equal
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty maps
	m.Reset()
	emptyMap1 := map[string]int{}
	emptyMap2 := map[string]int{}
	assert.Map(m, emptyMap1).NotEqual(emptyMap2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be different, but they are equal
  actual: {}`)

	// Test with maps of different lengths
	m.Reset()
	map1 := map[string]int{"a": 1, "b": 2}
	map2 := map[string]int{"a": 1, "b": 2, "c": 3}
	assert.Map(m, map1).NotEqual(map2)
	assert.String(t, m.String()).Equal("")

	// Test with nil maps
	m.Reset()
	var nilMap1 map[string]int
	var nilMap2 map[string]int
	assert.Map(m, nilMap1).NotEqual(nilMap2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be different, but they are equal
  actual: null`)

	// Test with custom message
	m.Reset()
	map3 := map[string]int{"x": 10, "y": 20}
	map4 := map[string]int{"x": 10, "y": 20}
	assert.Map(m, map3).NotEqual(map4, "maps should be different")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to be different, but they are equal
  actual: {"x":10,"y":20}
 message: "maps should be different"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, map3).NotEqual(map4, "required: maps must be different")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to be different, but they are equal
  actual: {"x":10,"y":20}
 message: "required: maps must be different"`)
}

func TestMap_ContainsKey(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with existing key
	m.Reset()
	assert.Map(m, testMap).ContainsKey("a")
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing key
	m.Reset()
	assert.Map(m, testMap).ContainsKey("b")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'b', but it is missing
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).ContainsKey("b", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain key 'b', but it is missing
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).ContainsKey("a")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'a', but it is missing
  actual: {}`)

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).ContainsKey("a")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'a', but it is missing
  actual: null`)

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).ContainsKey("other", "key should exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'other', but it is missing
  actual: {"item":100}
 message: "key should exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).ContainsKey("other", "required: key must exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain key 'other', but it is missing
  actual: {"item":100}
 message: "required: key must exist"`)
}

func TestMap_NotContainsKey(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with missing key
	m.Reset()
	assert.Map(m, testMap).NotContainsKey("b")
	assert.String(t, m.String()).Equal("")

	// Test failure case with existing key
	m.Reset()
	assert.Map(m, testMap).NotContainsKey("a")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain key 'a', but it is found
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).NotContainsKey("a", "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain key 'a', but it is found
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).NotContainsKey("a")
	assert.String(t, m.String()).Equal("")

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).NotContainsKey("a")
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).NotContainsKey("item", "key should not exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain key 'item', but it is found
  actual: {"item":100}
 message: "key should not exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).NotContainsKey("item", "required: key must not exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain key 'item', but it is found
  actual: {"item":100}
 message: "required: key must not exist"`)
}

func TestMap_ContainsValue(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with existing value
	m.Reset()
	assert.Map(m, testMap).ContainsValue(1)
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing value
	m.Reset()
	assert.Map(m, testMap).ContainsValue(2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 2, but it is missing
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).ContainsValue(2, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain value 2, but it is missing
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).ContainsValue(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 1, but it is missing
  actual: {}`)

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).ContainsValue(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 1, but it is missing
  actual: null`)

	// Test with multiple values (same value)
	m.Reset()
	duplicateValueMap := map[string]int{"a": 1, "b": 2, "c": 1}
	assert.Map(m, duplicateValueMap).ContainsValue(1)
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).ContainsValue(99, "value should exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 99, but it is missing
  actual: {"item":100}
 message: "value should exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).ContainsValue(99, "required: value must exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain value 99, but it is missing
  actual: {"item":100}
 message: "required: value must exist"`)
}

func TestMap_NotContainsValue(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with missing value
	m.Reset()
	assert.Map(m, testMap).NotContainsValue(2)
	assert.String(t, m.String()).Equal("")

	// Test failure case with existing value
	m.Reset()
	assert.Map(m, testMap).NotContainsValue(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain value 1, but it is found
  actual: {"a":1}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).NotContainsValue(1, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain value 1, but it is found
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).NotContainsValue(1)
	assert.String(t, m.String()).Equal("")

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).NotContainsValue(1)
	assert.String(t, m.String()).Equal("")

	// Test with multiple values containing the value
	m.Reset()
	duplicateValueMap := map[string]int{"a": 1, "b": 2, "c": 1}
	assert.Map(m, duplicateValueMap).NotContainsValue(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain value 1, but it is found
  actual: {"a":1,"b":2,"c":1}`)

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).NotContainsValue(100, "value should not exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain value 100, but it is found
  actual: {"item":100}
 message: "value should not exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).NotContainsValue(100, "required: value must not exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain value 100, but it is found
  actual: {"item":100}
 message: "required: value must not exist"`)
}

func TestMap_ContainsKeyValue(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1}

	// Test successful case with existing key-value pair
	m.Reset()
	assert.Map(m, testMap).ContainsKeyValue("a", 1)
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing key
	m.Reset()
	assert.Map(m, testMap).ContainsKeyValue("b", 2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'b', but it is missing
  actual: {"a":1}`)

	// Test fatal failure with wrong value
	m.Reset()
	require.Map(m, testMap).ContainsKeyValue("a", 2, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected value 2 for key 'a', but got 1 instead
  actual: {"a":1}
 message: "index is 0"`)

	// Test with empty map
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).ContainsKeyValue("a", 1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'a', but it is missing
  actual: {}`)

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).ContainsKeyValue("a", 1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'a', but it is missing
  actual: null`)

	// Test with custom message for missing key
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).ContainsKeyValue("other", 200, "key should exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'other', but it is missing
  actual: {"item":100}
 message: "key should exist"`)

	// Test with custom message for wrong value
	m.Reset()
	assert.Map(m, singleItemMap).ContainsKeyValue("item", 200, "value should match")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected value 200 for key 'item', but got 100 instead
  actual: {"item":100}
 message: "value should match"`)

	// Test fatal failure with custom message for missing key
	m.Reset()
	require.Map(m, singleItemMap).ContainsKeyValue("other", 200, "required: key must exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain key 'other', but it is missing
  actual: {"item":100}
 message: "required: key must exist"`)

	// Test fatal failure with custom message for wrong value
	m.Reset()
	require.Map(m, singleItemMap).ContainsKeyValue("item", 200, "required: value must match")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected value 200 for key 'item', but got 100 instead
  actual: {"item":100}
 message: "required: value must match"`)
}

func TestMap_ContainsKeys(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1, "b": 2}

	// Test successful case with existing keys
	m.Reset()
	assert.Map(m, testMap).ContainsKeys([]string{"a", "b"})
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing key
	m.Reset()
	assert.Map(m, testMap).ContainsKeys([]string{"c"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'c', but it is missing
  actual: {"a":1,"b":2}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).ContainsKeys([]string{"c"}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain key 'c', but it is missing
  actual: {"a":1,"b":2}
 message: "index is 0"`)

	// Test with empty keys slice
	m.Reset()
	assert.Map(m, testMap).ContainsKeys([]string{})
	assert.String(t, m.String()).Equal("")

	// Test with empty map and non-empty keys slice
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).ContainsKeys([]string{"a"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'a', but it is missing
  actual: {}`)

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).ContainsKeys([]string{"a"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'a', but it is missing
  actual: null`)

	// Test with duplicate keys in expected slice
	m.Reset()
	assert.Map(m, testMap).ContainsKeys([]string{"a", "b", "a"})
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).ContainsKeys([]string{"other"}, "keys should exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain key 'other', but it is missing
  actual: {"item":100}
 message: "keys should exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).ContainsKeys([]string{"other"}, "required: keys must exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain key 'other', but it is missing
  actual: {"item":100}
 message: "required: keys must exist"`)
}

func TestMap_NotContainsKeys(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1, "b": 2}

	// Test successful case with missing keys
	m.Reset()
	assert.Map(m, testMap).NotContainsKeys([]string{"c"})
	assert.String(t, m.String()).Equal("")

	// Test failure case with existing key
	m.Reset()
	assert.Map(m, testMap).NotContainsKeys([]string{"a"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain key 'a', but it is found
  actual: {"a":1,"b":2}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).NotContainsKeys([]string{"a"}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain key 'a', but it is found
  actual: {"a":1,"b":2}
 message: "index is 0"`)

	// Test with empty keys slice
	m.Reset()
	assert.Map(m, testMap).NotContainsKeys([]string{})
	assert.String(t, m.String()).Equal("")

	// Test with empty map and non-empty keys slice
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).NotContainsKeys([]string{"a"})
	assert.String(t, m.String()).Equal("")

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).NotContainsKeys([]string{"a"})
	assert.String(t, m.String()).Equal("")

	// Test with duplicate keys in expected slice
	m.Reset()
	assert.Map(m, testMap).NotContainsKeys([]string{"c", "d", "c"})
	assert.String(t, m.String()).Equal("")

	// Test with all keys present
	m.Reset()
	assert.Map(m, testMap).NotContainsKeys([]string{"a", "b"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain key 'a', but it is found
  actual: {"a":1,"b":2}`)

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).NotContainsKeys([]string{"item"}, "key should not exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain key 'item', but it is found
  actual: {"item":100}
 message: "key should not exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).NotContainsKeys([]string{"item"}, "required: key must not exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain key 'item', but it is found
  actual: {"item":100}
 message: "required: key must not exist"`)
}

func TestMap_ContainsValues(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1, "b": 2}

	// Test successful case with existing values
	m.Reset()
	assert.Map(m, testMap).ContainsValues([]int{1, 2})
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing value
	m.Reset()
	assert.Map(m, testMap).ContainsValues([]int{3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 3, but it is missing
  actual: {"a":1,"b":2}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).ContainsValues([]int{3}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain value 3, but it is missing
  actual: {"a":1,"b":2}
 message: "index is 0"`)

	// Test with empty values slice
	m.Reset()
	assert.Map(m, testMap).ContainsValues([]int{})
	assert.String(t, m.String()).Equal("")

	// Test with empty map and non-empty values slice
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).ContainsValues([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 1, but it is missing
  actual: {}`)

	// Test with nil map
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).ContainsValues([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 1, but it is missing
  actual: null`)

	// Test with duplicate values in expected slice
	m.Reset()
	assert.Map(m, testMap).ContainsValues([]int{1, 2, 1})
	assert.String(t, m.String()).Equal("")

	// Test with repeated values in map
	m.Reset()
	repeatedValuesMap := map[string]int{"a": 1, "b": 2, "c": 1}
	assert.Map(m, repeatedValuesMap).ContainsValues([]int{1, 2})
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	singleItemMap := map[string]int{"item": 100}
	assert.Map(m, singleItemMap).ContainsValues([]int{99}, "value should exist")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to contain value 99, but it is missing
  actual: {"item":100}
 message: "value should exist"`)

	// Test fatal failure with custom message
	m.Reset()
	require.Map(m, singleItemMap).ContainsValues([]int{99}, "required: value must exist")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to contain value 99, but it is missing
  actual: {"item":100}
 message: "required: value must exist"`)
}

func TestMap_NotContainsValues(t *testing.T) {
	m := new(internal.MockTestingT)
	testMap := map[string]int{"a": 1, "b": 2}

	// Test successful case with missing values
	m.Reset()
	assert.Map(m, testMap).NotContainsValues([]int{3})
	assert.String(t, m.String()).Equal("")

	// Test failure case with existing value
	m.Reset()
	assert.Map(m, testMap).NotContainsValues([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain value 1, but it is found
  actual: {"a":1,"b":2}`)

	// Test fatal failure with message
	m.Reset()
	require.Map(m, testMap).NotContainsValues([]int{1}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain value 1, but it is found
  actual: {"a":1,"b":2}
 message: "index is 0"`)

	// Test with multiple values where some are in the map
	m.Reset()
	assert.Map(m, testMap).NotContainsValues([]int{3, 1, 5})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain value 1, but it is found
  actual: {"a":1,"b":2}`)

	// Test with empty values slice
	m.Reset()
	assert.Map(m, testMap).NotContainsValues([]int{})
	assert.String(t, m.String()).Equal("")

	// Test with empty map
	emptyMap := map[string]int{}
	m.Reset()
	assert.Map(m, emptyMap).NotContainsValues([]int{1, 2, 3})
	assert.String(t, m.String()).Equal("")

	// Test with nil map
	var nilMap map[string]int
	m.Reset()
	assert.Map(m, nilMap).NotContainsValues([]int{1})
	assert.String(t, m.String()).Equal("")

	// Test with custom message and multiple values
	m.Reset()
	assert.Map(m, testMap).NotContainsValues([]int{2}, "value 2 should not be in map")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map not to contain value 2, but it is found
  actual: {"a":1,"b":2}
 message: "value 2 should not be in map"`)

	// Test fatal failure with multiple values
	m.Reset()
	require.Map(m, testMap).NotContainsValues([]int{2, 4}, "fatal: value 2 should not be in map")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map not to contain value 2, but it is found
  actual: {"a":1,"b":2}
 message: "fatal: value 2 should not be in map"`)
}

func TestMap_SubsetOf(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with subset map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).SubsetOf(map[string]int{"a": 1, "b": 2})
	assert.String(t, m.String()).Equal("")

	// Test failure case with unexpected key
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2}).SubsetOf(map[string]int{"a": 1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be a subset, but unexpected key 'b' is found
  actual: {"a":1,"b":2}
expected: {"a":1}`)

	// Test fatal failure with different value
	m.Reset()
	require.Map(m, map[string]int{"a": 1}).SubsetOf(map[string]int{"a": 2}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be a subset, but values for key 'a' are different
  actual: {"a":1}
expected: {"a":2}
 message: "index is 0"`)

	// Test with empty maps
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).SubsetOf(map[string]int{})
	assert.String(t, m.String()).Equal("")

	// Test with empty actual map and non-empty expected map
	m.Reset()
	assert.Map(m, emptyMap).SubsetOf(map[string]int{"a": 1})
	assert.String(t, m.String()).Equal("")

	// Test with key present but different value
	m.Reset()
	assert.Map(m, map[string]int{"a": 3}).SubsetOf(map[string]int{"a": 1, "b": 2})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be a subset, but values for key 'a' are different
  actual: {"a":3}
expected: {"a":1,"b":2}`)

	// Test with multiple keys not in expected map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "d": 4, "e": 5}).SubsetOf(map[string]int{"a": 1, "b": 2})
	assert.String(t, m.String()).Matches(`error# Assertion failed: expected map to be a subset, but unexpected key '[d,e]' is found
  actual: {"a":1,"d":4,"e":5}
expected: {"a":1,"b":2}`)

	// Test with nil maps
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).SubsetOf(nilMap)
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).SubsetOf(map[string]int{"b": 2}, "custom message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be a subset, but unexpected key 'a' is found
  actual: {"a":1}
expected: {"b":2}
 message: "custom message"`)
}

func TestMap_SupersetOf(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with superset map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2}).SupersetOf(map[string]int{"a": 1})
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing key
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).SupersetOf(map[string]int{"a": 1, "b": 2})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be a superset, but key 'b' is missing
  actual: {"a":1}
expected: {"a":1,"b":2}`)

	// Test fatal failure with different value
	m.Reset()
	require.Map(m, map[string]int{"a": 1}).SupersetOf(map[string]int{"a": 2}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected map to be a superset, but values for key 'a' are different
  actual: {"a":1}
expected: {"a":2}
 message: "index is 0"`)

	// Test with empty maps
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).SupersetOf(emptyMap)
	assert.String(t, m.String()).Equal("")

	// Test with empty expected map and non-empty actual map
	m.Reset()
	testMap := map[string]int{"a": 1, "b": 2}
	assert.Map(m, testMap).SupersetOf(emptyMap)
	assert.String(t, m.String()).Equal("")

	// Test with key missing from actual map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).SupersetOf(map[string]int{"b": 2})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be a superset, but key 'b' is missing
  actual: {"a":1}
expected: {"b":2}`)

	// Test with nil maps
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).SupersetOf(nilMap)
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).SupersetOf(map[string]int{"b": 2}, "custom message")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected map to be a superset, but key 'b' is missing
  actual: {"a":1}
expected: {"b":2}
 message: "custom message"`)
}

func TestMap_HasSameKeys(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with same keys
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2}).HasSameKeys(map[string]int{"b": 3, "a": 4})
	assert.String(t, m.String()).Equal("")

	// Test failure case with different lengths
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2}).HasSameKeys(map[string]int{"c": 3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same keys, but their lengths are different
  actual: {"a":1,"b":2}
expected: {"c":3}`)

	// Test fatal failure with missing key
	m.Reset()
	require.Map(m, map[string]int{"a": 1, "b": 2}).HasSameKeys(map[string]int{"b": 2, "c": 3}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to have the same keys, but key 'a' is missing
  actual: {"a":1,"b":2}
expected: {"b":2,"c":3}
 message: "index is 0"`)

	// Test with empty maps
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).HasSameKeys(emptyMap)
	assert.String(t, m.String()).Equal("")

	// Test with maps having same keys in different order
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2, "c": 3}).HasSameKeys(map[string]int{"c": 30, "a": 10, "b": 20})
	assert.String(t, m.String()).Equal("")

	// Test with different length - actual map larger
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2, "c": 3}).HasSameKeys(map[string]int{"a": 10, "b": 20})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same keys, but their lengths are different
  actual: {"a":1,"b":2,"c":3}
expected: {"a":10,"b":20}`)

	// Test with one key missing from actual map
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).HasSameKeys(map[string]int{"a": 10, "b": 20})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same keys, but their lengths are different
  actual: {"a":1}
expected: {"a":10,"b":20}`)

	// Test with nil maps
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).HasSameKeys(nilMap)
	assert.String(t, m.String()).Equal("")

	// Test with custom message - length mismatch
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).HasSameKeys(map[string]int{"a": 1, "b": 2}, "length mismatch")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same keys, but their lengths are different
  actual: {"a":1}
expected: {"a":1,"b":2}
 message: "length mismatch"`)

	// Test with custom message - key missing
	m.Reset()
	require.Map(m, map[string]int{"a": 1, "c": 3}).HasSameKeys(map[string]int{"a": 10, "b": 20}, "key missing")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to have the same keys, but key 'c' is missing
  actual: {"a":1,"c":3}
expected: {"a":10,"b":20}
 message: "key missing"`)
}

func TestMap_HasSameValues(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case with same values
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2}).HasSameValues(map[string]int{"x": 1, "y": 2})
	assert.String(t, m.String()).Equal("")

	// Test failure case with different lengths
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 2}).HasSameValues(map[string]int{"c": 3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same values, but their lengths are different
  actual: {"a":1,"b":2}
expected: {"c":3}`)

	// Test fatal failure with different values
	m.Reset()
	require.Map(m, map[string]int{"a": 1, "b": 2}).HasSameValues(map[string]int{"b": 2, "c": 3}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to have the same values, but their values are different
  actual: {"a":1,"b":2}
expected: {"b":2,"c":3}
 message: "index is 0"`)

	// Test with empty maps
	m.Reset()
	emptyMap := map[string]int{}
	assert.Map(m, emptyMap).HasSameValues(emptyMap)
	assert.String(t, m.String()).Equal("")

	// Test with duplicate values in maps
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 1, "c": 2}).HasSameValues(map[string]int{"x": 2, "y": 1, "z": 1})
	assert.String(t, m.String()).Equal("")

	// Test with different number of duplicate values
	m.Reset()
	assert.Map(m, map[string]int{"a": 1, "b": 1}).HasSameValues(map[string]int{"x": 1, "y": 1, "z": 1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same values, but their lengths are different
  actual: {"a":1,"b":1}
expected: {"x":1,"y":1,"z":1}`)

	// Test with nil maps
	m.Reset()
	var nilMap map[string]int
	assert.Map(m, nilMap).HasSameValues(nilMap)
	assert.String(t, m.String()).Equal("")

	// Test with custom message - length mismatch
	m.Reset()
	assert.Map(m, map[string]int{"a": 1}).HasSameValues(map[string]int{"a": 1, "b": 2}, "length mismatch")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same values, but their lengths are different
  actual: {"a":1}
expected: {"a":1,"b":2}
 message: "length mismatch"`)

	// Test with custom message - values different
	m.Reset()
	require.Map(m, map[string]int{"a": 1, "b": 2}).HasSameValues(map[string]int{"c": 3, "d": 4}, "values mismatch")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected maps to have the same values, but their values are different
  actual: {"a":1,"b":2}
expected: {"c":3,"d":4}
 message: "values mismatch"`)

	// Test with single value maps - not matching
	m.Reset()
	assert.Map(m, map[string]int{"a": 42}).HasSameValues(map[string]int{"x": 24})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected maps to have the same values, but their values are different
  actual: {"a":42}
expected: {"x":24}`)
}
