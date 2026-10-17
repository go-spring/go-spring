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

// Package guard defines the shape of an INBOUND protected request: the one
// object a server's middleware drives, and the vocabulary it answers with.
//
// It is the server-side counterpart of [go-spring.org/cloud/chain], and
// deliberately NOT its twin. A client call has a single around-function and,
// with retry, N attempts; an inbound request has one attempt and two hook
// points (admit before the handler, settle after it). That is why this package
// has no Operation, no Recorder, no Kind and no RetryPolicy: none of those
// describes an inbound request, and reusing them would make the server speak
// the client's model.
//
// # One server instance, one Guard, one state
//
// A Guard is created at wiring time and captured by the server's middleware.
// It is bound to the server instance it protects (see [Identity]) and owns the
// cross-request state of that instance — the rate-limit budget, the bulkhead
// count, the circuit breaker and the config generation it materializes. Two
// server instances never share this state, even when they carry the same
// label: a breaker belongs to a resource, and each instance is its own.
//
// The Guard is the STABLE shell. Its state is replaced whole when the
// governance authority pushes a new generation; the call path only reads the
// current generation, it never asks a registry. This mirrors the fault
// injector's shell/kernel model.
//
// # One judgment, one completion
//
// [Guard.Admit] runs the stages that must happen before the handler — rate
// limit, bulkhead acquisition, circuit-breaker gate and fault injection — and
// returns this request's [Result] plus the completion function for it. The
// caller (the protocol adapter) decides what to do with the result and runs
// the handler itself; the package never takes the handler as a callback.
//
// The completion function exists because two stages cannot be finished at
// admission: a bulkhead slot must be returned and a breaker's failure window
// must learn the outcome. It follows the Go convention of returning a cleanup
// function — call it, usually with defer — and is a no-op for a configuration
// that enables neither stage. It is idempotent, and a request context with a
// deadline arms a fallback settlement so a missed call cannot leak a slot.
//
// # What the caller owns
//
// Everything protocol-specific: mapping a [Verdict] onto an HTTP status,
// gRPC code or RPC result; deciding what a degraded response looks like;
// opening the span and writing the access log. Observability is therefore NOT
// part of this package, and a server that wants none of it simply ignores the
// result. The neutral facts a caller needs for those jobs are on the
// [Result], and the outcome it reports back is a protocol-level
// classification, not a Go error.
package guard
