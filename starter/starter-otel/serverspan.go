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

package StarterOTel

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// StartServerSpan begins a server-side span for an inbound request. It first
// extracts any trace context the caller propagated in header — using the global
// propagator this starter installs from ${spring.observability.trace.propagator}
// (W3C traceparent and/or B3) — so the new span joins the caller's trace instead
// of starting a disconnected one, then starts a span named name on the tracer.
//
// It is the inbound counterpart to the outbound propagation this starter wires
// automatically through discovery.SetTraceInjector: outbound calls inject the
// active context, inbound handlers call this to continue the trace. Keeping both
// in the one starter that owns the OTel propagator lets stdlib stay OTel-free
// while any handler — the built-in HTTP mux, a gin engine, a gateway filter —
// continues a trace with a single call instead of hand-rolling the same
// extract-then-start dance.
//
// The returned context carries the span; the caller owns the span's lifetime and
// must End it (typically `defer span.End()`) and serve the remainder of the
// request under the returned context so child spans and logs attach to it.
func StartServerSpan(ctx context.Context, header http.Header, tracer, name string) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(header))
	return otel.Tracer(tracer).Start(ctx, name)
}
