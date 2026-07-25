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

package StarterGateway

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "go-spring.org/starter-gateway"

// proxySpan wraps an http.Handler with OTel tracing: it extracts the incoming
// trace context, starts a server span for the gateway hop, then creates a child
// client span for the upstream call. The span names include the route id so
// operators can filter by route in their tracing backend.
//
// When starter-otel is not imported the global TracerProvider and propagator
// are no-ops, so this adds negligible overhead. Importing starter-otel is the
// opt-in.
func proxySpan(routeID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the caller's trace context so the gateway span joins the
		// incoming trace — otherwise every gateway span is a disconnected root.
		ctx := otel.GetTextMapPropagator().Extract(
			r.Context(),
			propagation.HeaderCarrier(r.Header),
		)

		// Server span: the inbound hop into the gateway.
		ctx, serverSpan := otel.Tracer(tracerName).Start(ctx, "gateway "+routeID,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.Path),
				attribute.String("gateway.route", routeID),
			),
		)

		// Client span: the outbound hop to the upstream. This nests inside the
		// server span so tracing backends show gateway→upstream latency broken
		// out from the total request latency.
		ctx, clientSpan := otel.Tracer(tracerName).Start(ctx, "proxy "+routeID,
			trace.WithSpanKind(trace.SpanKindClient),
		)

		// Propagate trace context to the upstream so it joins the same trace.
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))

		clientSpan.SetAttributes(attribute.Int("http.status_code", sw.status))
		if sw.status >= 500 {
			clientSpan.SetStatus(codes.Error, http.StatusText(sw.status))
		}
		clientSpan.End()

		serverSpan.SetAttributes(attribute.Int("http.status_code", sw.status))
		serverSpan.End()
	})
}
