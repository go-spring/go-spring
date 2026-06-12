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
	"math"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
	"github.com/go-spring/stdlib/testing/internal"
	"github.com/go-spring/stdlib/testing/require"
)

func TestNumber_Equal(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).Equal(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).Equal(10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be equal to 10, but it is 5`)

	m.Reset()
	require.Number(m, 5).Equal(10, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be equal to 10, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one success and one failure case for each type
	m.Reset()
	assert.Number(m, int8(5)).Equal(int8(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).Equal(int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be equal to 10, but it is 5`)

	m.Reset()
	assert.Number(m, int16(100)).Equal(int16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).Equal(int32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).Equal(int64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).Equal(uint(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(255)).Equal(uint8(255))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).Equal(uint16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).Equal(uint32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).Equal(uint64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).Equal(float32(3.14))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).Equal(float64(2.71828))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(1.1)).Equal(float64(2.2))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be equal to 2.2, but it is 1.1`)

	// Test with zero values
	m.Reset()
	assert.Number(m, 0).Equal(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(0)).Equal(int64(0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.0)).Equal(float32(0.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(0.0)).Equal(float64(0.0))
	assert.String(t, m.String()).Equal("")
}

func TestNumber_NotEqual(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).NotEqual(10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).NotEqual(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be equal to 5, but it is`)

	m.Reset()
	require.Number(m, 5).NotEqual(5, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number not to be equal to 5, but it is
 message: "index is 0"`)

	// Test with different numeric types - one success and one failure case for each type
	m.Reset()
	assert.Number(m, int8(5)).NotEqual(int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).NotEqual(int8(5))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be equal to 5, but it is`)

	m.Reset()
	assert.Number(m, int16(100)).NotEqual(int16(200))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).NotEqual(int32(2000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).NotEqual(int64(20000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).NotEqual(uint(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(255)).NotEqual(uint8(254))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).NotEqual(uint16(200))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).NotEqual(uint32(2000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).NotEqual(uint64(20000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).NotEqual(float32(2.71))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).NotEqual(float64(3.14159))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(1.1)).NotEqual(float64(1.1))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be equal to 1.1, but it is`)

	// Test with zero values
	m.Reset()
	assert.Number(m, 0).NotEqual(1)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(0)).NotEqual(int64(1))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.0)).NotEqual(float32(1.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(0.0)).NotEqual(float64(1.0))
	assert.String(t, m.String()).Equal("")

	// Test failure with zero values
	m.Reset()
	assert.Number(m, 0).NotEqual(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be equal to 0, but it is`)

	m.Reset()
	assert.Number(m, float64(0.0)).NotEqual(float64(0.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be equal to 0, but it is`)
}

func TestNumber_GreaterThan(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 10).GreaterThan(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).GreaterThan(10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 10, but it is 5`)

	m.Reset()
	require.Number(m, 5).GreaterThan(10, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be greater than 10, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one success and one failure case for each type
	m.Reset()
	assert.Number(m, int8(10)).GreaterThan(int8(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).GreaterThan(int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 10, but it is 5`)

	m.Reset()
	assert.Number(m, int16(100)).GreaterThan(int16(50))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).GreaterThan(int32(500))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).GreaterThan(int64(5000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(10)).GreaterThan(uint(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).GreaterThan(uint16(50))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).GreaterThan(uint32(500))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).GreaterThan(uint64(5000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).GreaterThan(float32(2.71))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).GreaterThan(float64(1.414))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(2.71)).GreaterThan(float32(3.14))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 3.14, but it is 2.71`)

	m.Reset()
	assert.Number(m, float64(1.414)).GreaterThan(float64(2.71828))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 2.71828, but it is 1.414`)

	// Test with equal values - should fail
	m.Reset()
	assert.Number(m, 5).GreaterThan(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 5, but it is 5`)

	m.Reset()
	assert.Number(m, float32(1.0)).GreaterThan(float32(1.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 1, but it is 1`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, -5).GreaterThan(-10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -10).GreaterThan(-5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than -5, but it is -10`)

	// Test with zero values
	m.Reset()
	assert.Number(m, 5).GreaterThan(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).GreaterThan(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 5, but it is 0`)

	m.Reset()
	assert.Number(m, -5).GreaterThan(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than 0, but it is -5`)
}

func TestNumber_GreaterOrEqual(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 10).GreaterOrEqual(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).GreaterOrEqual(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).GreaterOrEqual(10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to 10, but it is 5`)

	m.Reset()
	require.Number(m, 5).GreaterOrEqual(10, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be greater than or equal to 10, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one success case (greater), one (equal) and one failure case for each type
	m.Reset()
	assert.Number(m, int8(10)).GreaterOrEqual(int8(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).GreaterOrEqual(int8(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).GreaterOrEqual(int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to 10, but it is 5`)

	m.Reset()
	assert.Number(m, int16(100)).GreaterOrEqual(int16(50))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).GreaterOrEqual(int32(500))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).GreaterOrEqual(int64(5000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(10)).GreaterOrEqual(uint(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).GreaterOrEqual(uint16(50))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).GreaterOrEqual(uint32(500))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).GreaterOrEqual(uint64(5000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).GreaterOrEqual(float32(2.71))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).GreaterOrEqual(float64(1.414))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).GreaterOrEqual(float32(3.14))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(2.71)).GreaterOrEqual(float32(3.14))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to 3.14, but it is 2.71`)

	m.Reset()
	assert.Number(m, float64(1.414)).GreaterOrEqual(float64(2.71828))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to 2.71828, but it is 1.414`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, -5).GreaterOrEqual(-10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -5).GreaterOrEqual(-5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -10).GreaterOrEqual(-5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to -5, but it is -10`)

	// Test with zero values
	m.Reset()
	assert.Number(m, 5).GreaterOrEqual(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).GreaterOrEqual(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).GreaterOrEqual(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to 5, but it is 0`)

	m.Reset()
	assert.Number(m, -5).GreaterOrEqual(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be greater than or equal to 0, but it is -5`)
}

func TestNumber_LessThan(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).LessThan(10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 10).LessThan(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 5, but it is 10`)

	m.Reset()
	require.Number(m, 10).LessThan(5, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be less than 5, but it is 10
 message: "index is 0"`)

	// Test with different numeric types - one success and one failure case for each type
	m.Reset()
	assert.Number(m, int8(5)).LessThan(int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(10)).LessThan(int8(5))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 5, but it is 10`)

	m.Reset()
	assert.Number(m, int16(50)).LessThan(int16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(500)).LessThan(int32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(5000)).LessThan(int64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).LessThan(uint(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(50)).LessThan(uint16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(500)).LessThan(uint32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(5000)).LessThan(uint64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(2.71)).LessThan(float32(3.14))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(1.414)).LessThan(float64(2.71828))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).LessThan(float32(2.71))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 2.71, but it is 3.14`)

	m.Reset()
	assert.Number(m, float64(2.71828)).LessThan(float64(1.414))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 1.414, but it is 2.71828`)

	// Test with equal values - should fail
	m.Reset()
	assert.Number(m, 5).LessThan(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 5, but it is 5`)

	m.Reset()
	assert.Number(m, float32(1.0)).LessThan(float32(1.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 1, but it is 1`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, -10).LessThan(-5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -5).LessThan(-10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than -10, but it is -5`)

	// Test with zero values
	m.Reset()
	assert.Number(m, 0).LessThan(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).LessThan(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than 0, but it is 5`)

	m.Reset()
	assert.Number(m, 0).LessThan(-5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than -5, but it is 0`)
}

func TestNumber_LessOrEqual(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).LessOrEqual(10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).LessOrEqual(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 10).LessOrEqual(5)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to 5, but it is 10`)

	m.Reset()
	require.Number(m, 10).LessOrEqual(5, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be less than or equal to 5, but it is 10
 message: "index is 0"`)

	// Test with different numeric types - one success case (less), one (equal) and one failure case for each type
	m.Reset()
	assert.Number(m, int8(5)).LessOrEqual(int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).LessOrEqual(int8(5))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(10)).LessOrEqual(int8(5))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to 5, but it is 10`)

	m.Reset()
	assert.Number(m, int16(50)).LessOrEqual(int16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(500)).LessOrEqual(int32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(5000)).LessOrEqual(int64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).LessOrEqual(uint(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(50)).LessOrEqual(uint16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(500)).LessOrEqual(uint32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(5000)).LessOrEqual(uint64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(2.71)).LessOrEqual(float32(3.14))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(1.414)).LessOrEqual(float64(2.71828))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).LessOrEqual(float32(3.14))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).LessOrEqual(float32(2.71))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to 2.71, but it is 3.14`)

	m.Reset()
	assert.Number(m, float64(2.71828)).LessOrEqual(float64(1.414))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to 1.414, but it is 2.71828`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, -10).LessOrEqual(-5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -5).LessOrEqual(-5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -5).LessOrEqual(-10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to -10, but it is -5`)

	// Test with zero values
	m.Reset()
	assert.Number(m, 0).LessOrEqual(5)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).LessOrEqual(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).LessOrEqual(0)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to 0, but it is 5`)

	m.Reset()
	assert.Number(m, -5).LessOrEqual(-10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be less than or equal to -10, but it is -5`)
}

func TestNumber_Zero(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 0).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 5`)

	m.Reset()
	require.Number(m, 5).Zero("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be zero, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one success and one failure case for each type
	m.Reset()
	assert.Number(m, int8(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 5`)

	m.Reset()
	assert.Number(m, int16(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(0.0)).Zero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(100)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 100`)

	m.Reset()
	assert.Number(m, int32(1000)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 1000`)

	m.Reset()
	assert.Number(m, int64(10000)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 10000`)

	m.Reset()
	assert.Number(m, uint(5)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 5`)

	m.Reset()
	assert.Number(m, uint16(100)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 100`)

	m.Reset()
	assert.Number(m, uint32(1000)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 1000`)

	m.Reset()
	assert.Number(m, uint64(10000)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 10000`)

	m.Reset()
	assert.Number(m, float32(3.14)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 3.14`)

	m.Reset()
	assert.Number(m, float64(2.71828)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is 2.71828`)

	// Test with negative values
	m.Reset()
	assert.Number(m, -5).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is -5`)

	m.Reset()
	assert.Number(m, int32(-100)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is -100`)

	m.Reset()
	assert.Number(m, float64(-1.414)).Zero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be zero, but it is -1.414`)
}

func TestNumber_NotZero(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	require.Number(m, 0).NotZero("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number not to be zero, but it is 0
 message: "index is 0"`)

	// Test with different numeric types - one success and one failure case for each type
	m.Reset()
	assert.Number(m, int8(5)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, int16(100)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(255)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).NotZero()
	assert.String(t, m.String()).Equal("")

	// Test with negative values - success cases
	m.Reset()
	assert.Number(m, int32(-100)).NotZero()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(-1.414)).NotZero()
	assert.String(t, m.String()).Equal("")

	// Test with different numeric types - failure cases
	m.Reset()
	assert.Number(m, int16(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, int32(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, int64(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, uint(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, uint8(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, uint16(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, uint32(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, uint64(0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, float32(0.0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)

	m.Reset()
	assert.Number(m, float64(0.0)).NotZero()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be zero, but it is 0`)
}

func TestNumber_Positive(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -5).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -5`)

	m.Reset()
	require.Number(m, -5).Positive("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be positive, but it is -5
 message: "index is 0"`)

	// Test with different numeric types - one success case, one zero case and one negative case for each type
	m.Reset()
	assert.Number(m, int8(5)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, int8(-5)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -5`)

	m.Reset()
	assert.Number(m, int16(100)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(255)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).Positive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, int32(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, int64(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, uint(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, uint8(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, uint16(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, uint32(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, uint64(0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, float32(0.0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	m.Reset()
	assert.Number(m, float64(0.0)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is 0`)

	// Test with negative values - should fail
	m.Reset()
	assert.Number(m, int32(-100)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -100`)

	m.Reset()
	assert.Number(m, float64(-1.414)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -1.414`)

	m.Reset()
	assert.Number(m, int16(-100)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -100`)

	m.Reset()
	assert.Number(m, int64(-10000)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -10000`)

	m.Reset()
	assert.Number(m, float32(-3.14)).Positive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be positive, but it is -3.14`)
}

func TestNumber_NotPositive(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, -5).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 5`)

	m.Reset()
	require.Number(m, 5).NotPositive("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be non-positive, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one negative case, one zero case, and one positive case for each type
	m.Reset()
	assert.Number(m, int8(-5)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 5`)

	m.Reset()
	assert.Number(m, int16(-100)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(-1000)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(-10000)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(-3.14)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(-2.71828)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(0.0)).NotPositive()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(100)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 100`)

	m.Reset()
	assert.Number(m, int32(1000)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 1000`)

	m.Reset()
	assert.Number(m, int64(10000)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 10000`)

	m.Reset()
	assert.Number(m, uint(5)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 5`)

	m.Reset()
	assert.Number(m, uint8(255)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 255`)

	m.Reset()
	assert.Number(m, uint16(100)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 100`)

	m.Reset()
	assert.Number(m, uint32(1000)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 1000`)

	m.Reset()
	assert.Number(m, uint64(10000)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 10000`)

	m.Reset()
	assert.Number(m, float32(3.14)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 3.14`)

	m.Reset()
	assert.Number(m, float64(2.71828)).NotPositive()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-positive, but it is 2.71828`)
}

func TestNumber_Negative(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, -5).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 5`)

	m.Reset()
	require.Number(m, 5).Negative("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be negative, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one negative case, one zero case, and one positive case for each type
	m.Reset()
	assert.Number(m, int8(-5)).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, int8(5)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 5`)

	m.Reset()
	assert.Number(m, int16(-100)).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(-1000)).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(-10000)).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(-3.14)).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(-2.71828)).Negative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, int32(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, int64(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, uint(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, uint8(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, uint16(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, uint32(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, uint64(0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, float32(0.0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	m.Reset()
	assert.Number(m, float64(0.0)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 0`)

	// Test with positive values - should fail
	m.Reset()
	assert.Number(m, int32(100)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 100`)

	m.Reset()
	assert.Number(m, float64(1.414)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 1.414`)

	m.Reset()
	assert.Number(m, int16(100)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 100`)

	m.Reset()
	assert.Number(m, int64(10000)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 10000`)

	m.Reset()
	assert.Number(m, float32(3.14)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 3.14`)

	// Test with unsigned types - should fail (except for zero which is tested above)
	m.Reset()
	assert.Number(m, uint(5)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 5`)

	m.Reset()
	assert.Number(m, uint8(255)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 255`)

	m.Reset()
	assert.Number(m, uint16(100)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 100`)

	m.Reset()
	assert.Number(m, uint32(1000)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 1000`)

	m.Reset()
	assert.Number(m, uint64(10000)).Negative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be negative, but it is 10000`)
}

func TestNumber_NotNegative(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, -5).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -5`)

	m.Reset()
	require.Number(m, -5).NotNegative("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be non-negative, but it is -5
 message: "index is 0"`)

	// Test with different numeric types - one positive case, one zero case, and one negative case for each type
	m.Reset()
	assert.Number(m, int8(5)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(-5)).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -5`)

	m.Reset()
	assert.Number(m, int16(100)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(255)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(0.0)).NotNegative()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(-100)).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -100`)

	m.Reset()
	assert.Number(m, int32(-1000)).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -1000`)

	m.Reset()
	assert.Number(m, int64(-10000)).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -10000`)

	m.Reset()
	assert.Number(m, float32(-3.14)).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -3.14`)

	m.Reset()
	assert.Number(m, float64(-2.71828)).NotNegative()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be non-negative, but it is -2.71828`)
}

func TestNumber_Between(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5).Between(1, 10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 0).Between(1, 10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 10, but it is 0`)

	m.Reset()
	require.Number(m, 0).Between(1, 10, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be between 1 and 10, but it is 0
 message: "index is 0"`)

	// Test with different numeric types - one success case and one failure case (below lower bound) for each type
	m.Reset()
	assert.Number(m, int8(5)).Between(int8(1), int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(0)).Between(int8(1), int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 10, but it is 0`)

	m.Reset()
	assert.Number(m, int16(50)).Between(int16(10), int16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(500)).Between(int32(100), int32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(5000)).Between(int64(1000), int64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).Between(uint(1), uint(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(50)).Between(uint16(10), uint16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(500)).Between(uint32(100), uint32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(5000)).Between(uint64(1000), uint64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(2.5)).Between(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(1.5)).Between(float64(1.0), float64(2.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.5)).Between(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 3, but it is 0.5`)

	m.Reset()
	assert.Number(m, float64(0.5)).Between(float64(1.0), float64(2.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 2, but it is 0.5`)

	// Test boundary values - success cases
	m.Reset()
	assert.Number(m, int8(1)).Between(int8(1), int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(10)).Between(int8(1), int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(1.0)).Between(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.0)).Between(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal("")

	// Test with different numeric types - failure cases (above upper bound)
	m.Reset()
	assert.Number(m, int8(15)).Between(int8(1), int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 10, but it is 15`)

	m.Reset()
	assert.Number(m, int16(150)).Between(int16(10), int16(100))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 10 and 100, but it is 150`)

	m.Reset()
	assert.Number(m, int32(1500)).Between(int32(100), int32(1000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 100 and 1000, but it is 1500`)

	m.Reset()
	assert.Number(m, int64(15000)).Between(int64(1000), int64(10000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1000 and 10000, but it is 15000`)

	m.Reset()
	assert.Number(m, uint(15)).Between(uint(1), uint(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 10, but it is 15`)

	m.Reset()
	assert.Number(m, uint16(150)).Between(uint16(10), uint16(100))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 10 and 100, but it is 150`)

	m.Reset()
	assert.Number(m, uint32(1500)).Between(uint32(100), uint32(1000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 100 and 1000, but it is 1500`)

	m.Reset()
	assert.Number(m, uint64(15000)).Between(uint64(1000), uint64(10000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1000 and 10000, but it is 15000`)

	m.Reset()
	assert.Number(m, float32(3.5)).Between(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 3, but it is 3.5`)

	m.Reset()
	assert.Number(m, float64(2.5)).Between(float64(1.0), float64(2.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between 1 and 2, but it is 2.5`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, int32(-5)).Between(int32(-10), int32(0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(-15)).Between(int32(-10), int32(0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between -10 and 0, but it is -15`)

	m.Reset()
	assert.Number(m, int32(5)).Between(int32(-10), int32(0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be between -10 and 0, but it is 5`)
}

func TestNumber_NotBetween(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 0).NotBetween(1, 10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 15).NotBetween(1, 10)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5).NotBetween(1, 10)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 10, but it is 5`)

	m.Reset()
	require.Number(m, 5).NotBetween(1, 10, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number not to be between 1 and 10, but it is 5
 message: "index is 0"`)

	// Test with different numeric types - one success case and one failure case for each type
	m.Reset()
	assert.Number(m, int8(0)).NotBetween(int8(1), int8(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(5)).NotBetween(int8(1), int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 10, but it is 5`)

	m.Reset()
	assert.Number(m, int16(150)).NotBetween(int16(10), int16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1500)).NotBetween(int32(100), int32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(15000)).NotBetween(int64(1000), int64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(0)).NotBetween(uint(1), uint(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(150)).NotBetween(uint16(10), uint16(100))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1500)).NotBetween(uint32(100), uint32(1000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(15000)).NotBetween(uint64(1000), uint64(10000))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.5)).NotBetween(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.5)).NotBetween(float64(1.0), float64(2.0))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(2.5)).NotBetween(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 3, but it is 2.5`)

	m.Reset()
	assert.Number(m, float64(1.5)).NotBetween(float64(1.0), float64(2.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 2, but it is 1.5`)

	// Test boundary values - failure cases
	m.Reset()
	assert.Number(m, int8(1)).NotBetween(int8(1), int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 10, but it is 1`)

	m.Reset()
	assert.Number(m, int8(10)).NotBetween(int8(1), int8(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 10, but it is 10`)

	m.Reset()
	assert.Number(m, float32(1.0)).NotBetween(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 3, but it is 1`)

	m.Reset()
	assert.Number(m, float32(3.0)).NotBetween(float32(1.0), float32(3.0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 3, but it is 3`)

	// Test with different numeric types - failure cases (within range)
	m.Reset()
	assert.Number(m, int16(50)).NotBetween(int16(10), int16(100))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 10 and 100, but it is 50`)

	m.Reset()
	assert.Number(m, int32(500)).NotBetween(int32(100), int32(1000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 100 and 1000, but it is 500`)

	m.Reset()
	assert.Number(m, int64(5000)).NotBetween(int64(1000), int64(10000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1000 and 10000, but it is 5000`)

	m.Reset()
	assert.Number(m, uint(5)).NotBetween(uint(1), uint(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1 and 10, but it is 5`)

	m.Reset()
	assert.Number(m, uint16(50)).NotBetween(uint16(10), uint16(100))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 10 and 100, but it is 50`)

	m.Reset()
	assert.Number(m, uint32(500)).NotBetween(uint32(100), uint32(1000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 100 and 1000, but it is 500`)

	m.Reset()
	assert.Number(m, uint64(5000)).NotBetween(uint64(1000), uint64(10000))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between 1000 and 10000, but it is 5000`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, int32(-5)).NotBetween(int32(-10), int32(0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between -10 and 0, but it is -5`)

	m.Reset()
	assert.Number(m, int32(0)).NotBetween(int32(-10), int32(0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between -10 and 0, but it is 0`)

	m.Reset()
	assert.Number(m, int32(-10)).NotBetween(int32(-10), int32(0))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number not to be between -10 and 0, but it is -10`)
}

func TestNumber_InDelta(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5.2).InDelta(5.0, 0.3)
	assert.String(t, m.String()).Equal("")

	assert.Number(m, 5.2).InDelta(5.5, 0.3)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5.6).InDelta(5.0, 0.3)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±0.3 of 5, but it is 5.6`)

	m.Reset()
	require.Number(m, 5.6).InDelta(5.0, 0.3, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be within ±0.3 of 5, but it is 5.6
 message: "index is 0"`)

	// Test with different numeric types - one success case and one failure case for each type
	m.Reset()
	assert.Number(m, int8(5)).InDelta(int8(4), int8(2))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int8(10)).InDelta(int8(5), int8(2))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±2 of 5, but it is 10`)

	m.Reset()
	assert.Number(m, int16(100)).InDelta(int16(95), int16(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).InDelta(int32(990), int32(15))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(10000)).InDelta(int64(9990), int64(15))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).InDelta(uint(4), uint(2))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).InDelta(uint16(95), uint16(10))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).InDelta(uint32(990), uint32(15))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).InDelta(uint64(9990), uint64(15))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).InDelta(float32(3.10), float32(0.05))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).InDelta(float64(2.718), float64(0.001))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(100)).InDelta(int16(80), int16(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±10 of 80, but it is 100`)

	m.Reset()
	assert.Number(m, int32(1000)).InDelta(int32(950), int32(20))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±20 of 950, but it is 1000`)

	m.Reset()
	assert.Number(m, int64(10000)).InDelta(int64(9900), int64(50))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±50 of 9900, but it is 10000`)

	m.Reset()
	assert.Number(m, uint(10)).InDelta(uint(5), uint(2))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±2 of 5, but it is 10`)

	m.Reset()
	assert.Number(m, uint16(100)).InDelta(uint16(80), uint16(10))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±10 of 80, but it is 100`)

	m.Reset()
	assert.Number(m, uint32(1000)).InDelta(uint32(950), uint32(20))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±20 of 950, but it is 1000`)

	m.Reset()
	assert.Number(m, uint64(10000)).InDelta(uint64(9900), uint64(50))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±50 of 9900, but it is 10000`)

	m.Reset()
	assert.Number(m, float32(3.2)).InDelta(float32(3.10), float32(0.05))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±0.05 of 3.1, but it is 3.2`)

	m.Reset()
	assert.Number(m, float64(2.72)).InDelta(float64(2.718), float64(0.001))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±0.001 of 2.718, but it is 2.72`)

	// Test boundary values - failure cases
	m.Reset()
	assert.Number(m, float32(1.2)).InDelta(float32(1.0), float32(0.1))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±0.1 of 1, but it is 1.2`)

	m.Reset()
	assert.Number(m, float64(2.2)).InDelta(float64(2.0), float64(0.1))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±0.1 of 2, but it is 2.2`)

	// Test with negative numbers
	m.Reset()
	assert.Number(m, int32(-5)).InDelta(int32(-4), int32(2))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(-1.5)).InDelta(float64(-1.4), float64(0.2))
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(-10)).InDelta(int32(-5), int32(2))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±2 of -5, but it is -10`)

	m.Reset()
	assert.Number(m, float64(-1.7)).InDelta(float64(-1.4), float64(0.2))
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be within ±0.2 of -1.4, but it is -1.7`)
}

func TestNumber_IsNaN(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, math.NaN()).IsNaN()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5.0).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 5`)

	m.Reset()
	require.Number(m, 5.0).IsNaN("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be NaN, but it is 5
 message: "index is 0"`)

	// Test with float32 NaN
	m.Reset()
	assert.Number(m, float32(math.NaN())).IsNaN()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 3.14`)

	// Test with integer types - should always fail as integers cannot be NaN
	m.Reset()
	assert.Number(m, int(5)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 5`)

	m.Reset()
	assert.Number(m, int8(10)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 10`)

	m.Reset()
	assert.Number(m, int16(100)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 100`)

	m.Reset()
	assert.Number(m, int32(1000)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 1000`)

	m.Reset()
	assert.Number(m, int64(10000)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 10000`)

	m.Reset()
	assert.Number(m, uint(5)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 5`)

	m.Reset()
	assert.Number(m, uint8(255)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 255`)

	m.Reset()
	assert.Number(m, uint16(100)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 100`)

	m.Reset()
	assert.Number(m, uint32(1000)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 1000`)

	m.Reset()
	assert.Number(m, uint64(10000)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is 10000`)

	// Test with infinity - should fail as infinity is not NaN
	m.Reset()
	assert.Number(m, math.Inf(1)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is +Inf`)

	m.Reset()
	assert.Number(m, math.Inf(-1)).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is -Inf`)

	m.Reset()
	assert.Number(m, float32(float32(math.Inf(1)))).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is +Inf`)

	m.Reset()
	assert.Number(m, float32(float32(math.Inf(-1)))).IsNaN()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be NaN, but it is -Inf`)
}

func TestNumber_IsInf(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, math.Inf(1)).IsInf(1)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, math.Inf(-1)).IsInf(-1)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, 5.0).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 5`)

	m.Reset()
	require.Number(m, 5.0).IsInf(-1, "index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be -Inf, but it is 5
 message: "index is 0"`)

	// Test with float32 infinity
	m.Reset()
	assert.Number(m, float32(math.Inf(1))).IsInf(1)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(math.Inf(-1))).IsInf(-1)
	assert.String(t, m.String()).Equal("")

	// Test with sign = 0 (any infinity)
	m.Reset()
	assert.Number(m, math.Inf(1)).IsInf(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, math.Inf(-1)).IsInf(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(math.Inf(1))).IsInf(0)
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(math.Inf(-1))).IsInf(0)
	assert.String(t, m.String()).Equal("")

	// Test with wrong sign
	m.Reset()
	assert.Number(m, math.Inf(1)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is +Inf`)

	m.Reset()
	assert.Number(m, math.Inf(-1)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is -Inf`)

	m.Reset()
	assert.Number(m, float32(math.Inf(1))).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is +Inf`)

	m.Reset()
	assert.Number(m, float32(math.Inf(-1))).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is -Inf`)

	// Test with finite numbers - should fail
	m.Reset()
	assert.Number(m, int(5)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 5`)

	m.Reset()
	assert.Number(m, int8(10)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is 10`)

	m.Reset()
	assert.Number(m, int16(100)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 100`)

	m.Reset()
	assert.Number(m, int32(1000)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is 1000`)

	m.Reset()
	assert.Number(m, int64(10000)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 10000`)

	m.Reset()
	assert.Number(m, uint(5)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is 5`)

	m.Reset()
	assert.Number(m, uint8(255)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 255`)

	m.Reset()
	assert.Number(m, uint16(100)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is 100`)

	m.Reset()
	assert.Number(m, uint32(1000)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 1000`)

	m.Reset()
	assert.Number(m, uint64(10000)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is 10000`)

	m.Reset()
	assert.Number(m, float32(3.14)).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is 3.14`)

	m.Reset()
	assert.Number(m, float64(2.71828)).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is 2.71828`)

	// Test with NaN - should fail
	m.Reset()
	assert.Number(m, math.NaN()).IsInf(1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be +Inf, but it is NaN`)

	m.Reset()
	assert.Number(m, float32(math.NaN())).IsInf(-1)
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be -Inf, but it is NaN`)
}

