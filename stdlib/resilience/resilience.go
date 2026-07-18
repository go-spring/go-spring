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

// Package resilience defines a framework-agnostic, zero-dependency abstraction
// for client-side fault tolerance: rate limiting, circuit breaking, retry and
// per-attempt timeout.
//
// It answers one question for outbound calls (HTTP, RPC, DB, cache, ...):
// "before I let this operation run, should it be throttled, short-circuited or
// retried?". It says nothing about which library makes the call — each client
// starter plugs the single [Executor] seam into its own request hook
// (http.RoundTripper, redis.Hook, gorm plugin, ...).
//
// The abstraction is split from its implementations exactly like
// [go-spring.org/stdlib/discovery]:
//
//   - [Policy] is a backend-neutral, declarative description of the desired
//     protection.
//   - [Driver] turns a Policy into a live [Executor]. A company (or the bundled
//     builtin) implements Driver once and registers it via [RegisterDriver];
//     callers select it by name through [MustGetDriver] with no per-component
//     adaptation.
//   - The bundled "default" driver (see builtin.go) has zero third-party
//     dependencies so the framework runs standalone; the recommended
//     production driver (sentinel-golang) lives in its own module and registers
//     itself on blank import.
package resilience

import (
	"context"
	"errors"
	"time"
)

// ErrRateLimited is returned (or wrapped) by an [Executor] when an operation is
// rejected because the configured rate limit is exceeded.
var ErrRateLimited = errors.New("resilience: rate limited")

// ErrCircuitOpen is returned (or wrapped) by an [Executor] when an operation is
// rejected because the circuit breaker for its resource is open.
var ErrCircuitOpen = errors.New("resilience: circuit open")

// Policy is a backend-neutral description of the protection wanted for a set of
// operations. Each [Driver] maps these knobs onto its own primitives (the
// builtin driver reads them directly; sentinel-golang translates them into its
// flow/circuit-breaker rules). A zero Policy protects nothing — every stage is
// opt-in, so an unset Policy makes [Executor.Execute] a transparent pass-through.
type Policy struct {
	// RateLimit caps sustained throughput in operations per second. 0 disables
	// rate limiting.
	RateLimit float64

	// Burst is the maximum number of operations allowed to exceed RateLimit
	// momentarily. It defaults to a small multiple of RateLimit when unset; it
	// is ignored when RateLimit is 0.
	Burst int

	// ErrorThreshold is the number of consecutive failures that trips the
	// circuit breaker open. 0 disables circuit breaking.
	ErrorThreshold int

	// OpenDuration is how long the circuit stays open before a trial request is
	// allowed through (half-open). Ignored when ErrorThreshold is 0; defaults to
	// a few seconds when unset.
	OpenDuration time.Duration

	// MaxRetries is the number of extra attempts after the first failure. 0
	// means a single attempt with no retry. Retries respect the circuit breaker
	// and rate limiter.
	MaxRetries int

	// Timeout bounds each individual attempt via a derived context. 0 means no
	// per-attempt timeout is imposed by the executor.
	Timeout time.Duration
}

// Executor runs operations under a [Policy]. It is the single seam every client
// adapter calls; implementations must be safe for concurrent use.
type Executor interface {
	// Execute runs fn under the policy, scoping rate-limiter and circuit-breaker
	// state to resource (typically a downstream service name). It returns
	// [ErrRateLimited] or [ErrCircuitOpen] when the call is rejected before fn
	// runs, or fn's own (final) error otherwise. The context passed to fn may be
	// a per-attempt timeout derived from ctx.
	Execute(ctx context.Context, resource string, fn func(context.Context) error) error

	// Close releases any background resources held by the executor (e.g. metric
	// pumps in a production driver). It is safe to call more than once.
	Close() error
}

// Driver builds an [Executor] from a [Policy]. Backends implement it and
// register under a name via [RegisterDriver].
type Driver interface {
	NewExecutor(Policy) (Executor, error)
}
