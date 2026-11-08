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

// Package resilience is the centralized protection authority for the process:
// retry, circuit breaking, rate limiting and timeout.
//
// This package is CLIENT-ONLY. Outbound calls run through a [chain.Executor]
// held for the client's lifetime; inbound requests run through
// [go-spring.org/cloud/governance/dataplane/inbound/admission]'s per-instance [admission.Guard], which has its own
// policy vocabulary and engine. There is no inbound manager, no inbound
// [Driver] method and no inbound policy type here — the two directions do not
// share a shape.
//
// The manager is container-free: it imports no IoC concept and holds no global.
// Its dependency on the governance center is inverted into plain types (Apply
// takes the pushed half), so this package never imports a control-plane root and
// no center reaches into this package's internals. The gs wiring
// (providing the manager as a bean, injecting it into the starters) lives in
// starter.go.
//
// The BACKEND, unlike the policies, is not pushed: it is fixed at construction —
// the [Driver] the container selected from ${spring.resilience.driver}, a
// startup decision, since switching it drops every executor's state.
//
// # A push reaches the executor shell
//
// Each Apply publishes its capability's document and then moves only the organ
// whose configuration changed inside the executors it built (see
// [Manager.ApplyBreaker] and [Manager.ApplyRateLimit]). The executor object is
// permanent — a client holds it for life — so what moves is its internal state,
// never the shell. The timeout/retry policy carries no materialized state: its
// document is read on the next call.
//
// One label maps to exactly ONE executor per direction. That is not an
// optimization but a semantic requirement: a breaker and limiter are
// per-RESOURCE state, so two objects sharing a label (a rebuilt client, two pools
// for one service name) must share the breaker rather than each getting its own
// and effectively doubling the trip threshold.
//
// # Files
//
// The package is laid out one file per capability, over shared rule and runtime
// files:
//
//	timeoutretry.go              the timeout/retry policy, document, resolution, validation
//	                             and the selector/scope vocabulary all capabilities share
//	breaker.go                   the breaker policy, document, resolution, validation and state key
//	ratelimit.go                 the rate-limit document, resolution, stage and rule identity
//	manager.go                   the runtime: the three published documents
//	executor.go                  the chain node + its per-label state
//	observe.go                   the executor's own state metrics
//	adapter_run.go,              the outbound seams
//	adapter_http.go
package resilience
