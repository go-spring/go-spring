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

// Package require provides assertion helpers that stop test execution on failure.
// For assertions that should allow the test to continue on failure, use the `assert` package.
package require

import (
	"go-spring.org/stdlib/testing/internal"
)

// fatalOnFailure indicates whether to stop the test when an assertion fails.
const fatalOnFailure = true

// Panic asserts that `fn` panics and the panic message matches `expr`.
// It reports an error if `fn` does not panic or if the recovered message does not satisfy `expr`.
func Panic(t internal.TestingT, fn func(), expr string, msg ...string) {
	t.Helper()
	internal.Panic(t, fatalOnFailure, fn, expr, msg...)
}

// That creates an Assertion for the given value v and test context t.
func That(t internal.TestingT, v any) *internal.Assertion {
	return internal.That(t, v, fatalOnFailure)
}

// Error returns a new ErrorAssertion for the given error value.
func Error(t internal.TestingT, v error) *internal.ErrorAssertion {
	return internal.ThatError(t, v, fatalOnFailure)
}

// Number returns a NumberAssertion for the given testing object and number value.
func Number[T internal.Number](t internal.TestingT, v T) *internal.NumberAssertion[T] {
	return internal.ThatNumber(t, v, fatalOnFailure)
}

// String returns a StringAssertion for the given testing object and string value.
func String(t internal.TestingT, v string) *internal.StringAssertion {
	return internal.ThatString(t, v, fatalOnFailure)
}

// Slice returns a SliceAssertion for the given testing object and slice value.
func Slice[T comparable](t internal.TestingT, v []T) *internal.SliceAssertion[T] {
	return internal.ThatSlice(t, v, fatalOnFailure)
}

// Map returns a MapAssertion for the given testing object and map value.
func Map[K, V comparable](t internal.TestingT, v map[K]V) *internal.MapAssertion[K, V] {
	return internal.ThatMap(t, v, fatalOnFailure)
}
