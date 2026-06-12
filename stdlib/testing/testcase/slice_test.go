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

func TestSlice_Length(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful case
	m.Reset()
	assert.Slice(m, []float64{1.1, 2.2, 3.3}).Length(3)
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.Slice(m, []float64{1.1, 2.2}).Length(3)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to have length 3, but it has length 2
  actual: [1.1,2.2]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []float64{1.1}).Length(0, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to have length 0, but it has length 1
  actual: [1.1]
 message: "index is 0"`)

	// Test empty slice
	m.Reset()
	assert.Slice(m, []int{}).Length(0)
	assert.String(t, m.String()).Equal("")

	// Test nil slice
	m.Reset()
	assert.Slice(m, []int(nil)).Length(0)
	assert.String(t, m.String()).Equal("")

	// Test non-empty slice with wrong expected length
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c", "d"}).Length(2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to have length 2, but it has length 4
  actual: ["a","b","c","d"]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).Length(5, "should have length 5")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to have length 5, but it has length 3
  actual: [1,2,3]
 message: "should have length 5"`)

	// Test Require mode success (no output)
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).Length(3)
	assert.String(t, m.String()).Equal("")
}

func TestSlice_Nil(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful nil check
	m.Reset()
	assert.Slice(m, []int(nil)).Nil()
	assert.String(t, m.String()).Equal("")

	// Test failure case
	m.Reset()
	assert.Slice(m, []int{1, 2}).Nil()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to be nil, but it is not
  actual: [1,2]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2}).Nil("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to be nil, but it is not
  actual: [1,2]
 message: "index is 0"`)

	// Test empty slice (not nil) case
	m.Reset()
	assert.Slice(m, []float64{}).Nil()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to be nil, but it is not
  actual: []`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int(nil)).Nil("should be nil")
	assert.String(t, m.String()).Equal("")

	// Test Require mode success
	m.Reset()
	require.Slice(m, []string(nil)).Nil()
	assert.String(t, m.String()).Equal("")
}

func TestSlice_NotNil(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful non-nil check
	m.Reset()
	assert.Slice(m, []int{1, 2}).NotNil()
	assert.String(t, m.String()).Equal("")

	// Test failure case with nil slice
	m.Reset()
	assert.Slice(m, []int(nil)).NotNil()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to be nil, but it is
  actual: null`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int(nil)).NotNil("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice not to be nil, but it is
  actual: null
 message: "index is 0"`)

	// Test empty slice (non-nil) case
	m.Reset()
	assert.Slice(m, []float64{}).NotNil()
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int(nil)).NotNil("should not be nil")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to be nil, but it is
  actual: null
 message: "should not be nil"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []string{"hello"}).NotNil()
	assert.String(t, m.String()).Equal("")
}

func TestSlice_Empty(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful empty check with empty slice
	m.Reset()
	assert.Slice(m, []int{}).Empty()
	assert.String(t, m.String()).Equal("")

	// Test failure case with non-empty slice
	m.Reset()
	assert.Slice(m, []int{1, 2}).Empty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to be empty, but it is not
  actual: [1,2]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2}).Empty("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to be empty, but it is not
  actual: [1,2]
 message: "index is 0"`)

	// Test nil slice (should also be considered empty)
	m.Reset()
	assert.Slice(m, []string(nil)).Empty()
	assert.String(t, m.String()).Equal("")

	// Test single element slice failure
	m.Reset()
	assert.Slice(m, []bool{true}).Empty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to be empty, but it is not
  actual: [true]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1}).Empty("should be empty")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to be empty, but it is not
  actual: [1]
 message: "should be empty"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []string{}).Empty()
	assert.String(t, m.String()).Equal("")
}

func TestSlice_NotEmpty(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful non-empty check with single element slice
	m.Reset()
	assert.Slice(m, []string{"hello"}).NotEmpty()
	assert.String(t, m.String()).Equal("")

	// Test failure case with empty slice
	m.Reset()
	assert.Slice(m, []string{}).NotEmpty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to be empty, but it is
  actual: []`)

	// Test failure with Require mode and nil slice
	m.Reset()
	require.Slice(m, []string(nil)).NotEmpty("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice not to be empty, but it is
  actual: null
 message: "index is 0"`)

	// Test multi-element slice
	m.Reset()
	assert.Slice(m, []float64{1.1, 2.2, 3.3}).NotEmpty()
	assert.String(t, m.String()).Equal("")

	// Test nil slice failure
	m.Reset()
	assert.Slice(m, []int(nil)).NotEmpty()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to be empty, but it is
  actual: null`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []string{}).NotEmpty("should not be empty")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to be empty, but it is
  actual: []
 message: "should not be empty"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2}).NotEmpty()
	assert.String(t, m.String()).Equal("")
}

