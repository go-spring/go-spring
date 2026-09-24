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

// Package observability carries per-request telemetry attributes on a context,
// so they reach the observations the framework makes on the caller's behalf
// without any instrumentation point having to cooperate, and provides
// RefreshConf, the shared funnel for property-refresh triggers.
//
// Why a context carrier rather than a span option. Most of go-spring's
// instrumentation starts its span inside the framework: a registry
// registration, a lock acquisition, a cache or DB call. Each creates its span
// below the caller's frame and hands the span-carrying context to an inner
// closure, never back to the caller. Reading trace.SpanFromContext on the
// caller's context therefore finds the parent span, not the one being recorded,
// and there is nothing to call SetAttributes on. Attaching to the context
// instead lets those spans pick the attributes up from one place.
//
// The contract is deliberately split from its reader. This package defines what
// the context carries and is all a writer — business code, a company middleware
// library — needs to depend on; it pulls in no OTel SDK. The reader is a
// SpanProcessor, which does need the SDK, and so lives where the TracerProvider
// is built (starter-otel's trace package, which registers it automatically).
// The same split as log's WithFields for logs.
//
// The context carrier does not reach metrics: the metric SDK has no
// per-record hook, so built-in metric labels stay closed to arbitrary
// attributes by design. To attach attributes to your own instrument, pass
// them where you record it.
package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

// carrierKey is the private key under which a context carries attributes. An
// unexported empty-struct type keeps it collision-free: no other package can
// produce a value that compares equal to it.
type carrierKey struct{}

// carrier is what [WithContextAttributes] stores on a context: the attributes
// to apply to every span started with it. Each call stores a fresh slice, so
// sibling contexts derived from one parent never observe each other's
// attributes.
type carrier []attribute.KeyValue

// WithContextAttributes returns a context carrying attributes that every span
// started with it will include -- on top of the span's own static attributes.
// Attributes accumulate down the derivation chain; when two sources supply the
// same key, the one evaluated later wins.
//
// Nested spans inherit: a child is started from a context derived from this
// one, so the attributes reach it too.
//
// Two limits shape what this can do, and neither fails loudly:
//
// The attributes are applied when a span starts, so they cover "annotate
// before the call". A span the framework started internally cannot take
// attributes discovered while it ran -- by the time the caller learns them,
// the span is the framework's, not the caller's. Attributes for a caller's
// own span need no help from here: span.SetAttributes works until span.End.
//
// Something has to read the carrier. This function only puts attributes on a
// context; a registered sdktrace.SpanProcessor applies them (see
// starter-otel/trace.ContextAttributesProcessor, which NewTracerProvider
// registers on its own). An application that builds its own TracerProvider
// without it gets a context that carries attributes nobody applies -- no
// error, no attributes.
//
// The attributes live exactly as long as the context does. There is no Clear
// counterpart, and none is needed: unlike a thread-local, a context is never
// pooled for reuse.
//
// Process-level dimensions (environment, cluster, version) do not belong here;
// they are the same for every span in the process and belong on the OTel
// resource — see spring.observability.service-name and OTEL_RESOURCE_ATTRIBUTES.
func WithContextAttributes(ctx context.Context, attrs ...attribute.KeyValue) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	prev := ContextAttributes(ctx)
	next := make([]attribute.KeyValue, 0, len(prev)+len(attrs))
	next = append(next, prev...)
	next = append(next, attrs...)
	return context.WithValue(ctx, carrierKey{}, carrier(next))
}

// ContextAttributes returns the attributes the context carries, in evaluation
// order, or nil when it carries none.
//
// The result is owned by the context and must not be modified: it is handed out
// without copying because the reader runs once per span, on the hot path of
// starting one. Callers that need to derive from it should copy first.
func ContextAttributes(ctx context.Context) []attribute.KeyValue {
	if c, ok := ctx.Value(carrierKey{}).(carrier); ok {
		return c
	}
	return nil
}
