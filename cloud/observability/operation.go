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

// operation.go carries the semantic identity of one client operation on the
// context. The single emitter on the executor chain (see the resilience
// observe layer) reads it to name the span, the metric and the access log the
// way the client's kind demands — a database operation reports under
// db.client.*, a message publish under messaging.client.*.
//
// The split is deliberate: the client that owns the call declares the identity
// with [WithOperation] (it is the only side that knows it is talking to a cache
// and not to a broker), and the emitter applies it. Nothing here emits, and
// nothing here classifies an outcome — this file only defines what travels on
// the context, so nothing in it needs the OTel SDK.

package observability

import (
	"context"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// operationKey is the private key under which a context carries an Operation.
// An unexported empty-struct type keeps it collision-free: no other package can
// produce a value that compares equal to it.
type operationKey struct{}

// Operation is the semantic identity of one client operation: how the process
// should describe a call it is about to make to a downstream.
//
// It is value-only and self-contained — the emitter needs no other declaration
// to report the call, which is what lets the client that owns the call live
// apart from the code that emits it.
type Operation struct {
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
	// It must not be empty — [WithOperation] panics on an empty one, because the
	// instrument names the emitter builds from it would be invalid and the client
	// would silently lose every metric.
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

	// NonIdempotent marks an operation whose repetition is a second side effect
	// rather than a second attempt: sending an email, publishing to a broker,
	// handling a consumed record. The executor chain runs such a call ONCE
	// regardless of the configured retry count, because a retry would send a
	// second email or deliver a second message — something no policy downstream
	// can take back.
	//
	// The default is false, because for a request/response client retrying is the
	// safe assumption. Only the side that owns the call knows which kind it is, so
	// the declaration is the client's to make here; the framework supplies the
	// mechanism, not the judgement.
	NonIdempotent bool
}

// WithOperation returns a context carrying op as the semantic identity of the
// operation about to run. Attributes accumulate down the derivation chain when
// nested calls each declare their own (see [WithSpanAttributes] for the same
// accumulation rule); the innermost declaration is what an emitter reads.
//
// The declaration is per call: the operation name inside a family varies per
// method ("get" vs "set"), so it is set where the method is known, not once at
// wiring time.
func WithOperation(ctx context.Context, op Operation) context.Context {
	if op.Metric == "" {
		// A programmer error, caught at the first call rather than degrading
		// silently: the emitter builds its instrument names from this prefix, so
		// an empty one would register `.operation.duration` — an invalid name
		// whose error the OTel API returns and the emitter discards, losing every
		// metric for that client with nothing to show for it.
		panic("observability: Operation.Metric must not be empty (it is the metric-name prefix the emitter suffixes)")
	}
	return context.WithValue(ctx, operationKey{}, op)
}

// OperationFrom returns the Operation ctx carries and true, or the zero
// Operation and false when it carries none. A caller that finds none reports
// the call without a family-specific name rather than failing.
func OperationFrom(ctx context.Context) (Operation, bool) {
	op, ok := ctx.Value(operationKey{}).(Operation)
	return op, ok
}
