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
// starter plugs the single [ClientExecutor] seam into its own request hook
// (http.RoundTripper, redis.Hook, gorm plugin, ...).
//
// The abstraction is split from its implementations exactly like
// [go-spring.org/cloud/discovery]:
//
//   - [ClientPolicy] is a backend-neutral, declarative description of the desired
//     protection.
//   - [Driver] turns a ClientPolicy into a live [ClientExecutor]. A company (or the bundled
//     default) implements Driver once and contributes it to the container as a
//     named bean; the governance center looks the configured name up in the
//     directory of driver beans, so callers select a backend by name with no
//     per-component adaptation.
//   - The bundled "default" driver (see driver.go) has zero third-party
//     dependencies so the framework runs standalone; the recommended
//     production driver (sentinel-golang) lives in its own module and
//     contributes itself as a bean on blank import.
//
// The package's files map to the model, top-down:
//
//	policy.go                          the declarative spec (ClientPolicy) + the service
//	                                   label scheme; the rule-document binding lives
//	                                   in the governance package (its center.go)
//	executor.go, manager.go            the runtime: seams, the bundled engine, hot reload
//	executor_default*.go                the bundled default driver: assembly + its
//	                                    breaker / bulkhead / retry organs
//	ratelimit.go                        the rate-limit stage: the counters seam + algorithms
//	adapter_run.go, adapter_http.go,   the client-side seams: command-style Run,
//	adapter_dial.go                     http.RoundTripper, net dialer
//	observe.go                          the observability wrapper around any ClientExecutor
//
// This file holds the runtime protection seams themselves: the [ClientExecutor]
// interface (the single seam every client adapter calls), the [Driver] seam that
// builds one, the rejection sentinels implementations return, and the [Fallback]
// degradation helper.

package resilience

import (
	"context"
	"errors"

	"go-spring.org/stdlib/errutil"
)

// ErrRateLimited is returned (or wrapped) by an [ClientExecutor] when an operation is
// rejected because the configured rate limit is exceeded.
var ErrRateLimited = errors.New("resilience: rate limited")

// ErrCircuitOpen is returned (or wrapped) by an [ClientExecutor] when an operation is
// rejected because the circuit breaker for its service is open.
var ErrCircuitOpen = errors.New("resilience: circuit open")

// ErrBulkheadFull is returned (or wrapped) by an [ClientExecutor] when an operation is
// rejected because the service already has the maximum number of concurrent
// in-flight operations allowed by the bulkhead.
var ErrBulkheadFull = errors.New("resilience: bulkhead full")

// ErrRetryBudgetExceeded is returned by an [ClientExecutor] when a RETRY attempt is
// rejected because the executor-wide in-flight retry cap ([ClientPolicy.RetryBudget])
// is exhausted — the process-level guard against retry amplification. The first
// attempt of a call never takes budget, so this error only ever replaces a
// retry, not the initial call.
var ErrRetryBudgetExceeded = errors.New("resilience: retry budget exceeded")

// ClientExecutor runs operations under a [ClientPolicy] for the ONE service it was built
// for (see [Driver.NewClientExecutor]). Binding the service at construction rather
// than taking it per call is what lets every stage keep its state the same way:
// per-service protection state belongs to the executor, exactly like its
// breaker, bulkhead and retry budget — the rate-limit stage keeps its budget
// there too, unless the driver was handed a [Counters] store to count in
// instead. Implementations must be safe for concurrent use.
type ClientExecutor interface {
	// Execute runs fn under the policy. It returns [ErrRateLimited] or
	// [ErrCircuitOpen] when the call is rejected before fn runs, or fn's own
	// (final) error otherwise. The context passed to fn may be a per-attempt
	// timeout derived from ctx.
	Execute(ctx context.Context, fn func(context.Context) error) error

	// Close releases any background resources held by the executor (e.g. metric
	// pumps in a production driver). It is safe to call more than once.
	Close() error

	// Refresh adopts p as the new policy at runtime — hot-reloading
	// rate/breaker/bulkhead/retry thresholds without rebuilding the bean.
	// Refreshing resets per-service protection state (breaker counters, token
	// buckets, bulkhead slots): a new policy starts clean, which is the
	// intended semantic of "the threshold changed". Every driver implements
	// this so adapters driven by a config binding can call it directly
	// when the bound policy changes.
	Refresh(p ClientPolicy) error
}

// ServerExecutor runs INBOUND requests under a [ServerPolicy] for the ONE route
// it was built for (see [Driver.NewServerExecutor]). It is the inbound counterpart of
// [ClientExecutor]: the same Execute/Close contract, but refreshed with the model it was
// built from, so a backend that maps a [ServerPolicy] onto its own inbound
// primitives keeps that mapping on hot reload instead of being handed a foreign
// [ClientPolicy]. Implementations must be safe for concurrent use.
type ServerExecutor interface {
	// Execute runs fn under the admission model. It returns [ErrRateLimited],
	// [ErrCircuitOpen] or [ErrBulkheadFull] when the request is rejected before fn
	// runs, or fn's own error otherwise. There is no retry stage inbound, so the
	// context passed to fn carries at most the handling budget.
	Execute(ctx context.Context, fn func(context.Context) error) error

	// Close releases any background resources held by the executor (e.g. metric
	// pumps in a production driver). It is safe to call more than once.
	Close() error

	// Refresh adopts a as the new admission model at runtime —
	// hot-reloading rate/breaker/bulkhead thresholds without rebuilding the bean.
	// Refreshing resets per-route admission state (breaker counters, token buckets,
	// bulkhead slots), the same "the threshold changed, so start clean" semantic as
	// [ClientExecutor.Refresh].
	Refresh(p ServerPolicy) error
}

