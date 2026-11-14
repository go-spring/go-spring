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

// adapter_run.go is the command-style client seam: [Run] threads one client
// operation through an [chain.Executor], [Tolerate] declares client-specific
// sentinel errors, and [IsRejection] identifies protection rejections.

package resilience

import (
	"context"
	"errors"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// IsRejection reports whether err is one of the resilience protection rejects:
// rate-limited, circuit-open, bulkhead-full, or retry-budget-exceeded. Every
// client adapter checks these sentinels after an Execute to decide whether a
// rejection should be surfaced verbatim (rather than treated as the operation's
// own error), so the check is shared here rather than copy-pasted per starter.
func IsRejection(err error) bool {
	return errors.Is(err, chain.ErrRateLimited) ||
		errors.Is(err, chain.ErrCircuitOpen) ||
		errors.Is(err, chain.ErrBulkheadFull) ||
		errors.Is(err, chain.ErrRetryExceeded)
}

// Option adjusts how [Run] classifies a call's outcome.
type Option func(*options)

type options struct {
	// nilErr classifies a returned error as "not a fault for resilience
	// purposes"; defaults to "every error is a fault".
	nilErr func(error) bool
	// retryOn classifies a returned error as eligible for retry; nil means
	// none are (the whitelist default).
	retryOn func(error) bool
}

// newOptions applies opts over the defaults.
func newOptions(opts []Option) options {
	o := options{nilErr: func(error) bool { return false }}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Tolerate marks a sentinel error (matched with errors.Is) as a normal
// outcome rather than a fault. Such an error means the downstream answered
// the protocol correctly — a cache miss / "no rows" — so it must neither trip
// the breaker nor trigger a retry, while still being returned to the caller
// verbatim. Without this, a service with a low cache hit rate would count its
// misses as failures and trip the breaker against a healthy downstream.
//
// Which errors count as normal is client-specific, so each caller declares its
// own sentinels: redis.Nil, gorm.ErrRecordNotFound, memcache.ErrCacheMiss.
// Multiple Tolerate options stack (OR). Use [TolerateFunc] when the normal set
// is a PROPERTY of the error rather than a fixed value.
func Tolerate(sentinel error) Option {
	return func(o *options) {
		prev := o.nilErr
		o.nilErr = func(e error) bool {
			return prev(e) || errors.Is(e, sentinel)
		}
	}
}

// TolerateFunc is [Tolerate] for a class of errors: pred answers whether err is
// a normal outcome. It exists because some clients express "the server answered
// definitively" as a family of values (an SMTP permanent failure, a Cypher
// client error) that no single sentinel matches.
//
// The contract is the same as [Tolerate]: a matched error is returned verbatim
// but counts as neither a breaker failure nor a retry trigger. pred is only
// called for a non-nil error.
func TolerateFunc(pred func(error) bool) Option {
	return func(o *options) {
		prev := o.nilErr
		o.nilErr = func(e error) bool {
			return prev(e) || pred(e)
		}
	}
}

// RetryOn marks errors for which pred returns true as eligible for retry.
//
// Retry is a WHITELIST: without a matching option nothing is retried, because an
// unclassified failure is not known to be transient and a retry may repeat a
// side effect. This option is how a client declares its own transient class —
// the errors of a third-party library cannot implement [Retryable] themselves.
// Multiple RetryOn options stack (OR), and a [Tolerate]/[TolerateFunc] match
// wins: a tolerated error is a normal outcome and is never retried.
//
// The caller-visible error is unchanged: the retry decision is carried by an
// internal wrapper handed to the executor only, and [Run] still returns the
// original error.
func RetryOn(pred func(error) bool) Option {
	return func(o *options) {
		prev := o.retryOn
		o.retryOn = func(e error) bool {
			return (prev != nil && prev(e)) || pred(e)
		}
	}
}

// retryableError carries "this failure is transient" to the executor's retry
// loop without altering what the caller receives: [Run] returns the original
// error, never this wrapper.
type retryableError struct{ err error }

func (e retryableError) Error() string   { return e.err.Error() }
func (e retryableError) Unwrap() error   { return e.err }
func (e retryableError) Retryable() bool { return true }

// Run executes fn under exec for the call described by call. Protection is
// scoped to the service exec was built for, but call must still STATE that
// service ([chain.Operation.Service]): layers above exec — the fault layer
// among them — read the operation, not the executor, and a call that names no
// service matches only the rules that name none.
// It is the single "run one client operation through the resilience executor"
// body that every client adapter (redis.Hook, gorm callback, connection wrapper,
// http.RoundTripper, ...) otherwise copy-pastes, extracted so the nil-as-success
// and rejection/fault translation semantics live in exactly one place.
//
// A returned error matched by a [Tolerate] option (a cache miss / "no rows")
// is treated as success for resilience purposes — it does not count toward
// the breaker or trigger a retry — yet is still returned to the caller
// verbatim, so the caller's errors.Is checks keep working. Clients with no
// such sentinel pass no option.
//
// A returned error matched by a [RetryOn] option is presented to the executor
// as retry-eligible: the label's configured schedule decides whether another
// attempt runs. The caller still receives the original error, never the
// internal marker. A [Tolerate] match wins (a normal outcome is never retried).
//
// The return is the operation's own value (a command reply, a fetched item, ...)
// plus the error to propagate. Callers whose operation yields nothing pass
// struct{}{} for T and discard the result. Semantics:
//
//   - A resilience rejection ([chain.ErrRateLimited] / [chain.ErrCircuitOpen] /
//     [chain.ErrBulkheadFull]) is returned verbatim.
//   - On a normal downstream failure, the returned error is fn's own error.
//   - When a fault injector short-circuited the attempt before fn ran (fn's
//     error is nil but the executor still returns an injected error), the
//     injected error is preferred so it is not silently swallowed as success.
func Run[T any](ctx context.Context, exec chain.Executor, call chain.Operation,
	fn func(context.Context, chain.Peer) (T, error), opts ...Option) (T, error) {

	o := newOptions(opts)
	nilErr, retryOn := o.nilErr, o.retryOn

	var result T
	if exec == nil {
		return fn(ctx, chain.Peer{})
	}
	var callErr error
	execErr := exec.Execute(ctx, call, new(chain.Recorder), func(attemptCtx context.Context, rd *chain.Round) error {
		var peer chain.Peer // the round's routing handoff; zero when no round ran
		if rd != nil {
			peer = rd.Peer
		}
		result, callErr = fn(attemptCtx, peer)
		if callErr == nil || nilErr(callErr) {
			return nil // success or a tolerated sentinel (cache miss / "no rows")
		}
		if retryOn != nil && retryOn(callErr) {
			// The executor sees a retry-eligible error; the caller still gets
			// callErr verbatim (see Run's return below).
			return retryableError{callErr}
		}
		return callErr // a real failure feeds the breaker
	})
	if execErr != nil {
		if IsRejection(execErr) {
			return result, execErr // rejected before (or around) the call
		}
		// On the normal failure path execErr equals callErr (the closure returned
		// it). They diverge only when the closure never ran — e.g. a fault injector
		// short-circuited the attempt before reaching fn, leaving callErr nil
		// while the executor still returns the injected error. Prefer execErr so
		// the failure is not silently swallowed as success.
		if callErr == nil {
			return result, execErr
		}
		return result, callErr
	}
	return result, callErr
}