func TestSlice_Equal(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful equality check
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).Equal([]int{1, 2, 3})
	assert.String(t, m.String()).Equal("")

	// Test failure case with different lengths
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).Equal([]int{4, 5})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be equal, but their lengths are different
  actual: [1,2,3]
expected: [4,5]`)

	// Test failure with Require mode and different values
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).Equal([]int{1, 2, 4}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slices to be equal, but values at index 2 are different
  actual: [1,2,3]
expected: [1,2,4]
 message: "index is 0"`)

	// Test empty slices equality
	m.Reset()
	assert.Slice(m, []string{}).Equal([]string{})
	assert.String(t, m.String()).Equal("")

	// Test nil slice equals empty slice
	m.Reset()
	assert.Slice(m, []int(nil)).Equal([]int{})
	assert.String(t, m.String()).Equal("")

	// Test length difference where actual is longer
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).Equal([]int{1, 2, 3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be equal, but their lengths are different
  actual: [1,2,3,4]
expected: [1,2,3]`)

	// Test first element different
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).Equal([]int{0, 2, 3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be equal, but values at index 0 are different
  actual: [1,2,3]
expected: [0,2,3]`)

	// Test middle element different
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).Equal([]string{"a", "x", "c"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be equal, but values at index 1 are different
  actual: ["a","b","c"]
expected: ["a","x","c"]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2}).Equal([]int{1, 3}, "should be equal")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be equal, but values at index 1 are different
  actual: [1,2]
expected: [1,3]
 message: "should be equal"`)

	// Test Require mode length failure
	m.Reset()
	require.Slice(m, []int{1, 2}).Equal([]int{1, 2, 3})
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slices to be equal, but their lengths are different
  actual: [1,2]
expected: [1,2,3]`)
}

func TestSlice_NotEqual(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful not-equal check
	m.Reset()
	assert.Slice(m, []string{"a", "b"}).NotEqual([]string{"c", "d"})
	assert.String(t, m.String()).Equal("")

	// Test failure case with equal slices
	m.Reset()
	assert.Slice(m, []string{"a", "b"}).NotEqual([]string{"a", "b"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be different, but they are equal
  actual: ["a","b"]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []string{"a", "b"}).NotEqual([]string{"a", "b"}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slices to be different, but they are equal
  actual: ["a","b"]
 message: "index is 0"`)

	// Test empty slice vs non-empty slice
	m.Reset()
	assert.Slice(m, []int{}).NotEqual([]int{1})
	assert.String(t, m.String()).Equal("")

	// Test nil slice vs non-empty slice
	m.Reset()
	assert.Slice(m, []int(nil)).NotEqual([]int{1})
	assert.String(t, m.String()).Equal("")

	// Test different types of slices with different values
	m.Reset()
	assert.Slice(m, []float64{1.1, 2.2}).NotEqual([]float64{1.1, 3.3})
	assert.String(t, m.String()).Equal("")

	// Test equal single element slices
	m.Reset()
	assert.Slice(m, []int{42}).NotEqual([]int{42})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be different, but they are equal
  actual: [42]`)

	// Test slices with same length but different last element
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).NotEqual([]string{"a", "b", "d"})
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).NotEqual([]int{1, 2, 3}, "should be different")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slices to be different, but they are equal
  actual: [1,2,3]
 message: "should be different"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []bool{true}).NotEqual([]bool{false})
	assert.String(t, m.String()).Equal("")
}

