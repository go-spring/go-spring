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
// operation through an [ClientExecutor], [Tolerate] declares client-specific
// sentinel errors, and [IsRejection] identifies protection rejections.

package resilience

import (
	"context"
	"errors"
)

// IsRejection reports whether err is one of the resilience protection rejects:
// rate-limited, circuit-open, bulkhead-full, or retry-budget-exceeded. Every
// client adapter checks these sentinels after an Execute to decide whether a
// rejection should be surfaced verbatim (rather than treated as the operation's
// own error), so the check is shared here rather than copy-pasted per starter.
func IsRejection(err error) bool {
	return errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrCircuitOpen) ||
		errors.Is(err, ErrBulkheadFull) ||
		errors.Is(err, ErrRetryBudgetExceeded)
}

// Option adjusts how [Run] classifies a call's outcome.
type Option func(*options)

type options struct {
	// nilErr classifies a returned error as "not a fault for resilience
	// purposes"; defaults to "every error is a fault".
	nilErr func(error) bool
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
// Multiple Tolerate options stack (OR). Only sentinel matching (errors.Is) is
// offered — no caller needs a custom predicate; add a TolerateFunc-style
// option if one ever does.
func Tolerate(sentinel error) Option {
	return func(o *options) {
		prev := o.nilErr
		o.nilErr = func(e error) bool {
			return prev(e) || errors.Is(e, sentinel)
		}
	}
}

// Run executes call under exec. Protection is scoped to the service exec was built
// for, so the caller names nothing.
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
// The return is the operation's own value (a command reply, a fetched item, ...)
// plus the error to propagate. Callers whose operation yields nothing pass
// struct{}{} for T and discard the result. Semantics:
//
//   - A resilience rejection ([ErrRateLimited] / [ErrCircuitOpen] /
//     [ErrBulkheadFull]) is returned verbatim.
//   - On a normal downstream failure, the returned error is call's own error.
//   - When a fault injector short-circuited the attempt before call ran (call's
//     error is nil but the executor still returns an injected error), the
//     injected error is preferred so it is not silently swallowed as success.
func Run[T any](ctx context.Context, exec ClientExecutor,
	call func(context.Context) (T, error), opts ...Option) (T, error) {

	o := newOptions(opts)
	nilErr := o.nilErr

	var result T
	if exec == nil {
		return call(ctx)
	}
	var callErr error
	execErr := exec.Execute(ctx, func(attemptCtx context.Context) error {
		result, callErr = call(attemptCtx)
		if callErr != nil && !nilErr(callErr) {
			return callErr // a real failure feeds the breaker/retry
		}
		return nil // success or a tolerated sentinel (cache miss / "no rows")
	})
	if execErr != nil {
		if IsRejection(execErr) {
			return result, execErr // rejected before (or around) the call
		}
		// On the normal failure path execErr equals callErr (the closure returned
		// it). They diverge only when the closure never ran — e.g. a fault injector
		// short-circuited the attempt before reaching call, leaving callErr nil
		// while the executor still returns the injected error. Prefer execErr so
		// the failure is not silently swallowed as success.
		if callErr == nil {
			return result, execErr
		}
		return result, callErr
	}
	return result, callErr
}
