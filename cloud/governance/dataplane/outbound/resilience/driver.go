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
// [go-spring.org/cloud/governance/capability/discovery]:
//
//   - [Limits], [BreakerPolicy] and [RateLimitPolicy] are backend-neutral,
//     declarative descriptions of the desired protection, one per capability.
//   - [Driver] builds a live [chain.Executor] for a label. A company (or the
//     bundled default) implements Driver once and contributes it to the container
//     as a named bean; the resilience manager injects the bean named by
//     ${spring.resilience.driver} (a static startup key), so callers select a
//     backend by name with no per-component adaptation.
//   - The bundled "default" driver (see driver.go) has zero third-party
//     dependencies so the framework runs standalone; the recommended
//     production driver (sentinel-golang) lives in its own module and
//     contributes itself as a bean on blank import.
//
// The package's files map to the model, top-down:
//
//	policy.go                       the declarative outbound spec (Policy)
//	                                       + the service label scheme; the rule-document
//	                                       binding lives in the governance package
//	driver.go, manager.go           the runtime: the Driver seam and the
//	                                       per-label registry
//	executor.go                     the bundled driver's engine: the
//	                                       timeout / retry loop, its retry
//	                                       helper, and [Fallback]
//	ratelimit.go                           the outbound view of the shared
//	                                       rate-limit stage
//	breaker.go                             the outbound view of the shared breaker
//	adapter_run.go,                 the outbound seams: command-style Run and
//	adapter_http.go                 http.RoundTripper
//
// The breaker state machine lives in go-spring.org/cloud/governance/capability/breaker and the
// rate-limit vocabulary, budget and algorithms in
// go-spring.org/cloud/governance/capability/ratelimit — neutral leaf packages shared with inbound
// guard, so neither direction owns the other's implementation.
//
// This file holds the runtime seams themselves: the [chain.Executor] interface
// every adapter and inbound middleware calls, and the [Driver] seam that builds
// one. (The rejection sentinels live in the [chain] package; [Fallback] lives
// with the outbound engine in executor.go.)

package resilience

import (
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// DefaultDriverName is the name the bundled [NewDefaultDriver] backend answers
// to, and the value [DriverProperty] falls back to.
const DefaultDriverName = "default"

// DriverProperty is the STATIC config key that selects the [Driver] backend: its
// value names the [Driver] bean to build executors with (DefaultDriverName for the
// bundled engine, or a backend starter's own bean name such as "sentinel").
//
// It is read at wiring time by the manager's registration (see the starter),
// which injects the named bean — not pushed like a policy: the backend is fixed
// for the process's lifetime, because swapping it drops every executor's breaker
// and limiter state, a deploy-time act rather than a runtime one.
const DriverProperty = "spring.resilience.driver"

// Driver builds a [chain.Executor] for one OUTBOUND service label.
// Backends implement it and are contributed to the container as a bean named
// after the backend (e.g. "sentinel"), exported as a [Driver] so name-keyed
// directory injection finds them: the container is the driver directory, and
// the resilience manager is built over the map of them the center resolves the
// configured name in. This package holds no registry of its own, so selecting a
// backend by name is the caller's job.
//
// A Driver receives NO configuration: the policy an executor runs under is read
// from the [Manager] on every call. The manager is passed in because the driver
// is what knows the executor's concrete type, and an executor with no route to
// the manager would be unable to follow the config (or a hot reload). The
// driver must not retain it beyond construction.
//
// It is CLIENT-ONLY. Inbound requests are not a [chain.Executor] and do not use
// this seam: they run through [go-spring.org/cloud/governance/dataplane/inbound/admission], whose per-instance
// engine is fixed. A backend that once supplied an inbound engine
// (sentinel-golang) now contributes only its outbound executor; admission's bundled
// engine covers the inbound policy fields the framework exposes.
type Driver interface {
	// NewExecutor builds the executor protecting label. The label is part of the
	// request, not of the call: the executor it returns is bound to it for life
	// (see [chain.Executor]). m is the manager the executor asks for its policy;
	// it carries no configuration itself.
	NewExecutor(label string, m *Manager) (chain.Executor, error)
}

// NewDefaultDriver returns the bundled driver: a self-contained [chain.Executor]
// builder with no third-party dependencies, so the framework is usable out of
// the box and in tests. Production deployments select a richer driver (for
// example sentinel-golang, in its own module) purely by changing the configured
// driver name — the [chain.Executor] seam and every adapter stay put.
//
// limiter is the cluster-wide rate-limit store the executors it builds count in.
// nil — the default — leaves each executor counting for its service in a bucket
// of its own, which is how every other stage keeps its state and is enough for
// one budget per service (the manager builds one executor per label, so every
// caller of that label spends the same budget). Pass a store instead (the
// wiring injects one when a backend starter contributed it, Redis typically)
// and every executor this driver builds spends that one — which is what extends
// a single budget past the process, to every replica. Sharing breadth is
// therefore a property of the store, not of the driver.
func NewDefaultDriver(limiter ratelimit.ClusterLimiter) Driver {
	return defaultDriver{limiter: limiter}
}

// defaultDriver is the bundled [Driver]: it builds a [defaultExecutor] for a
// service. The executor itself (the stage pipeline + the per-service breaker
// state) lives in executor.go. limiter is an
// optional override of where those executors keep their rate-limit state; nil
// leaves each one counting in a bucket of its own (see [NewDefaultDriver]).
type defaultDriver struct {
	limiter ratelimit.ClusterLimiter
}

func (d defaultDriver) NewExecutor(label string, m *Manager) (chain.Executor, error) {
	if m == nil {
		return nil, errutil.Explain(nil, "resilience: NewExecutor requires a non-nil Manager")
	}
	return newExecutor(label, m, d.limiter), nil
}
