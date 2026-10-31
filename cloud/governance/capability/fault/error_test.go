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

package fault

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestInjectedError_Classification pins the two questions the resilience stack
// asks and how each class answers them — the whole reason Class rides on the
// error. The zero value keeps the historical transient behaviour.
func TestInjectedError_Classification(t *testing.T) {
	cases := []struct {
		class     string
		retryable bool
		counts    bool
	}{
		{ClassTransient, true, true},
		{ClassPermanent, false, true},
		{ClassBusiness, false, false},
		{"", true, true}, // a hand-built zero value behaves as transient
	}
	for _, tc := range cases {
		e := &InjectedError{Kind: ErrorGeneric, Class: tc.class}
		assert.That(t, e.Retryable()).Equal(tc.retryable)
		assert.That(t, e.CountsAsFailure()).Equal(tc.counts)
	}
}

// TestInjectedError_TypedKindsSurfaceAsFamiliars pins the whole point of the
// typed kinds: errors.Is against the familiar Go error works, so downstream
// classifiers (the observe outcome mapping, a protocol adapter) label an
// injected timeout exactly like a real one. The generic kind wraps nothing.
func TestInjectedError_TypedKindsSurfaceAsFamiliars(t *testing.T) {
	cases := []struct {
		kind  string
		inner error
	}{
		{ErrorTimeout, context.DeadlineExceeded},
		{ErrorReset, syscall.ECONNRESET},
		{ErrorRefused, syscall.ECONNREFUSED},
	}
	for _, tc := range cases {
		err := newInjectedError(tc.kind, ClassTransient)
		assert.That(t, errors.Is(err, tc.inner)).True()

		var ie *InjectedError
		assert.That(t, errors.As(err, &ie)).True()
		assert.String(t, ie.Kind).Equal(tc.kind)
	}
	assert.That(t, IsInjected(newInjectedError(ErrorGeneric, ClassTransient))).True()
	var generic *InjectedError
	assert.That(t, errors.As(newInjectedError(ErrorGeneric, ClassTransient), &generic)).True()
	assert.That(t, generic.Unwrap()).Nil() // the generic kind wraps nothing
}

// TestInjectedError_Sentinel pins the Is contract: EVERY injected fault —
// generic or typed — satisfies errors.Is against [ErrInjected], while Is
// deliberately does not match arbitrary *InjectedError targets, so comparing
// two differently-built values stays meaningful.
func TestInjectedError_Sentinel(t *testing.T) {
	assert.That(t, errors.Is(newInjectedError(ErrorGeneric, ClassTransient), ErrInjected)).True()
	assert.That(t, errors.Is(newInjectedError(ErrorTimeout, ClassBusiness), ErrInjected)).True()
	assert.That(t, errors.Is(ErrInjected, ErrInjected)).True()
	assert.That(t, errors.Is(newInjectedError(ErrorReset, ClassPermanent), &InjectedError{Kind: ErrorReset, Class: ClassPermanent})).False()
}

// TestInjectedError_Messages pins the Error strings an operator actually reads
// in a log: the kind always rides along, the inner error when one is wrapped.
func TestInjectedError_Messages(t *testing.T) {
	assert.String(t, newInjectedError(ErrorGeneric, ClassTransient).Error()).Equal("fault: injected generic failure")
	assert.String(t, newInjectedError(ErrorTimeout, ClassTransient).Error()).Equal("fault: injected timeout failure: context deadline exceeded")
}

// TestIsInjected pins the detection helper: true for every kind and through
// wrapping, false for nil and for a non-injected error.
func TestIsInjected(t *testing.T) {
	assert.That(t, IsInjected(nil)).False()
	assert.That(t, IsInjected(errors.New("real downstream failure"))).False()
	assert.That(t, IsInjected(newInjectedError(ErrorGeneric, ClassTransient))).True()
	assert.That(t, IsInjected(fmt.Errorf("call failed: %w", newInjectedError(ErrorRefused, ClassTransient)))).True()
}
