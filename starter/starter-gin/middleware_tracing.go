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

package StarterGin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-gin"

// tracingMiddleware starts a server span for each inbound request: it extracts
// any trace context the caller propagated in headers (via the global propagator
// that starter-otel installs, or the SDK's no-op default) and starts a span
// named "HTTP <method>". When the request completes, status code and route are
// stamped as span attributes; 5xx responses mark the span as errored.
//
// The middleware rides the OTel globals — when starter-otel is not imported the
// global TracerProvider and TextMapPropagator are no-ops, so this costs almost
// nothing and changes no response behaviour. The middleware is on by default; importing starter-otel activates it.
func tracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract the incoming trace context from headers so the new span joins
		// the caller's trace — the same extract-then-start dance that
		// StarterOTel.StartServerSpan wraps. Using the global propagator means
		// this works with any propagation format starter-otel configures (W3C,
		// B3, etc.) without copying that configuration here.
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)
		ctx, span := otel.Tracer(tracerName).Start(ctx, "HTTP "+c.Request.Method,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		c.Request = c.Request.WithContext(ctx)

		// Stamp request attributes on the span as early as possible, so they
		// appear even when a later middleware (e.g. body limit) short-circuits.
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.target", c.Request.URL.Path),
			attribute.String("net.host.name", c.Request.Host),
			attribute.String("http.scheme", scheme(c.Request)),
		)

		c.Next()

		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
		)
		if c.FullPath() != "" {
			span.SetAttributes(attribute.String("http.route", c.FullPath()))
		}
		if status := c.Writer.Status(); status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		}
		span.End()
	}
}

// scheme returns "https" when the request arrived over TLS, "http" otherwise.
func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