func TestSlice_Contains(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful containment check for middle element
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).Contains(2)
	assert.String(t, m.String()).Equal("")

	// Test failure case with missing element
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).Contains(4)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain element 4, but it is missing
  actual: [1,2,3]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).Contains(4, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to contain element 4, but it is missing
  actual: [1,2,3]
 message: "index is 0"`)

	// Test empty slice
	m.Reset()
	assert.Slice(m, []int{}).Contains(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain element 1, but it is missing
  actual: []`)

	// Test nil slice
	m.Reset()
	assert.Slice(m, []string(nil)).Contains("a")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain element "a", but it is missing
  actual: null`)

	// Test single element slice containing that element
	m.Reset()
	assert.Slice(m, []float64{3.14}).Contains(3.14)
	assert.String(t, m.String()).Equal("")

	// Test slice with duplicate elements
	m.Reset()
	assert.Slice(m, []int{1, 2, 2, 3}).Contains(2)
	assert.String(t, m.String()).Equal("")

	// Test first element match
	m.Reset()
	assert.Slice(m, []int{42, 1, 2}).Contains(42)
	assert.String(t, m.String()).Equal("")

	// Test last element match
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).Contains("c")
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2}).Contains(3, "should contain 3")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain element 3, but it is missing
  actual: [1,2]
 message: "should contain 3"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).Contains(2)
	assert.String(t, m.String()).Equal("")
}

func TestSlice_NotContains(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful not-containment check
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).NotContains(4)
	assert.String(t, m.String()).Equal("")

	// Test failure case with existing element
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).NotContains(2)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to contain element 2, but it is found
  actual: [1,2,3]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).NotContains(2, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice not to contain element 2, but it is found
  actual: [1,2,3]
 message: "index is 0"`)

	// Test empty slice
	m.Reset()
	assert.Slice(m, []int{}).NotContains(1)
	assert.String(t, m.String()).Equal("")

	// Test nil slice
	m.Reset()
	assert.Slice(m, []string(nil)).NotContains("a")
	assert.String(t, m.String()).Equal("")

	// Test single element slice not containing another element
	m.Reset()
	assert.Slice(m, []float64{3.14}).NotContains(2.71)
	assert.String(t, m.String()).Equal("")

	// Test single element slice containing that element (failure case)
	m.Reset()
	assert.Slice(m, []bool{true}).NotContains(true)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to contain element true, but it is found
  actual: [true]`)

	// Test slice with duplicate elements not containing unexisting element
	m.Reset()
	assert.Slice(m, []int{1, 2, 2, 3}).NotContains(4)
	assert.String(t, m.String()).Equal("")

	// Test last element not contained
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).NotContains("d")
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2}).NotContains(2, "should not contain 2")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to contain element 2, but it is found
  actual: [1,2]
 message: "should not contain 2"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).NotContains(4)
	assert.String(t, m.String()).Equal("")
}

func TestSlice_ContainsSlice(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful containment of sub-slice in middle
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).ContainsSlice([]int{2, 3})
	assert.String(t, m.String()).Equal("")

	// Test successful containment of nil sub-slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).ContainsSlice(nil)
	assert.String(t, m.String()).Equal("")

	// Test failure case with non-contained sub-slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).ContainsSlice([]int{2, 4})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain sub-slice, but it is not
  actual: [1,2,3,4]
     sub: [2,4]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 3, 4}).ContainsSlice([]int{2, 4}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to contain sub-slice, but it is not
  actual: [1,2,3,4]
     sub: [2,4]
 message: "index is 0"`)

	// Test empty slice contains empty sub-slice
	m.Reset()
	assert.Slice(m, []int{}).ContainsSlice([]int{})
	assert.String(t, m.String()).Equal("")

	// Test complete match
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).ContainsSlice([]string{"a", "b", "c"})
	assert.String(t, m.String()).Equal("")

	// Test sub-slice longer than main slice
	m.Reset()
	assert.Slice(m, []int{1, 2}).ContainsSlice([]int{1, 2, 3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain sub-slice, but it is not
  actual: [1,2]
     sub: [1,2,3]`)

	// Test empty slice not containing non-empty sub-slice
	m.Reset()
	assert.Slice(m, []int{}).ContainsSlice([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain sub-slice, but it is not
  actual: []
     sub: [1]`)

	// Test single element match
	m.Reset()
	assert.Slice(m, []bool{true, false, true}).ContainsSlice([]bool{false})
	assert.String(t, m.String()).Equal("")

	// Test elements exist but in different order
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).ContainsSlice([]int{3, 2})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain sub-slice, but it is not
  actual: [1,2,3,4]
     sub: [3,2]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []string{"a", "b"}).ContainsSlice([]string{"c"}, "should contain c")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to contain sub-slice, but it is not
  actual: ["a","b"]
     sub: ["c"]
 message: "should contain c"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).ContainsSlice([]int{2, 3})
	assert.String(t, m.String()).Equal("")
}

