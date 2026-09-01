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

// Package serverspan continues an inbound trace in this demo's services. It is a
// small helper that extracts any trace context the caller propagated in the
// request header - using the global OTel propagator starter-otel installs from
// ${spring.observability.trace.propagator} (W3C traceparent and/or B3) - and
// starts a server-side span on the given tracer, so the service's spans join the
// caller's trace instead of starting a disconnected one.
//
// It lives in the demo (not in starter-otel) because it is example plumbing, not
// a framework API: no starter wires it into a middleware automatically. Each
// service wraps its own framework handler around it (net/http in cmd/order, gin
// in cmd/inventory); this package is just the shared extract-then-start step so
// those wrappers don't duplicate it.
package serverspan

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// StartServerSpan begins a server-side span for an inbound request. It extracts
// any trace context the caller propagated in header using the global propagator,
// then starts a span named name on the tracer. The returned context carries the
// span; the caller owns the span's lifetime and must End it (typically
// `defer span.End()`) and serve the remainder of the request under the returned
// context so child spans and logs attach to it.
func StartServerSpan(ctx context.Context, header http.Header, tracer, name string) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(header))
	return otel.Tracer(tracer).Start(ctx, name)
}
