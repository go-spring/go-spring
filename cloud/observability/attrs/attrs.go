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

// Package attrs is the neutral telemetry vocabulary: the per-request attribute
// carrier and the shared duration buckets. It is NOT part of either plane. It
// deliberately imports nothing from the governance stack (no chain, guard or
// breaker), so a package that only needs a telemetry primitive — a cache, a
// lock, a scheduler, the control-plane registration package — does not drag
// request-path instrumentation into its dependency graph.
//
// It lives at the top level for the same reason `cloud/propagate`
// does: both planes and every capability may depend on neutral vocabulary, but
// none of them owns it.
//
// The property-refresh funnel is a separate neutral sibling
// (`cloud/observability/conf`): it is config
// instrumentation, not span vocabulary, and it builds on this package's
// duration buckets.
//
// The governed-call layer itself (`cloud/observability/attrs`)
// builds on this package: it wraps [chain.Executor] and drives the guard's
// Admit/Done pair, and it is the only half that knows about policy.
//
// Why the carrier lives on the context. Most of go-spring's instrumentation
// starts its span inside the framework — a registry registration, a lock
// acquisition, a cache or DB call — and hands the span-carrying context to an
// inner closure, never back to the caller. On the caller's context,
// trace.SpanFromContext therefore finds the parent span, not the one being
// recorded, and there is nothing to call SetAttributes on. Carrying the
// attributes on the context lets those spans pick them up anyway.
//
// The reader lives here too — [SpanAttributesProcessor], which a TracerProvider
// must be given or the carrier is inert. Writing needs no OTel SDK; reading
// does, and both halves ship in this one package rather than splitting a single
// contract across two.
//
// The carrier does not reach metrics: the metric SDK has no per-record hook,
// so built-in metric labels stay closed to arbitrary attributes by design. To
// attach attributes to your own instrument, pass them where you record it.
package attrs

import (
	"context"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// carrierKey is the private key under which a context carries attributes. An
// unexported empty-struct type keeps it collision-free: no other package can
// produce a value that compares equal to it.
type carrierKey struct{}

// carrier is what [WithSpanAttributes] stores on a context: the attributes to
// apply to every span started with it. Each call stores a fresh slice, so
// sibling contexts derived from one parent never observe each other's
// attributes.
type carrier []attribute.KeyValue

// WithSpanAttributes returns a context carrying attributes that every span
// started with it includes, on top of the span's own static attributes.
// Attributes accumulate down the derivation chain; when two sources supply the
// same key, the one evaluated later wins. Nested spans inherit, because a child
// is started from a context derived from this one. With no attributes it
// returns ctx unchanged.
//
// Two limits shape what this can do, and neither fails loudly.
//
// The attributes are applied when a span starts, so they annotate before the
// call. A span the framework started internally cannot take attributes
// discovered while it ran: by the time the caller learns them, the span is the
// framework's, not the caller's. A caller's own span needs no help from here —
// span.SetAttributes works until span.End.
//
// Something has to read the carrier. This function only puts attributes on a
// context; a registered sdktrace.SpanProcessor applies them (see
// [SpanAttributesProcessor], which starter-otel's NewTracerProvider registers
// on its own). An application that builds its own TracerProvider without it
// gets a context that carries attributes nobody applies — no error, no
// attributes.
//
// The attributes live exactly as long as the context does. There is no Clear
// counterpart, and none is needed: unlike a thread-local, a context is never
// pooled for reuse.
//
// Process-level dimensions (environment, cluster, version) do not belong here;
// they are the same for every span in the process and belong on the OTel
// resource — see spring.observability.service-name and OTEL_RESOURCE_ATTRIBUTES.
func WithSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	prev := SpanAttributes(ctx)
	next := make([]attribute.KeyValue, 0, len(prev)+len(attrs))
	next = append(next, prev...)
	next = append(next, attrs...)
	return context.WithValue(ctx, carrierKey{}, carrier(next))
}