func TestSlice_NotContainsSlice(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful not-containment of sub-slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).NotContainsSlice([]int{2, 4})
	assert.String(t, m.String()).Equal("")

	// Test successful not-containment of nil sub-slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).NotContainsSlice(nil)
	assert.String(t, m.String()).Equal("")

	// Test failure case with contained sub-slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).NotContainsSlice([]int{2, 3})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to contain sub-slice, but it is
  actual: [1,2,3,4]
     sub: [2,3]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 3, 4}).NotContainsSlice([]int{2, 3}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice not to contain sub-slice, but it is
  actual: [1,2,3,4]
     sub: [2,3]
 message: "index is 0"`)

	// Test empty slice not containing non-empty sub-slice
	m.Reset()
	assert.Slice(m, []int{}).NotContainsSlice([]int{1})
	assert.String(t, m.String()).Equal("")

	// Test complete mismatch
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).NotContainsSlice([]string{"d", "e", "f"})
	assert.String(t, m.String()).Equal("")

	// Test single element mismatch
	m.Reset()
	assert.Slice(m, []bool{true, false, true}).NotContainsSlice([]bool{false, true, false})
	assert.String(t, m.String()).Equal("")

	// Test sub-slice longer than main slice
	m.Reset()
	assert.Slice(m, []int{1, 2}).NotContainsSlice([]int{1, 2, 3})
	assert.String(t, m.String()).Equal("")

	// Test elements exist but in different order
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 4}).NotContainsSlice([]int{3, 2})
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).NotContainsSlice([]string{"b", "c"}, "should not contain bc")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice not to contain sub-slice, but it is
  actual: ["a","b","c"]
     sub: ["b","c"]
 message: "should not contain bc"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).NotContainsSlice([]int{2, 4})
	assert.String(t, m.String()).Equal("")
}

func TestSlice_HasPrefix(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful prefix check
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasPrefix([]int{1, 2})
	assert.String(t, m.String()).Equal("")

	// Test failure case with prefix longer than slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasPrefix([]int{1, 2, 3, 4})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to start with prefix, but it is not
  actual: [1,2,3]
  prefix: [1,2,3,4]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).HasPrefix([]int{2, 3}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to start with prefix, but it is not
  actual: [1,2,3]
  prefix: [2,3]
 message: "index is 0"`)

	// Test empty prefix
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasPrefix([]int{})
	assert.String(t, m.String()).Equal("")

	// Test complete match
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).HasPrefix([]string{"a", "b", "c"})
	assert.String(t, m.String()).Equal("")

	// Test empty slice with non-empty prefix
	m.Reset()
	assert.Slice(m, []int{}).HasPrefix([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to start with prefix, but it is not
  actual: []
  prefix: [1]`)

	// Test nil slice with non-empty prefix
	m.Reset()
	assert.Slice(m, []int(nil)).HasPrefix([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to start with prefix, but it is not
  actual: null
  prefix: [1]`)

	// Test first element mismatch
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasPrefix([]int{2, 1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to start with prefix, but it is not
  actual: [1,2,3]
  prefix: [2,1]`)

	// Test middle element mismatch
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).HasPrefix([]string{"a", "x", "c"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to start with prefix, but it is not
  actual: ["a","b","c"]
  prefix: ["a","x","c"]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2}).HasPrefix([]int{2, 3}, "should start with 2,3")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to start with prefix, but it is not
  actual: [1,2]
  prefix: [2,3]
 message: "should start with 2,3"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).HasPrefix([]int{1, 2})
	assert.String(t, m.String()).Equal("")
}

func TestSlice_HasSuffix(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful suffix check
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasSuffix([]int{2, 3})
	assert.String(t, m.String()).Equal("")

	// Test failure case with suffix longer than slice
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasSuffix([]int{1, 2, 3, 4})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to end with suffix, but it is not
  actual: [1,2,3]
  suffix: [1,2,3,4]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).HasSuffix([]int{1, 2}, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected slice to end with suffix, but it is not
  actual: [1,2,3]
  suffix: [1,2]
 message: "index is 0"`)

	// Test empty suffix
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasSuffix([]int{})
	assert.String(t, m.String()).Equal("")

	// Test complete match
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).HasSuffix([]string{"a", "b", "c"})
	assert.String(t, m.String()).Equal("")

	// Test empty slice with non-empty suffix
	m.Reset()
	assert.Slice(m, []int{}).HasSuffix([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to end with suffix, but it is not
  actual: []
  suffix: [1]`)

	// Test nil slice with non-empty suffix
	m.Reset()
	assert.Slice(m, []int(nil)).HasSuffix([]int{1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to end with suffix, but it is not
  actual: null
  suffix: [1]`)

	// Test last element mismatch
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).HasSuffix([]int{2, 1})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to end with suffix, but it is not
  actual: [1,2,3]
  suffix: [2,1]`)

	// Test middle element mismatch
	m.Reset()
	assert.Slice(m, []string{"a", "b", "c"}).HasSuffix([]string{"x", "c"})
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to end with suffix, but it is not
  actual: ["a","b","c"]
  suffix: ["x","c"]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2}).HasSuffix([]int{1, 3}, "should end with 1,3")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected slice to end with suffix, but it is not
  actual: [1,2]
  suffix: [1,3]
 message: "should end with 1,3"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 2, 3}).HasSuffix([]int{2, 3})
	assert.String(t, m.String()).Equal("")
}

func TestSlice_AllUnique(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful unique elements check
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).AllUnique()
	assert.String(t, m.String()).Equal("")

	// Test failure case with duplicate elements
	m.Reset()
	assert.Slice(m, []int{1, 2, 1}).AllUnique()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to be unique, but duplicate element 1 is found
  actual: [1,2,1]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 2, 1}).AllUnique("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected all elements in the slice to be unique, but duplicate element 1 is found
  actual: [1,2,1]
 message: "index is 0"`)

	// Test empty slice
	m.Reset()
	assert.Slice(m, []int{}).AllUnique()
	assert.String(t, m.String()).Equal("")

	// Test nil slice
	m.Reset()
	assert.Slice(m, []int(nil)).AllUnique()
	assert.String(t, m.String()).Equal("")

	// Test single element slice
	m.Reset()
	assert.Slice(m, []string{"hello"}).AllUnique()
	assert.String(t, m.String()).Equal("")

	// Test two identical elements
	m.Reset()
	assert.Slice(m, []int{5, 5}).AllUnique()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to be unique, but duplicate element 5 is found
  actual: [5,5]`)

	// Test first and last elements are the same
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 1}).AllUnique()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to be unique, but duplicate element 1 is found
  actual: [1,2,3,1]`)

	// Test boolean type with duplicates
	m.Reset()
	assert.Slice(m, []bool{true, false, true}).AllUnique()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to be unique, but duplicate element true is found
  actual: [true,false,true]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2, 1}).AllUnique("all elements should be unique")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to be unique, but duplicate element 1 is found
  actual: [1,2,1]
 message: "all elements should be unique"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []string{"a", "b", "c"}).AllUnique()
	assert.String(t, m.String()).Equal("")
}

