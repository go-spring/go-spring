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
// errors.go holds the chain's failure vocabulary: the sentinels Execute returns
// when it refuses to run fn or cuts a call off, the provenance markers a site
// attaches to an existing error, and the classifiers that map a call's error
// onto the words [Round.ErrorType] and the observability plane carry. They live
// here — not in any engine — because they are part of the chain contract: every
// engine, and every observer of one, speaks the same words. Wrap or compare the
// sentinels with errors.Is; test a marker with its exported predicate.

package chain

import (
	"context"
	"errors"
	"net"
	"os"
)

// ErrCallTimeout is the chain's verdict that its WHOLE-CALL timeout budget
// elapsed before the call finished. Unlike the rejection words it is not a
// refusal: fn ran and the budget cut the call off. It is distinct from a
// caller's deadline (the parent context) and from a deadline a peer reported
// on its own, which stay plain [context.DeadlineExceeded] — only the chain
// can claim its own budget.
//
// A CONNECT budget that expires is not this word: the dial site marks it
// [ConnectError], so it reports as [ErrorTypeConnectFailed].
var ErrCallTimeout = errors.New("chain: call timeout")

// ErrRateLimited is the chain's verdict that the configured rate limit is
// exceeded. The rate-limit stage reports admission as a boolean (see
// go-spring.org/cloud/governance/capability/ratelimit.Limiter), so this package owns the
// word without depending on the stage's implementation.
var ErrRateLimited = errors.New("chain: rate limited")

// ErrCircuitOpen is the chain's verdict that the circuit breaker for the
// executor's service is open.
var ErrCircuitOpen = errors.New("chain: circuit open")

// ErrBulkheadFull is the chain's verdict that the service already has the
// maximum number of concurrent in-flight operations allowed by the bulkhead.
var ErrBulkheadFull = errors.New("chain: bulkhead full")

// ErrRetryExceeded is the chain's verdict that a RETRY attempt was
// rejected because the executor-wide in-flight retry cap is exhausted — the
// process-level guard against retry amplification. The first attempt of a
// call never takes budget, so it only ever replaces a retry, not the initial
// call.
var ErrRetryExceeded = errors.New("chain: retry exceeded")

// ErrNoEndpoints is the addressing stage's verdict that no endpoint remains
// for a round: the balancer's candidate set is empty, or every candidate was
// excluded as a peer this call already failed against. Like the gate words
// it describes a round that never went out.
var ErrNoEndpoints = errors.New("chain: no endpoints")

// connectError is the structural form of a dial failure. Its type is
// unexported on purpose: the only way to produce it is [ConnectError], so the
// dial site — the one place that knows the failure was a dial — is the only
// place that can make the claim.
type connectError struct{ err error }

func (c *connectError) Error() string { return c.err.Error() }
func (c *connectError) Unwrap() error { return c.err }

// ConnectError marks err as a DIAL failure. The dial site is the only one that
// knows a failure was a connect rather than a downstream answer, so it wraps
// the error here and lets it travel up untouched: the chain's round settlement
// recognizes the mark ([Classify] maps it to [ConnectFailure]) while every
// layer in between — vendor pools, adapters, retries — sees the original error
// through Unwrap. err must be non-nil — this is the FAILURE path's marker, not
// a branch-free wrapper a success path may also run through.
func ConnectError(err error) error {
	return &connectError{err: err}
}

// IsConnectError reports whether err was marked by [ConnectError]. The marker
// wins over the timeout marker: a dial that timed out is a connectivity verdict
// about the peer, which is the stronger fact.
func IsConnectError(err error) bool {
	var ce *connectError
	return errors.As(err, &ce)
}

// resourceError is the structural form of a local resource shortage: one of
// OUR resources (a pool's connections) ran out or was closed before anything
// was sent. Its type is unexported on purpose: the only way
// to produce it is [ResourceError].
type resourceError struct{ err error }

func (l *resourceError) Error() string { return l.err.Error() }
func (l *resourceError) Unwrap() error { return l.err }