// SpanAttributes returns the attributes the context carries, in evaluation
// order, or nil when it carries none.
//
// The result is owned by the context and must not be modified: it is handed out
// without copying because the reader runs once per span, on the hot path of
// starting one. Callers that need to derive from it should copy first.
func SpanAttributes(ctx context.Context) []attribute.KeyValue {
	if c, ok := ctx.Value(carrierKey{}).(carrier); ok {
		return c
	}
	return nil
}

// SetSpanAttributes adds attrs to the span ctx currently carries, or does
// nothing when it carries none.
//
// It serves the case [WithSpanAttributes] cannot: a layer that already holds
// the span-carrying context — the closure under an instrumented call, the
// observed operation itself — writes straight to the span that is running, and
// needs no reader for it.
//
// The two are not interchangeable: a caller outside the span's window has no
// span on its context yet and must hand its attributes in with
// [WithSpanAttributes] instead; a caller inside the window that reached for
// the carrier would be annotating spans started later, not the one it is
// running in.
//
// A context without a recording span — no provider installed, or outside any
// span — makes this a no-op: [trace.SpanFromContext] returns a non-recording
// span rather than nil, and writing to one is legal and discarded.
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if len(attrs) == 0 {
		return
	}
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
}

// spanAttrsProcessor applies the attributes a context carries (see
// [WithSpanAttributes]) to every span started with it.
//
// It is what makes those attributes need no cooperation from instrumentation
// points. The OTel SDK calls OnStart with the context the caller passed to
// tracer.Start -- "Use original context", sdk/trace/tracer.go -- so one
// processor installed on the provider covers every span in the process,
// including the ones the framework starts internally.
//
// The writer half of this contract needs no OTel SDK — see [WithSpanAttributes].
// The reader does, and it lives in the same file because the two are halves of
// one contract; the cost of that choice is that importing the package links the
// SDK even for a caller that only ever writes.
type spanAttrsProcessor struct{}

// OnStart implements sdktrace.SpanProcessor.
func (spanAttrsProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if attrs := SpanAttributes(parent); len(attrs) > 0 {
		s.SetAttributes(attrs...)
	}
}

// OnEnd implements sdktrace.SpanProcessor.
func (spanAttrsProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

// Shutdown implements sdktrace.SpanProcessor. It holds no resources, so there is
// nothing to release.
func (spanAttrsProcessor) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdktrace.SpanProcessor. It holds no resources, so there
// is nothing to flush.
func (spanAttrsProcessor) ForceFlush(context.Context) error { return nil }

// SpanAttributesProcessor returns the [sdktrace.SpanProcessor] that applies the
// attributes a context carries (see [WithSpanAttributes]) to every span started
// with it.
//
// starter-otel's NewTracerProvider registers one automatically, so an
// application that lets it build the TracerProvider needs to do nothing. It is
// exported for the other case: an application that builds its own
// TracerProvider must register this processor itself, or the carrier has no
// reader and is inert.
func SpanAttributesProcessor() sdktrace.SpanProcessor { return spanAttrsProcessor{} }

// DurationBuckets returns the duration-histogram boundaries, in seconds, that
// every component builds its duration histograms from: the OTel HTTP semconv
// recommended set. One definition rather than a literal per package: a boundary
// is what makes two histograms comparable, and a drifted boundary is silent — the
// metric still reports, it just can no longer be bucketed against its siblings.
//
// It returns a fresh copy on every call. A component whose durations live on a
// different scale (cloud/scheduling's wake-up lag is the one such case today)
// takes the copy, edits it, and keeps it local; it cannot retune every other
// component's histograms by writing through this name.
func DurationBuckets() []float64 { return slices.Clone(durationBuckets[:]) }

// durationBuckets is the single definition [DurationBuckets] hands out. An
// array, not a slice literal, so passing it whole copies rather than aliases.
var durationBuckets = [14]float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