// DefaultDriverName is the name the bundled [NewDefaultDriver] backend answers
// to, and the name the governance center falls back to when its driver
// directory holds no entry for the configured name.
const DefaultDriverName = "default"

// Driver builds an [ClientExecutor] from a [ClientPolicy] for OUTBOUND calls and an
// [ServerExecutor] from a [ServerPolicy] for INBOUND requests. Backends implement
// it and are contributed to the container as a bean named after the backend (e.g.
// "sentinel"), exported as a [Driver] so name-keyed directory injection finds
// them: the container is the driver directory, and the governance wiring bean
// collects the beans into a map the center resolves the configured name in.
// This package holds no registry of its own, so selecting a backend by name is
// the caller's job — see the discovery package for the same shape.
//
// TWO methods rather than one direction-agnostic one, because the two directions
// do not share a model: a backend maps [ClientPolicy] onto its outbound primitives and
// [ServerPolicy] onto its inbound ones, and a sentinel-style backend's inbound
// primitives (a system-load rule, a hotspot rule) have no outbound expression at
// all. One object still answers for BOTH directions, so `driver=sentinel` remains
// one key that switches the whole process — nothing is selected twice.
type Driver interface {
	// NewClientExecutor builds the executor protecting service under p. The service is
	// part of the request, not of the call: the executor it returns is bound to
	// it for life (see [ClientExecutor]).
	NewClientExecutor(service string, p ClientPolicy) (ClientExecutor, error)

	// NewServerExecutor builds the admission executor protecting the inbound route
	// service under p. The service is the route label an inbound middleware passes
	// (e.g. "gin:0.0.0.0:8080"), bound for life like [Driver.NewClientExecutor]'s. A
	// backend with no inbound story of its own returns an executor built from
	// p.AsPolicy()'s projection — see the bundled driver.
	NewServerExecutor(service string, p ServerPolicy) (ServerExecutor, error)
}

// NewDefaultDriver returns the bundled driver: a self-contained [ClientExecutor]
// builder with no third-party dependencies, so the framework is usable out of
// the box and in tests. Production deployments select a richer driver (for
// example sentinel-golang, in its own module) purely by changing the configured
// driver name — the [ClientExecutor] seam and every adapter stay put.
//
// counters is the rate-limit store the executors it builds count in. nil — the
// default — leaves each executor counting for its service in a bucket of its
// own, which is how every other stage keeps its state and is enough for one
// budget per service (the manager builds one executor per label, so every
// caller of that label spends the same budget). Pass a store instead (the
// wiring injects one when a backend starter contributed it, Redis typically)
// and every executor this driver builds spends that one — which is what extends
// a single budget past the process, to every replica. Sharing breadth is
// therefore a property of the store, not of the driver.
func NewDefaultDriver(counters Counters) Driver {
	return defaultDriver{counters: counters}
}

// defaultDriver is the bundled [Driver]: it builds a [defaultExecutor] from a
// service and a [ClientPolicy]. The executor itself (the stage pipeline + the
// per-service breaker/bulkhead state) lives in executor_default.go. counters is
// an optional override of where those executors keep their rate-limit state; nil
// leaves each one counting in a bucket of its own (see [NewDefaultDriver]).
type defaultDriver struct {
	counters Counters
}

func (d defaultDriver) NewClientExecutor(service string, p ClientPolicy) (ClientExecutor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	e := newDefaultExecutor(service, p, d.counters)
	return e, nil
}

// NewServerExecutor builds an inbound admission executor by reusing the same engine
// [defaultDriver.NewClientExecutor] does. That is legitimate for THIS driver because it
// is a pure implementation of the two models: it has no inbound primitives of its
// own that [ServerPolicy] cannot express, so the projection is exact and nothing is
// lost. A backend that does have them maps ServerPolicy natively instead of
// projecting.
func (d defaultDriver) NewServerExecutor(service string, p ServerPolicy) (ServerExecutor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	return serverExecutorAdapter{newDefaultExecutor(service, p.AsPolicy(), d.counters)}, nil
}

// serverExecutorAdapter adapts the bundled engine — which is defined over [ClientPolicy] —
// to the inbound seam, whose model is [ServerPolicy]. It exists only because the
// refresh seam is model-typed (see [ServerExecutor]): the executor itself is
// the same type, and this wrapper just re-projects the admission model on each
// refresh, so the engine never learns that there are two directions at all.
type serverExecutorAdapter struct {
	*defaultExecutor
}

func (e serverExecutorAdapter) Refresh(p ServerPolicy) error {
	return e.defaultExecutor.Refresh(p.AsPolicy())
}

// Fallback runs fn through exec and, when the operation is rejected (rate
// limited, circuit open, bulkhead full) or fails after all retries, invokes
// degrade to produce a graceful result instead of surfacing the error. It is
// the degradation stage of the framework and composes with any [ClientExecutor]
// regardless of driver: degrade receives the triggering error so it can serve
// cached data for [ErrCircuitOpen] yet propagate a genuine bug, for example.
//
// degrade's own error (or nil) becomes the final result. When exec is nil the
// call is a transparent pass-through: fn runs once and its error, if any, still
// reaches degrade, so wiring stays a no-op until a policy is configured.
func Fallback(ctx context.Context, exec ClientExecutor,
	fn func(context.Context) error, degrade func(context.Context, error) error) error {
	var err error
	if exec == nil {
		err = fn(ctx)
	} else {
		err = exec.Execute(ctx, fn)
	}
	if err == nil {
		return nil
	}
	return degrade(ctx, err)
}