func TestSlice_AllMatches(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful condition check for all elements
	m.Reset()
	assert.Slice(m, []int{2, 4, 6, 8}).AllMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal("")

	// Test failure case with one element not satisfying condition
	m.Reset()
	assert.Slice(m, []int{2, 3, 4, 6}).AllMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to satisfy the condition, but element 3 does not
  actual: [2,3,4,6]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{2, 3, 4, 6}).AllMatches(func(n int) bool { return n%2 == 0 }, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected all elements in the slice to satisfy the condition, but element 3 does not
  actual: [2,3,4,6]
 message: "index is 0"`)

	// Test empty slice
	m.Reset()
	assert.Slice(m, []int{}).AllMatches(func(n int) bool { return n > 0 })
	assert.String(t, m.String()).Equal("")

	// Test nil slice
	m.Reset()
	assert.Slice(m, []int(nil)).AllMatches(func(n int) bool { return n > 0 })
	assert.String(t, m.String()).Equal("")

	// Test single element satisfying condition
	m.Reset()
	assert.Slice(m, []string{"hello"}).AllMatches(func(s string) bool { return len(s) > 3 })
	assert.String(t, m.String()).Equal("")

	// Test single element not satisfying condition
	m.Reset()
	assert.Slice(m, []int{5}).AllMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to satisfy the condition, but element 5 does not
  actual: [5]`)

	// Test first element not satisfying condition
	m.Reset()
	assert.Slice(m, []int{1, 2, 4, 6}).AllMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to satisfy the condition, but element 1 does not
  actual: [1,2,4,6]`)

	// Test last element not satisfying condition
	m.Reset()
	assert.Slice(m, []int{2, 4, 6, 7}).AllMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to satisfy the condition, but element 7 does not
  actual: [2,4,6,7]`)

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).AllMatches(func(n int) bool { return n%2 == 0 }, "all elements should be even")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected all elements in the slice to satisfy the condition, but element 1 does not
  actual: [1,2,3]
 message: "all elements should be even"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{2, 4, 6}).AllMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal("")
}

