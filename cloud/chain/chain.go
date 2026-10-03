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

// Package chain defines the shape of a governed call chain: the one interface
// every traffic direction runs through. A chain's Executor wraps a caller's
// operation with whatever the deployment governing that call asked for — rate
// limiting, circuit breaking, retry, inbound protection — and with the observability
// (span, metrics, access log) that must surround every governed call, which is
// why the two travel as one concept rather than as separable layers.
//
// The package is deliberately a leaf: it depends on nothing but the standard
// library, so the many packages that only need to HOLD a chain (a fault
// injector, a batch runner, a transaction coordinator) can depend on the shape
// without depending on any engine that builds one. The engines live in
// go-spring.org/cloud/resilience; this package is the seam they are selected
// through.
package chain

import (
	"context"
	"errors"
)

// Executor runs one operation under the governance chain built for the ONE
// service or route it was constructed for. The chain — not the caller — decides
// whether the operation is throttled, short-circuited, retried or simply
// observed; the caller hands over fn and gets back the final outcome.
//
// Binding the service at construction rather than taking it per call is what
// lets every link in the chain keep its state the same way: per-service
// protection state belongs to the executor, exactly like its breaker, bulkhead
// and retry budget.
//
// The policy is construction-bound: a threshold change is a NEW executor from
// the driver, not a mutation of a running one — so every stage starts clean,
// with no second way to change a policy and no refresh call to thread through
// every wrapper. Implementations must be safe for concurrent use.
type Executor interface {
	// Execute runs fn under the chain. It returns the chain's rejection (rate
	// limited, circuit open, bulkhead full — see the engine's sentinels) when
	// the call is rejected before fn runs, or fn's own (final) error otherwise.
	// The context passed to fn may be a timeout derived from ctx.
	Execute(ctx context.Context, fn func(context.Context) error) error

	// Close releases any background resources held by the executor (e.g. metric
	// pumps in a production engine). It is safe to call more than once.
	Close() error
}

// The rejection sentinels a chain returns from Execute when it refuses to run
// fn at all. They live here — not in any engine — because they are part of the
// chain contract: every engine, and every observer of one, speaks the same four
// words. Wrap or compare them with errors.Is.
var (
	// ErrRateLimited is the chain's verdict that the configured rate limit is
	// exceeded.
	ErrRateLimited = errors.New("resilience: rate limited")

	// ErrCircuitOpen is the chain's verdict that the circuit breaker for the
	// executor's service is open.
	ErrCircuitOpen = errors.New("resilience: circuit open")

	// ErrBulkheadFull is the chain's verdict that the service already has the
	// maximum number of concurrent in-flight operations allowed by the bulkhead.
	ErrBulkheadFull = errors.New("resilience: bulkhead full")

	// ErrRetryBudgetExceeded is the chain's verdict that a RETRY attempt was
	// rejected because the executor-wide in-flight retry cap is exhausted — the
	// process-level guard against retry amplification. The first attempt of a
	// call never takes budget, so it only ever replaces a retry, not the initial
	// call.
	ErrRetryBudgetExceeded = errors.New("resilience: retry budget exceeded")
)
