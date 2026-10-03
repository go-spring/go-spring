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
// starter plugs the single [chain.Executor] seam into its own request hook
// (http.RoundTripper, redis.Hook, gorm plugin, ...).
//
// The abstraction is split from its implementations exactly like
// [go-spring.org/cloud/discovery]:
//
//   - [ClientPolicy] is a backend-neutral, declarative description of the desired
//     protection.
//   - [Driver] turns a ClientPolicy into a live [chain.Executor]. A company (or the bundled
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
//	policy.go, server_policy.go        the two declarative specs (ClientPolicy for
//	                                   outbound, ServerPolicy for inbound) + the
//	                                   service label scheme; the rule-document
//	                                   binding lives in the governance package
//	executor.go, manager.go            the runtime: the two seams, the driver
//	                                   directory, per-direction registries
//	executor_default*.go               the bundled driver's OUTBOUND engine: assembly +
//	                                    its breaker / bulkhead / retry stages
//	server_executor_default.go              the bundled driver's INBOUND engine: the same
//	                                    stages minus retry
//	ratelimit.go                       the rate-limit stage: the counters seam + algorithms
//	adapter_run.go, adapter_http.go,   the outbound seams: command-style Run,
//	adapter_dial.go                     http.RoundTripper, net dialer
//	observe.go                          the observability wrappers around either executor
//
// This file holds the runtime protection seams themselves: the [chain.Executor] and
// [chain.Executor] interfaces (the single seams every adapter and inbound
// middleware call), the [Driver] seam that builds them, the rejection sentinels
// implementations return, and the [Fallback] degradation helper.

package resilience

import (
	"go-spring.org/cloud/chain"
	"go-spring.org/stdlib/errutil"
)

// DefaultDriverName is the name the bundled [NewDefaultDriver] backend answers
// to, and the name the governance center falls back to when its driver
// directory holds no entry for the configured name.
const DefaultDriverName = "default"

// Driver builds an [chain.Executor] from a [ClientPolicy] for OUTBOUND calls and an
// [chain.Executor] from a [ServerPolicy] for INBOUND requests. Backends implement
// it and are contributed to the container as a bean named after the backend (e.g.
// "sentinel"), exported as a [Driver] so name-keyed directory injection finds
// them: the container is the driver directory, and the resilience manager is
// built over the map of them the center resolves the configured name in.
// This package holds no registry of its own, so selecting a backend by name is
// the caller's job.
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
	// it for life (see [chain.Executor]).
	NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error)

	// NewServerExecutor builds the inbound executor protecting the inbound route
	// service under p. The service is the route label an inbound middleware passes
	// (e.g. "gin:0.0.0.0:8080"), bound for life like [Driver.NewClientExecutor]'s. A
	// backend with no inbound story of its own returns an executor built from
	// p.AsPolicy()'s projection — see the bundled driver.
	NewServerExecutor(service string, p ServerPolicy) (chain.Executor, error)
}

// NewDefaultDriver returns the bundled driver: a self-contained [chain.Executor]
// builder with no third-party dependencies, so the framework is usable out of
// the box and in tests. Production deployments select a richer driver (for
// example sentinel-golang, in its own module) purely by changing the configured
// driver name — the [chain.Executor] seam and every adapter stay put.
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

func (d defaultDriver) NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	e := newClientExecutor(service, p, d.counters)
	return e, nil
}

// NewServerExecutor builds the inbound engine (server_executor_default.go) for one
// inbound route and hands it to the caller.
func (d defaultDriver) NewServerExecutor(service string, p ServerPolicy) (chain.Executor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	return newServerExecutor(service, p, d.counters), nil
}
