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
// sequence that builds these errors is [evaluate] in manager.go, and the adapters
// that turn one into their protocol's failure live in outbound/fault and
// inbound/fault.

// The three failure classes a rule can declare ([Rule.Class]). They exist
// because "an error" is two independent questions to the caller's resilience
// stack — may I retry it, and is it evidence against the downstream? — and the
// mainstream in-process chaos libraries separate them for the same reason
// (Hystrix's HystrixBadRequestException is the canonical "don't retry, don't
// count" case).
const (
	// ClassTransient is the default: retryable, and counted as a downstream
	// failure by the circuit breaker. It models an ordinary transient fault.
	ClassTransient = "transient"

	// ClassPermanent is not retryable but is still evidence against the
	// downstream: a deterministic failure that repeated attempts cannot fix, yet
	// that a breaker should still count.
	ClassPermanent = "permanent"

	// ClassBusiness is neither retryable nor counted by the breaker — the shape
	// of a caller mistake (an HTTP 4xx equivalent). It must not be retried and
	// must not open a circuit against a healthy downstream.
	ClassBusiness = "business"
)

// The failure kinds a rule can declare ([Rule.Error]) — WHICH familiar Go
// failure the injection surfaces as. They are string constants, not a named
// enum type, because [Rule.Error] is a bound config field: values arrive as
// strings, judged by the field's `expr` tag. The tag's default fills
// [ErrorGeneric] when the key is omitted; an explicit empty is rejected.
const (
	// ErrorGeneric is a plain retryable injected error ([ErrInjected]) that
	// wraps nothing — the kind to use when the drill is about the resilience
	// stack's reaction, not about a specific network failure.
	ErrorGeneric = "generic"

	// ErrorTimeout surfaces as context.DeadlineExceeded, so classifiers and
	// errors.Is treat the call as a real timeout.
	ErrorTimeout = "timeout"

	// ErrorReset surfaces as syscall.ECONNRESET — the connection was reset
	// mid-flight.
	ErrorReset = "reset"

	// ErrorRefused surfaces as syscall.ECONNREFUSED — nothing is listening.
	ErrorRefused = "refused"
)

// InjectedError is the error shape every fault-injected failure returns. It
// implements [resilience.Retryable] and [resilience.FailureClass], so the
// resilience executor's retry loop and breaker both classify an injected fault
// exactly as the rule declared ([Rule.Class]):
//
//   - [ClassTransient] — retried, counted as a failure;
//   - [ClassPermanent] — NOT retried, still counted;
//   - [ClassBusiness]  — NOT retried, NOT counted.
//
// Carrying the class ON the error (rather than in some caller-side list) is the
// only way this framework can express it: classification here is identity-driven
// — an error answers [resilience.Retryable]/[resilience.FailureClass] for
// itself, like every real error does. It is what lets a drill exercise the
// paths a hard-wired retryable injection cannot reach: "a 4xx must not be
// retried", "a 4xx must not open the breaker".
//
// Detect any injected fault with errors.As:
//
//	var ie *fault.InjectedError
//	if errors.As(err, &ie) { ... }
//
// Inner, when set, lets the injected error surface as a familiar Go error:
// context.DeadlineExceeded for [ErrorTimeout], syscall.ECONNRESET for
// [ErrorReset], syscall.ECONNREFUSED for [ErrorRefused]. errors.Is(err,
// context.DeadlineExceeded) then works as expected, so downstream classifiers
// (e.g. the observe outcome mapping) label the call the same way a real
// timeout would be labelled.
type InjectedError struct {
	// Kind is which familiar Go failure the injection surfaces as
	// ([ErrorGeneric], [ErrorTimeout], [ErrorReset] or [ErrorRefused]).
	Kind string

	// Inner is the optional underlying error — context.DeadlineExceeded for
	// [ErrorTimeout], syscall.ECONNRESET for [ErrorReset], and so on.
	Inner error

	// Class is the failure class the rule declared ([ClassTransient],
	// [ClassPermanent], [ClassBusiness]); a rule always carries one (its expr
	// tag rejects the empty spelling). A hand-built zero value behaves as
	// [ClassTransient].
	Class string
}

func (e *InjectedError) Error() string {
	if e.Inner != nil {
		return fmt.Sprintf("fault: injected %s failure: %v", e.Kind, e.Inner)
	}
	return fmt.Sprintf("fault: injected %s failure", e.Kind)
}

// Retryable opts the injected fault into (or out of) retry. It is the hook the
// resilience executor's ShouldRetry looks for first; a non-transient class opts
// out, so "a permanent/business failure is not retried" is testable.
func (e *InjectedError) Retryable() bool {
	return e.Class != ClassPermanent && e.Class != ClassBusiness
}

// CountsAsFailure reports whether the breaker should count this failure. A
// business failure — the caller's mistake — does not, mirroring how a 4xx is
// not evidence against the downstream. The resilience executor consults this
// through [resilience.FailureClass].
func (e *InjectedError) CountsAsFailure() bool { return e.Class != ClassBusiness }

// Unwrap exposes Inner so errors.Is(err, context.DeadlineExceeded) and similar
// checks work for the typed kinds.
func (e *InjectedError) Unwrap() error { return e.Inner }

// Is makes errors.Is(err, fault.ErrInjected) true for ANY injected fault,
// including the typed kinds that do not share identity with ErrInjected. It
// deliberately matches only the sentinel: matching every *InjectedError would
// make errors.Is(err, &InjectedError{Class: ClassBusiness}) true for a transient
// error too, erasing the class. Compare Kind or Class on the value instead.
func (e *InjectedError) Is(target error) bool { return target == ErrInjected }

// ErrInjected is the sentinel a generic injected failure relates to. The typed
// kinds ([ErrorTimeout], [ErrorReset], [ErrorRefused]) return distinct
// *InjectedError values that wrap a real error; detect them all with
// errors.As against *InjectedError, or with errors.Is against this sentinel.
var ErrInjected = &InjectedError{Kind: ErrorGeneric, Class: ClassTransient}

// IsInjected reports whether err is a fault-injected failure of any kind. It is
// a shorthand for the errors.As call documented on [InjectedError].
func IsInjected(err error) bool {
	var ie *InjectedError
	return errors.As(err, &ie)
}

// newInjectedError builds the error returned to the executor for a rolled dose.
// The kind arrives already vetted by [Rule.Error]'s expr tag, so the switch
// only attaches the familiar inner error; [ErrorGeneric] wraps nothing.
func newInjectedError(kind, class string) error {
	ie := &InjectedError{Kind: kind, Class: class}
	switch kind {
	case ErrorTimeout:
		ie.Inner = context.DeadlineExceeded
	case ErrorReset:
		ie.Inner = syscall.ECONNRESET
	case ErrorRefused:
		ie.Inner = syscall.ECONNREFUSED
	}
	return ie
}
