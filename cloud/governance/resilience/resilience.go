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
// [go-spring.org/cloud/discovery]:
//
//   - [Policy] is a backend-neutral, declarative description of the desired
//     protection.
//   - [Driver] turns a Policy into a live [Executor]. A company (or the bundled
//     default) implements Driver once and registers it via [RegisterDriver];
//     callers select it by name through [GetDriver] with no per-component
//     adaptation.
//   - The bundled "default" driver (see driver.go) has zero third-party
//     dependencies so the framework runs standalone; the recommended
//     production driver (sentinel-golang) lives in its own module and registers
//     itself on blank import.
//
// The package's files map one-to-one to its concepts: policy.go (the declarative
// spec), breaker.go / ratelimit.go (the primitives), executor.go / driver.go
// (the runtime + pluggable backend), backoff.go (retry), dialer.go /
// roundtripper.go (the client-side seams), config.go (the gs-bound config).
package resilience

import "errors"

// ErrRateLimited is returned (or wrapped) by an [Executor] when an operation is
// rejected because the configured rate limit is exceeded.
var ErrRateLimited = errors.New("resilience: rate limited")

// ErrCircuitOpen is returned (or wrapped) by an [Executor] when an operation is
// rejected because the circuit breaker for its resource is open.
var ErrCircuitOpen = errors.New("resilience: circuit open")

// ErrBulkheadFull is returned (or wrapped) by an [Executor] when an operation is
// rejected because the resource already has the maximum number of concurrent
// in-flight operations allowed by the bulkhead.
var ErrBulkheadFull = errors.New("resilience: bulkhead full")
