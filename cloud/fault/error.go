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
)

// error.go is the OUTCOME vocabulary: the one error shape every injected failure
// returns, and the two ways to detect it. Nothing here decides anything — the
// sequence that builds these errors is manager.go, and the adapters that turn one
// into their protocol's failure are executor.go and [Server.Fire].

// InjectedError is the error shape every fault-injected failure returns. It
// implements [resilience.Retryable] (Retryable reports true) so the resilience
// executor's retry loop treats injected faults as retryable — the classification
// in [resilience.ShouldRetry] lets a Retryable error's verdict win, so injection
// deterministically drives retries regardless of the host's configured retry
// predicate.
//
// Detect any injected fault with errors.As:
//
//	var ie *fault.InjectedError
//	if errors.As(err, &ie) { ... }
//
// Inner, when set, lets the injected error surface as a familiar Go error:
// context.DeadlineExceeded for the "timeout" kind, syscall.ECONNRESET for
// "reset", syscall.ECONNREFUSED for "refused". errors.Is(err,
// context.DeadlineExceeded) then works as expected, so downstream classifiers
// (e.g. the observe outcome mapping) label the call the same way a real
// timeout would be labelled.
type InjectedError struct {
	Kind  string // "generic", "timeout", "reset" or "refused"
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

// Is reports whether target is an [InjectedError] (any kind). It lets callers
// write errors.Is(err, fault.ErrInjected) to detect any injected fault,
// including the typed kinds that do not share identity with ErrInjected.
func (e *InjectedError) Is(target error) bool {
	_, ok := target.(*InjectedError)
	return ok
}

// ErrInjected is a convenience sentinel for a generic injected failure. The
// typed kinds ("timeout", "reset") return distinct *InjectedError values that
// wrap a real error; detect them all with errors.As against *InjectedError.
var ErrInjected = &InjectedError{Kind: "generic"}

// IsInjected reports whether err is a fault-injected failure of any kind. It is
// a shorthand for the errors.As call documented on [InjectedError].
func IsInjected(err error) bool {
	var ie *InjectedError
	return errors.As(err, &ie)
}

// newInjectedError builds the error returned to the executor for a configured
// kind. Empty or unknown kinds behave as "generic".
func newInjectedError(kind string) error {
	switch kind {
	case "timeout":
		return &InjectedError{Kind: "timeout", Inner: context.DeadlineExceeded}
	case "reset":
		return &InjectedError{Kind: "reset", Inner: syscall.ECONNRESET}
	case "refused":
		return &InjectedError{Kind: "refused", Inner: syscall.ECONNREFUSED}
	default:
		return ErrInjected
	}
}