func TestNumber_IsFinite(t *testing.T) {
	m := new(internal.MockTestingT)

	m.Reset()
	assert.Number(m, 5.0).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(5.0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(5)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, math.Inf(1)).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is +Inf`)

	m.Reset()
	require.Number(m, math.Inf(-1)).IsFinite("index is 0")
	assert.String(t, m.String()).Equal(`fatal# Assertion failed: expected number to be finite, but it is -Inf
 message: "index is 0"`)

	// Test with different numeric types - one success case for each type
	m.Reset()
	assert.Number(m, int8(5)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(100)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(1000)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(5)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(255)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(100)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(1000)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(10000)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(3.14)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(2.71828)).IsFinite()
	assert.String(t, m.String()).Equal("")

	// Test with zero values - should succeed
	m.Reset()
	assert.Number(m, int8(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint8(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint16(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint32(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, uint64(0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(0.0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(0.0)).IsFinite()
	assert.String(t, m.String()).Equal("")

	// Test with negative values - should succeed
	m.Reset()
	assert.Number(m, int8(-5)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int16(-100)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int32(-1000)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, int64(-10000)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float32(-3.14)).IsFinite()
	assert.String(t, m.String()).Equal("")

	m.Reset()
	assert.Number(m, float64(-2.71828)).IsFinite()
	assert.String(t, m.String()).Equal("")

	// Test with NaN - should fail
	m.Reset()
	assert.Number(m, math.NaN()).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is NaN`)

	m.Reset()
	assert.Number(m, float32(math.NaN())).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is NaN`)

	// Test with infinity - should fail
	m.Reset()
	assert.Number(m, math.Inf(1)).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is +Inf`)

	m.Reset()
	assert.Number(m, math.Inf(-1)).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is -Inf`)

	m.Reset()
	assert.Number(m, float32(math.Inf(1))).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is +Inf`)

	m.Reset()
	assert.Number(m, float32(math.Inf(-1))).IsFinite()
	assert.String(t, m.String()).Equal(`error# Assertion failed: expected number to be finite, but it is -Inf`)
}