// ResourceError marks err as a local resource failure — a resource of OURS ran
// out (or was closed), the endpoint was never asked anything. The resource's owner wraps the
// error here; the chain's round settlement recognizes the mark ([Classify]
// maps it to [ResourceExhausted], no evidence against the peer) while every layer
// in between sees the original error through Unwrap. err must be non-nil —
// this is the FAILURE path's marker, not a branch-free wrapper a success path
// may also run through.
func ResourceError(err error) error {
	return &resourceError{err: err}
}

// IsResourceError reports whether err was marked by [ResourceError].
func IsResourceError(err error) bool {
	var re *resourceError
	return errors.As(err, &re)
}

// IsIOTimeout reports whether err is an I/O timeout — a read/write deadline the
// TRANSPORT hit, meaning the peer did not answer within the connection's
// budget. It recognizes os.ErrDeadlineExceeded and the net.Error Timeout()
// contract, so a net.OpError or a wrapped deadline surfaces here.
//
// It deliberately does NOT match context.DeadlineExceeded: a context deadline
// is a caller or chain signal (who stopped waiting), not evidence about the
// peer. [RoundErrorType] names that case separately. (context.DeadlineExceeded
// itself implements net.Error with Timeout()==true, so the exclusion must come
// before the Timeout() branch.)
func IsIOTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// ClassifyStatus maps a call's return error onto the status axis go-spring uses
// everywhere else — `ok` / `error`, the two words a cross-family query joins on.
// A protected call is not a special case: it reports the same two values as an
// inbound request or a discovery lookup.
func ClassifyStatus(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

// ErrorType is the closed vocabulary of failure classes the chain reports. It
// is a named string type (not a bare string) so a signature says what the word
// is, and so the words live in ONE place: the telemetry attributes are the
// string form of these constants.
type ErrorType string

const (
	// ErrorTypeNone is the zero value: no error. It is the ONLY empty value —
	// both levels classify every non-nil error, so "" cannot be mistaken for
	// "unclassified".
	ErrorTypeNone ErrorType = ""

	// ErrorTypeRateLimited means a rate-limit gate refused the call.
	ErrorTypeRateLimited ErrorType = "rate_limited"
	// ErrorTypeCircuitOpen means the circuit breaker was open.
	ErrorTypeCircuitOpen ErrorType = "circuit_open"
	// ErrorTypeBulkheadFull means the isolation pool had no capacity.
	ErrorTypeBulkheadFull ErrorType = "bulkhead_full"
	// ErrorTypeRetryExceeded means the retry budget was exhausted.
	ErrorTypeRetryExceeded ErrorType = "retry_exceeded"
	// ErrorTypeNoEndpoints means no peer could be addressed.
	ErrorTypeNoEndpoints ErrorType = "no_endpoints"
	// ErrorTypeCallTimeout means the chain's OWN timeout budget elapsed
	// ([ErrCallTimeout]); a caller deadline is [ErrorTypeContextCanceled], and a
	// peer's own timeout carries no provenance and stays [ErrorTypeUnknown].
	ErrorTypeCallTimeout ErrorType = "call_timeout"

	// ErrorTypeIOTimeout means the TRANSPORT hit a read/write deadline: the peer
	// did not answer within the connection's budget (the fault `timeout` kind
	// surfaces as one). Unlike [ErrorTypeCallTimeout] (the chain's own budget) and
	// [ErrorTypeContextCanceled] (the caller walking away), this is evidence
	// about the peer and is counted against it.
	ErrorTypeIOTimeout ErrorType = "io_timeout"

	// ErrorTypeConnectFailed names a failure to ESTABLISH the connection itself —
	// refused, unreachable, TLS handshake broke — the errors a dial site marks
	// with [ConnectError]. It is distinct from a call that reached the peer and
	// failed (that one is [ErrorTypeUnknown]; the protocol's own attribute
	// carries the detail).
	ErrorTypeConnectFailed ErrorType = "connect_failed"

	// ErrorTypeResourceExhausted names a refusal by one of OUR resources — a
	// pool that is closed or exhausted, an acquire that timed out — the errors
	// marked with [ResourceError]. Nothing was sent, so it says nothing about the
	// peer; it mirrors gRPC's RESOURCE_EXHAUSTED and [RoundOutcome]'s
	// ResourceExhausted.
	ErrorTypeResourceExhausted ErrorType = "resource_exhausted"

	// ErrorTypeContextCanceled names a call whose CONTEXT was canceled —
	// typically the caller walked away or its deadline elapsed. It is not a
	// downstream failure: the dependency was never given the chance to answer,
	// so counting it against the dependency would be wrong.
	ErrorTypeContextCanceled ErrorType = "context_canceled"

	// ErrorTypeUnknown is the residual: a non-nil error the chain could not
	// attribute to a gate, a connection failure, an I/O timeout or the caller. It deliberately
	// says nothing about blame — the error carries no provenance, so calling it
	// "downstream" would assert something the chain cannot prove. Protocol-level
	// detail belongs to the family's own attribute (status code, SQLSTATE, ...).
	// It corresponds to OTel's `_OTHER` fallback.
	ErrorTypeUnknown ErrorType = "unknown"
)

// RoundErrorType names the class of an error ONE ROUND produced, on the
// vocabulary [Round.ErrorType] carries. It is TOTAL: nil is [ErrorTypeNone],
// a gate sentinel is its gate word, a [ConnectError]-marked error is
// [ErrorTypeConnectFailed], a [ResourceError]-marked error is
// [ErrorTypeResourceExhausted], an [ErrCallTimeout]-marked error is
// [ErrorTypeCallTimeout], an I/O timeout ([IsIOTimeout]) is
// [ErrorTypeIOTimeout], a canceled context is [ErrorTypeContextCanceled], and anything
// else is [ErrorTypeUnknown] — including a plain [context.DeadlineExceeded]
// whose origin the chain cannot prove (a caller's deadline, or a peer's own
// timeout). [CallErrorType] refines the caller case with the caller's context.
//
// Whether the call succeeded is a different question, and [ClassifyStatus]
// answers it: `status` is the shared axis every family reports on, while this is
// the family-specific key that says what went wrong. The split is the same one
// as `status` beside `rpc.grpc.status_code`.
//
// It lives here, beside the sentinels whose errors it classifies, so an engine
// and the emitter both read it from the owner of the vocabulary rather than the
// emitter owning a word that is really the governance layer's.
func RoundErrorType(err error) ErrorType {
	switch {
	case err == nil:
		return ErrorTypeNone
	case errors.Is(err, ErrRateLimited):
		return ErrorTypeRateLimited
	case errors.Is(err, ErrCircuitOpen):
		return ErrorTypeCircuitOpen
	case errors.Is(err, ErrBulkheadFull):
		return ErrorTypeBulkheadFull
	case errors.Is(err, ErrRetryExceeded):
		return ErrorTypeRetryExceeded
	case errors.Is(err, ErrNoEndpoints):
		return ErrorTypeNoEndpoints
	case IsConnectError(err):
		return ErrorTypeConnectFailed
	case IsResourceError(err):
		return ErrorTypeResourceExhausted
	case errors.Is(err, ErrCallTimeout):
		return ErrorTypeCallTimeout
	case IsIOTimeout(err):
		return ErrorTypeIOTimeout
	case errors.Is(err, context.Canceled):
		return ErrorTypeContextCanceled
	default:
		return ErrorTypeUnknown
	}
}

// CallErrorType names the FINAL error type of one call, on the same closed
// vocabulary: the gate words above plus [ErrorTypeConnectFailed],
// [ErrorTypeResourceExhausted], [ErrorTypeContextCanceled], [ErrorTypeUnknown] and
// [ErrorTypeNone]. Empty means success.
//
// It is the CALL-level counterpart of [RoundErrorType] and differs in exactly
// one way: it can see the caller's context, so a deadline the CALLER set (err
// is ctx.Err(), the call was abandoned) is [ErrorTypeContextCanceled] even though the
// error value alone is a plain [context.DeadlineExceeded]. The chain's own
// budget is already marked [ErrCallTimeout] where it is produced, and every other
// word is the round-level answer.
func CallErrorType(ctx context.Context, err error) ErrorType {
	if err == nil {
		return ErrorTypeNone
	}
	if ctx != nil && ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return ErrorTypeContextCanceled
	}
	return RoundErrorType(err)
}
