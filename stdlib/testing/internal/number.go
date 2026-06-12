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

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

// NumberAssertion encapsulates a number value and a test handler for making assertions on the number.
type NumberAssertion[T Number] struct {
	AssertionBase
	v T
}

// ThatNumber returns a NumberAssertion for the given testing object and number value.
func ThatNumber[T Number](t TestingT, v T, fatalOnFailure bool) *NumberAssertion[T] {
	return &NumberAssertion[T]{
		AssertionBase: AssertionBase{
			t:              t,
			fatalOnFailure: fatalOnFailure,
		},
		v: v,
	}
}

// Equal asserts that the number value is equal to the expected value.
func (a *NumberAssertion[T]) Equal(expect T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v != expect {
		str := fmt.Sprintf(`expected number to be equal to %v, but it is %v`, expect, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotEqual asserts that the number value is not equal to the expected value.
func (a *NumberAssertion[T]) NotEqual(expect T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v == expect {
		str := fmt.Sprintf(`expected number not to be equal to %v, but it is`, expect)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// GreaterThan asserts that the number value is greater than the expected value.
func (a *NumberAssertion[T]) GreaterThan(expect T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v <= expect {
		str := fmt.Sprintf(`expected number to be greater than %v, but it is %v`, expect, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// GreaterOrEqual asserts that the number value is greater than or equal to the expected value.
func (a *NumberAssertion[T]) GreaterOrEqual(expect T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v < expect {
		str := fmt.Sprintf(`expected number to be greater than or equal to %v, but it is %v`, expect, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// LessThan asserts that the number value is less than the expected value.
func (a *NumberAssertion[T]) LessThan(expect T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v >= expect {
		str := fmt.Sprintf(`expected number to be less than %v, but it is %v`, expect, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// LessOrEqual asserts that the number value is less than or equal to the expected value.
func (a *NumberAssertion[T]) LessOrEqual(expect T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v > expect {
		str := fmt.Sprintf(`expected number to be less than or equal to %v, but it is %v`, expect, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Zero asserts that the number value is zero.
func (a *NumberAssertion[T]) Zero(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v != 0 {
		str := fmt.Sprintf(`expected number to be zero, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotZero asserts that the number value is not zero.
func (a *NumberAssertion[T]) NotZero(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v == 0 {
		str := fmt.Sprintf(`expected number not to be zero, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Positive asserts that the number value is positive.
func (a *NumberAssertion[T]) Positive(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v <= 0 {
		str := fmt.Sprintf(`expected number to be positive, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotPositive asserts that the number value is non-positive.
func (a *NumberAssertion[T]) NotPositive(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v > 0 {
		str := fmt.Sprintf(`expected number to be non-positive, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Negative asserts that the number value is negative.
func (a *NumberAssertion[T]) Negative(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v >= 0 {
		str := fmt.Sprintf(`expected number to be negative, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotNegative asserts that the number value is non-negative.
func (a *NumberAssertion[T]) NotNegative(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v < 0 {
		str := fmt.Sprintf(`expected number to be non-negative, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// Between asserts that the number value is between the lower and upper bounds.
func (a *NumberAssertion[T]) Between(lower, upper T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v < lower || a.v > upper {
		str := fmt.Sprintf(`expected number to be between %v and %v, but it is %v`, lower, upper, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// NotBetween asserts that the number value is not between the lower and upper bounds.
func (a *NumberAssertion[T]) NotBetween(lower, upper T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if a.v >= lower && a.v <= upper {
		str := fmt.Sprintf(`expected number not to be between %v and %v, but it is %v`, lower, upper, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// InDelta asserts that the number value is within the delta range of the expected value.
func (a *NumberAssertion[T]) InDelta(expect T, delta T, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	diff := a.v - expect
	if diff < 0 {
		diff = -diff
	}
	if diff > delta { // todo (lvan100) 精度问题
		str := fmt.Sprintf(`expected number to be within ±%v of %v, but it is %v`, delta, expect, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsNaN asserts that the number value is NaN (Not a Number).
func (a *NumberAssertion[T]) IsNaN(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if !isNaN(a.v) {
		str := fmt.Sprintf(`expected number to be NaN, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsInf asserts that the number value is infinite.
func (a *NumberAssertion[T]) IsInf(sign int, msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if !isInf(a.v, sign) {
		var c string
		if sign >= 0 {
			c = "+"
		} else {
			c = "-"
		}
		str := fmt.Sprintf(`expected number to be %sInf, but it is %v`, c, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// IsFinite asserts that the number value is finite.
func (a *NumberAssertion[T]) IsFinite(msg ...string) *NumberAssertion[T] {
	a.t.Helper()
	if isNaN(a.v) || isInf(a.v, 0) {
		str := fmt.Sprintf(`expected number to be finite, but it is %v`, a.v)
		Fail(a.t, a.fatalOnFailure, str, msg...)
	}
	return a
}

// isNaN checks if the value is NaN.
func isNaN[T Number](v T) bool {
	switch any(v).(type) {
	case float32:
		return any(v).(float32) != any(v).(float32)
	case float64:
		return any(v).(float64) != any(v).(float64)
	default:
		return false
	}
}

// isInf checks if the value is infinite.
func isInf[T Number](v T, sign int) bool {
	switch any(v).(type) {
	case float32:
		f := any(v).(float32)
		return (sign >= 0 && f > maxFloat32) || (sign <= 0 && f < -maxFloat32)
	case float64:
		f := any(v).(float64)
		return (sign >= 0 && f > maxFloat64) || (sign <= 0 && f < -maxFloat64)
	default:
		return false
	}
}

const (
	maxFloat32 = 3.4028234663852886e+38
	maxFloat64 = 1.7976931348623157e+308
)
