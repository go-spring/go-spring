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

// Package fault is the in-process fault-injection companion to
// go-spring.org/cloud/governance/resilience. It wraps a [resilience.Executor] so a
// configurable fraction of operations are made to fail or slow down on demand
// — "setting fire" to a running client — to verify that retry, circuit-breaker,
// per-attempt timeout and Fallback actually engage, and that the observe kit
// records the resulting outcomes.
//
// The injection happens INSIDE the resilience executor's retry loop (the
// wrapped operation fn returns the injected error, then the real executor
// retries/breaks around it), so a fault exercises the protection stack rather
// than bypassing it. See [WrapExecutor].
//
// fault stays stdlib-only and depends only on [resilience]; the gs runtime
// binding (hot-reload via the governance source) lives in starter-governance,
// mirroring the layering of the resilience package.
package fault

import (
	"context"
	"errors"
	"fmt"
	"syscall"
)

// InjectedError is the error shape every fault-injected failure returns. It
// implements [resilience.Retryable] (Retryable reports true) so the resilience
// executor's retry loop treats injected faults as retryable — the resolution in
// [resilience.Policy.ShouldRetry] lets a Retryable error's verdict win, so
// injection deterministically drives retries regardless of the host's configured
// retry predicate.
//
// Detect any injected fault with errors.As:
//
//	var ie *fault.InjectedError
//	if errors.As(err, &ie) { ... }
//
// Inner, when set, lets the injected error surface as a familiar Go error:
// context.DeadlineExceeded for the "timeout" kind, syscall.ECONNRESET for
// "reset". errors.Is(err, context.DeadlineExceeded) then works as expected, so
// downstream classifiers (e.g. the observe outcome mapping) label the call the
// same way a real timeout would be labelled.
type InjectedError struct {
	Kind  string // "generic", "timeout" or "reset"
	Inner error  // optional underlying error
}

func (e *InjectedError) Error() string {
	if e.Inner != nil {
		return fmt.Sprintf("fault: injected %s failure: %v", e.Kind, e.Inner)
	}
	return fmt.Sprintf("fault: injected %s failure", e.Kind)
}

// Retryable opts the injected fault into retry. It is the hook the resilience
// executor's ShouldRetry looks for first.
func (e *InjectedError) Retryable() bool { return true }

// Unwrap exposes Inner so errors.Is(err, context.DeadlineExceeded) and similar
// checks work for the typed kinds.
func (e *InjectedError) Unwrap() error { return e.Inner }

// ErrInjected is a convenience sentinel for a generic injected failure. The
// typed kinds ("timeout", "reset") return distinct *InjectedError values that
// wrap a real error; detect them all with errors.As against *InjectedError.
var ErrInjected = &InjectedError{Kind: "generic"}

// Is reports whether target is an [InjectedError] (any kind). It lets callers
// write errors.Is(err, fault.ErrInjected) to detect any injected fault,
// including the typed kinds that do not share identity with ErrInjected.
func (e *InjectedError) Is(target error) bool {
	_, ok := target.(*InjectedError)
	return ok
}

// mapError translates a configured kind into the error returned to the
// executor. Empty or unknown kinds behave as "generic".
func mapError(kind string) error {
	switch kind {
	case "timeout":
		return &InjectedError{Kind: "timeout", Inner: context.DeadlineExceeded}
	case "reset":
		return &InjectedError{Kind: "reset", Inner: syscall.ECONNRESET}
	default:
		return ErrInjected
	}
}

// IsInjected reports whether err is a fault-injected failure of any kind. It is
// a shorthand for the errors.As call documented on [InjectedError].
func IsInjected(err error) bool {
	var ie *InjectedError
	return errors.As(err, &ie)
}
