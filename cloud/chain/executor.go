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
// The package is a leaf in the sense that matters: it depends on no engine that
// BUILDS a chain (go-spring.org/cloud/resilience, or a vendor backend), so the
// many packages that only need to HOLD a chain (a fault injector, a batch
// runner, a transaction coordinator) depend on the shape without taking on a
// backend. It does depend on OTel and go-spring.org/log, because the call
// description ([Operation]) is where a client's declaration and the emitter's
// vocabulary meet — and [Operation] lives here, beside the [Executor] that
// carries it. The engines live in go-spring.org/cloud/resilience; this package
// is the seam they are selected through.
package chain

import (
	"context"
	"errors"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	// Execute runs fn under the chain. op is the operation's description — its
	// target, scenario, idempotency, and how to name the signals it produces —
	// passed explicitly so every link reads it from the signature rather than
	// from the context. It returns the chain's rejection (rate limited, circuit
	// open, bulkhead full — see the engine's sentinels) when the call is rejected
	// before fn runs, or fn's own (final) error otherwise. The context passed to
	// fn may be a timeout derived from ctx.
	Execute(ctx context.Context, op Operation, fn func(context.Context) error) error

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

// Operation is the description of one governed call: what it targets, which
// scenario within that target it belongs to, whether repeating it is safe, and
// how to name the signals it produces. It is the single descriptor passed to
// [Executor.Execute], unifying what a call IS (the downstream it targets, the
// scenario within it, whether repeating it is safe) with how the process should
// DESCRIBE it (the span name, the metric family, the access-log tag). Both are
// declared by the client that owns the call — the only side that knows whether
// it is talking to a cache, a broker or a database — and both travel to every
// link of the chain through the Execute signature.
//
// The identity half is what protection keys on; the description half is what an
// emitter reads. Keeping them one type means a layer can read either without a
// second channel, and a future inbound/outbound concern has one place to land.
//
// It is value-only and self-contained — the emitter needs no other declaration
// to report the call, which is what lets the client that owns the call live
// apart from the code that emits it.
type Operation struct {
	// Target is the downstream this call is aimed at — a service name or a
	// system label. It is the dimension protection is scoped by: two calls to
	// different targets must not share a breaker or a budget.
	Target string

	// Scenario is the finer dimension within a target that a call belongs to,
	// when one exists — a tenant, an experiment arm, a routing label, often taken
	// from a request header. Empty means the call belongs to no finer scenario
	// than its target.
	//
	// It exists so calls to one target that must be judged separately can be:
	// the tenant paying for a slow call is not the tenant answering it. A layer
	// that keys nothing on it is unaffected.
	Scenario string

	// Retry is the call's retry declaration — the explicit channel through which
	// the caller (the only side that knows whether repeating the call repeats a
	// side effect) opts into retry. The zero value means "no retry": a call runs
	// exactly once unless someone declares otherwise, because retry is the one
	// protection stage that amplifies a downstream's load rather than gating it.
	//
	// It travels per call, unlike the rest of a service's protection, which is
	// construction-bound on the executor. A governance center MAY still push a
	// retry node on the policy for the whole service — that node overrides this
	// one whole when it declares a schedule (a live hot-fix must win), while the
	// NonIdempotent flag is preserved from both (see the resilience package's
	// EffectiveRetry, which is the single place the two are merged).
	Retry RetryPolicy

	// Name is what the operation is called where a single word is wanted — the
	// span name today ("get", "publish"). It is separate from [Operation.Attrs]
	// because the word for the operation lives under a family-specific key there
	// (db.operation, messaging.operation), and the emitter must name the span
	// without knowing which family it is looking at.
	Name string

	// Metric is the metric-name prefix a client-side operation of this kind
	// reports under, without the trailing dot: "db.client" for a database
	// client, "messaging.client" for a message broker client. The emitter
	// appends the suffix that names the signal (".operation.duration",
	// ".attempt.duration", ".active_requests").
	//
	// The prefix is the client's to declare, not the emitter's: only the client
	// knows whether these calls are a database or a broker, and db.client.*
	// attached to a broker would name the wrong thing.
	//
	// It must not be empty for a described call — the emitter builds its
	// instrument names from it, so an empty one registers an invalid name whose
	// error is discarded and every metric for that client is silently lost. A
	// call that declares no description leaves it empty and takes the emitter's
	// generic fallback.
	Metric string

	// Attrs are the operation's semantic attributes, in the family's own
	// vocabulary: db.system / db.operation, or messaging.system /
	// messaging.operation. They reach all three signals — metric labels, span
	// attributes and log fields — so one spelling covers everything.
	//
	// They MUST be bounded: every attribute here becomes a metric label, and a
	// label whose values are drawn from an open set (a cache key, a message
	// subject) multiplies the series until the metric is unusable. Per-call
	// detail that varies per request belongs in [Operation.Detail] instead.
	Attrs []attribute.KeyValue

	// Detail is per-call detail that may be unbounded — the operation's
	// argument, a cache key, a message subject. It reaches the span attributes
	// and the log fields, but never a metric label, which is what keeps a key
	// from becoming a label. It is also what a success log is levelled by: a
	// success carrying detail is high-frequency and uninteresting until it
	// fails (logged at Debug), one carrying none is worth a line (Info).
	Detail []attribute.KeyValue

	// SpanKind is what the call's span is to the trace topology — a
	// [trace.SpanKindProducer] for a message sent into a broker, a
	// [trace.SpanKindConsumer] for one taken out of it. The client declares it
	// because only the client knows the direction of the edge it is adding; the
	// emitter sets it on the span it opens.
	//
	// The zero value is [trace.SpanKindUnspecified], and the emitter reads it as
	// [trace.SpanKindInternal] — the kind an in-process client call has always
	// had, so a client that declares nothing keeps today's shape.
	SpanKind trace.SpanKind

	// LogTag is the tag the access log for this operation is written under. It
	// is the client's, not the emitter's, so a service's existing access log
	// keeps its identity across the move to a single emitter.
	LogTag *log.Tag
}
