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
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestInjectedError_RetryClassification pins the retry question each class
// answers — the whole reason Class rides on the error. The zero value keeps the
// historical transient behaviour.
func TestInjectedError_RetryClassification(t *testing.T) {
	cases := []struct {
		class     string
		retryable bool
	}{
		{ClassTransient, true},
		{ClassPermanent, false},
		{"", true}, // a hand-built zero value behaves as transient
	}
	for _, tc := range cases {
		e := &InjectedError{Kind: ErrorGeneric, Class: tc.class}
		assert.That(t, e.Retryable()).Equal(tc.retryable)
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
		{ErrorTimeout, os.ErrDeadlineExceeded},
		{ErrorReset, syscall.ECONNRESET},
		{ErrorRefused, syscall.ECONNREFUSED},
	}
	for _, tc := range cases {
		err := NewInjectedError(tc.kind, ClassTransient, 0)
		assert.That(t, errors.Is(err, tc.inner)).True()

		var ie *InjectedError
		assert.That(t, errors.As(err, &ie)).True()
		assert.String(t, ie.Kind).Equal(tc.kind)
	}
	assert.That(t, IsInjected(NewInjectedError(ErrorGeneric, ClassTransient, 0))).True()
	var generic *InjectedError
	assert.That(t, errors.As(NewInjectedError(ErrorGeneric, ClassTransient, 0), &generic)).True()
	assert.That(t, generic.Unwrap()).Nil() // the generic kind wraps nothing
}

// TestInjectedError_Sentinel pins the Is contract: EVERY injected fault —
// generic or typed — satisfies errors.Is against [ErrInjected], while Is
// deliberately does not match arbitrary *InjectedError targets, so comparing
// two differently-built values stays meaningful.
func TestInjectedError_Sentinel(t *testing.T) {
	assert.That(t, errors.Is(NewInjectedError(ErrorGeneric, ClassTransient, 0), ErrInjected)).True()
	assert.That(t, errors.Is(NewInjectedError(ErrorTimeout, ClassPermanent, 0), ErrInjected)).True()
	assert.That(t, errors.Is(ErrInjected, ErrInjected)).True()
	assert.That(t, errors.Is(NewInjectedError(ErrorReset, ClassPermanent, 0), &InjectedError{Kind: ErrorReset, Class: ClassPermanent})).False()
}

// TestInjectedError_Messages pins the Error strings an operator actually reads
// in a log: the kind always rides along, the inner error when one is wrapped,
// and the declared status when the rule carried one.
func TestInjectedError_Messages(t *testing.T) {
	assert.String(t, NewInjectedError(ErrorGeneric, ClassTransient, 0).Error()).Equal("fault: injected generic failure")
	assert.String(t, NewInjectedError(ErrorTimeout, ClassTransient, 0).Error()).Equal("fault: injected timeout failure: i/o timeout")
	assert.String(t, NewInjectedError(ErrorGeneric, ClassTransient, 503).Error()).Equal("fault: injected generic failure (status 503)")
}

// TestInjectedError_Status pins the protocol-status channel: a declared status
// rides on the error for an adapter to translate, and StatusOf reads it through
// wrapping.
func TestInjectedError_Status(t *testing.T) {
	err := NewInjectedError(ErrorGeneric, ClassTransient, 503)
	s, ok := StatusOf(err)
	assert.That(t, ok).True()
	assert.Number(t, s).Equal(503)

	s, ok = StatusOf(fmt.Errorf("call failed: %w", err))
	assert.That(t, ok).True()
	assert.Number(t, s).Equal(503)

	_, ok = StatusOf(NewInjectedError(ErrorGeneric, ClassTransient, 0))
	assert.That(t, ok).False()
	_, ok = StatusOf(errors.New("plain"))
	assert.That(t, ok).False()
}

// TestIsInjected pins the detection helper: true for every kind and through
// wrapping, false for nil and for a non-injected error.
func TestIsInjected(t *testing.T) {
	assert.That(t, IsInjected(nil)).False()
	assert.That(t, IsInjected(errors.New("real downstream failure"))).False()
	assert.That(t, IsInjected(NewInjectedError(ErrorGeneric, ClassTransient, 0))).True()
	assert.That(t, IsInjected(fmt.Errorf("call failed: %w", NewInjectedError(ErrorRefused, ClassTransient, 0)))).True()
}
