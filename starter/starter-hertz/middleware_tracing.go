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

package StarterHertz

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-hertz"

// tracingMiddleware starts a server span for each inbound request: it extracts
// any trace context the caller propagated in headers and starts a span named
// "HTTP <method>". When the request completes, status code and path are stamped
// as span attributes; 5xx responses mark the span as errored.
//
// The middleware rides the OTel globals — when starter-otel is not imported the
// global TracerProvider and TextMapPropagator are no-ops, so this costs almost
// nothing. Importing starter-otel is the opt-in.
func tracingMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// Hertz stores headers in a []args.Header but the OTel Propagation
		// API needs http.Header. Build an adapter.
		hdr := make(http.Header)
		c.Request.Header.VisitAll(func(key, value []byte) {
			hdr.Add(string(key), string(value))
		})

		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(hdr))
		ctx, span := otel.Tracer(tracerName).Start(ctx, "HTTP "+string(c.Request.Method()),
			trace.WithSpanKind(trace.SpanKindServer),
		)

		span.SetAttributes(
			attribute.String("http.method", string(c.Request.Method())),
			attribute.String("http.target", string(c.Request.URI().Path())),
			attribute.String("net.host.name", string(c.Request.Host())),
		)

		c.Next(ctx)

		span.SetAttributes(
			attribute.Int("http.status_code", c.Response.StatusCode()),
		)
		if status := c.Response.StatusCode(); status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		}
		span.End()
	}
}