func TestSlice_AnyMatches(t *testing.T) {
	m := new(internal.MockTestingT)

	// Test successful condition check for at least one element
	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 5}).AnyMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal("")

	// Test failure case with no elements satisfying condition
	m.Reset()
	assert.Slice(m, []int{1, 3, 5, 7}).AnyMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected at least one element in the slice to satisfy the condition, but none do
  actual: [1,3,5,7]`)

	// Test failure with Require mode
	m.Reset()
	require.Slice(m, []int{1, 3, 5, 7}).AnyMatches(func(n int) bool { return n%2 == 0 }, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected at least one element in the slice to satisfy the condition, but none do
  actual: [1,3,5,7]
 message: "index is 0"`)

	// Test empty slice
	m.Reset()
	assert.Slice(m, []int{}).AnyMatches(func(n int) bool { return n > 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected at least one element in the slice to satisfy the condition, but none do
  actual: []`)

	// Test nil slice
	m.Reset()
	assert.Slice(m, []int(nil)).AnyMatches(func(n int) bool { return n > 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected at least one element in the slice to satisfy the condition, but none do
  actual: null`)

	// Test single element satisfying condition
	m.Reset()
	assert.Slice(m, []string{"hello"}).AnyMatches(func(s string) bool { return len(s) > 3 })
	assert.String(t, m.String()).Equal("")

	// Test single element not satisfying condition
	m.Reset()
	assert.Slice(m, []int{5}).AnyMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected at least one element in the slice to satisfy the condition, but none do
  actual: [5]`)

	// Test last element satisfying condition
	m.Reset()
	assert.Slice(m, []int{1, 3, 5, 8}).AnyMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 3, 5}).AnyMatches(func(n int) bool { return n%2 == 0 }, "should have at least one even number")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected at least one element in the slice to satisfy the condition, but none do
  actual: [1,3,5]
 message: "should have at least one even number"`)

	// Test Require mode success
	m.Reset()
	require.Slice(m, []int{1, 3, 4}).AnyMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal("")
}

func TestSlice_NoneMatches(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Slice(m, []int{1, 3, 5, 7}).NoneMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Slice(m, []int{1, 2, 3, 5}).NoneMatches(func(n int) bool { return n%2 == 0 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected no element in the slice to satisfy the condition, but element 2 does
  actual: [1,2,3,5]`)

	m.Reset()
	require.Slice(m, []int{1, 2, 3, 5}).NoneMatches(func(n int) bool { return n%2 == 0 }, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected no element in the slice to satisfy the condition, but element 2 does
  actual: [1,2,3,5]
 message: "index is 0"`)

	// Test empty slice case
	m.Reset()
	assert.Slice(m, []int{}).NoneMatches(func(n int) bool { return n > 0 })
	assert.String(t, m.String()).Equal("")

	// Test nil slice case
	m.Reset()
	assert.Slice(m, []int(nil)).NoneMatches(func(n int) bool { return n > 0 })
	assert.String(t, m.String()).Equal("")

	// Test single element that satisfies condition (failure case)
	m.Reset()
	assert.Slice(m, []string{"hello"}).NoneMatches(func(s string) bool { return len(s) > 3 })
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected no element in the slice to satisfy the condition, but element "hello" does
  actual: ["hello"]`)

	// Test all elements do not satisfy condition
	m.Reset()
	assert.Slice(m, []float64{1.1, 3.3, 5.5}).NoneMatches(func(f float64) bool { return f > 10 })
	assert.String(t, m.String()).Equal("")

	// Test with custom message
	m.Reset()
	assert.Slice(m, []int{1, 2, 3}).NoneMatches(func(n int) bool { return n%2 == 0 }, "should not contain even numbers")
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected no element in the slice to satisfy the condition, but element 2 does
  actual: [1,2,3]
 message: "should not contain even numbers"`)

	// Test complex type
	m.Reset()
	assert.Slice(m, []struct{ A, B int }{{1, 3}, {3, 5}}).NoneMatches(func(s struct{ A, B int }) bool { return s.A%2 == 0 })
	assert.String(t, m.String()).Equal("")
}
