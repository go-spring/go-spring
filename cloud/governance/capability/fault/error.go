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
)

// error.go is the OUTCOME vocabulary: the one error shape an injected OUTBOUND
// failure returns, and the two ways to detect it. Nothing here decides
// anything — the sequence that builds these errors is [Kernel.Fire], and the
// adapters that turn one into their protocol's failure live in outbound/fault
// and inbound/fault.

// The two failure classes an OUTBOUND rule can declare. They exist because
// "an error" is two independent questions to the caller's resilience stack —
// may I retry it, and is it evidence against the downstream? — and the
// mainstream in-process chaos libraries separate them for the same reason
// (Hystrix's ignoreExceptions is the caller-side "don't retry, don't count"
// case).
//
// There is no "business" class: a business answer is a RESPONSE, not an error
// on the governance chain. A real client adapter folds one to nil with
// resilience.Tolerate before the executor runs, and an inbound drill declares
// its response status instead.
const (
	// ClassTransient is the default: retryable, and counted as a downstream
	// failure by the circuit breaker. It models an ordinary transient fault.
	ClassTransient = "transient"

	// ClassPermanent is not retryable but is still evidence against the
	// downstream: a deterministic failure that repeated attempts cannot fix, yet
	// that a breaker should still count.
	ClassPermanent = "permanent"
)

// The failure kinds an OUTBOUND rule can declare — WHICH familiar Go failure
// the injection surfaces as. They are string constants, not a named enum type,
// because the rule field is bound config: values arrive as strings, judged by
// the field's `expr` tag. The tag's default fills [ErrorGeneric] when the key is
// omitted; an explicit empty is rejected.
const (
	// ErrorGeneric is a plain retryable injected error ([ErrInjected]) that
	// wraps nothing — the kind to use when the drill is about the resilience
	// stack's reaction, not about a specific network failure.
	ErrorGeneric = "generic"

	// ErrorTimeout surfaces as os.ErrDeadlineExceeded — an I/O READ/WRITE
	// deadline, the shape a connection or read timeout has. It is deliberately
	// NOT context.DeadlineExceeded: that one is a caller/chain-signal deadline,
	// which the chain treats as no evidence against the peer.
	ErrorTimeout = "timeout"

	// ErrorReset surfaces as syscall.ECONNRESET — the connection was reset
	// mid-flight.
	ErrorReset = "reset"

	// ErrorRefused surfaces as syscall.ECONNREFUSED — nothing is listening.
	ErrorRefused = "refused"
)

// InjectedError is the error shape an injected OUTBOUND failure returns. It
// carries the rule's class and implements [resilience.Retryable], so the
// resilience executor's retry loop classifies an injected fault exactly as the
// rule declared:
//
//   - [ClassTransient] — retried, counted as a failure;
//   - [ClassPermanent] — NOT retried, still counted.
//
// Carrying the class ON the error remains what lets a drill exercise the path a
// hard-wired retryable injection cannot reach: "this failure must not be
// retried". An inbound drill does not use this vocabulary: it declares the
// response status it wants ([Result.Status]) and still returns an error only to
// short-circuit the handler.
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

	// Inner is the optional underlying error — os.ErrDeadlineExceeded for
	// [ErrorTimeout], syscall.ECONNRESET for [ErrorReset], and so on.
	Inner error

	// Class is the failure class the rule declared ([ClassTransient],
	// [ClassPermanent]); an outbound rule always carries one (its expr tag
	// rejects the empty spelling). A hand-built zero value behaves as
	// [ClassTransient]. Empty on an inbound injection, which has no class.
	Class string

	// Status is the protocol status the rule declared (400–599), when it gave
	// one. It is the adapter's to interpret: a client library whose failure IS
	// an error (gRPC-like protocols) can translate it into its native error,
	// while a client where a 5xx is a RESPONSE (net/http) can only surface it
	// through response replacement. Zero means "no protocol status declared".
	Status int
}

func (e *InjectedError) Error() string {
	base := fmt.Sprintf("fault: injected %s failure", e.Kind)
	if e.Inner != nil {
		base = fmt.Sprintf("%s: %v", base, e.Inner)
	}
	if e.Status != 0 {
		base = fmt.Sprintf("%s (status %d)", base, e.Status)
	}
	return base
}

// Retryable opts the injected fault into (or out of) retry. It is the hook the
// resilience executor's ShouldRetry looks for first; a non-transient class opts
// out, so "a permanent failure is not retried" is testable.
func (e *InjectedError) Retryable() bool {
	return e.Class != ClassPermanent
}

// Unwrap exposes Inner so errors.Is(err, os.ErrDeadlineExceeded) and similar
// checks work for the typed kinds.
func (e *InjectedError) Unwrap() error { return e.Inner }

// Is makes errors.Is(err, fault.ErrInjected) true for ANY injected fault,
// including the typed kinds that do not share identity with ErrInjected. It
// deliberately matches only the sentinel: matching every *InjectedError would
// make errors.Is(err, &InjectedError{Class: ClassPermanent}) true for a
// transient error too, erasing the class. Compare Kind or Class on the value
// instead.
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

// StatusOf reports the protocol status an injected error carries, if any. A
// client adapter whose protocol has a native status error can use it to
// translate; an unrecognized error reports false.
func StatusOf(err error) (int, bool) {
	var ie *InjectedError
	if errors.As(err, &ie) && ie.Status != 0 {
		return ie.Status, true
	}
	return 0, false
}

// NewInjectedError builds the error a direction's rule returns for a rolled
// dose. The kind arrives already vetted by the rule field's expr tag, so the
// switch only attaches the familiar inner error; [ErrorGeneric] wraps nothing.
// status is the rule's declared protocol status (0 when none).
//
// It is exported for the direction rule types (their Roll implementations); the
// kernel itself never constructs an error.
func NewInjectedError(kind, class string, status int) error {
	ie := &InjectedError{Kind: kind, Class: class, Status: status}
	switch kind {
	case ErrorTimeout:
		ie.Inner = os.ErrDeadlineExceeded
	case ErrorReset:
		ie.Inner = syscall.ECONNRESET
	case ErrorRefused:
		ie.Inner = syscall.ECONNREFUSED
	}
	return ie
}
