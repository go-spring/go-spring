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
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-hertz"

// metricsMiddleware records HTTP request metrics — total count, duration, and
// in-flight gauge — through the global MeterProvider that starter-otel installs.
// When starter-otel is not imported the global MeterProvider is a no-op, so this
// costs almost nothing. Importing starter-otel is the opt-in.
func metricsMiddleware() app.HandlerFunc {
	meter := otel.GetMeterProvider().Meter(meterName)

	requestCount, _ := meter.Int64Counter(
		"http.server.request_count",
		metric.WithDescription("Number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := meter.Float64Histogram(
		"http.server.request_duration",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("s"),
	)
	requestsInFlight, _ := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of HTTP requests currently in-flight"),
		metric.WithUnit("{request}"),
	)

	return func(ctx context.Context, c *app.RequestContext) {
		method := string(c.Request.Method())

		attrs := metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", string(c.Request.URI().Path())),
		)

		requestsInFlight.Add(ctx, 1, attrs)
		start := time.Now()

		c.Next(ctx)

		status := strconv.Itoa(c.Response.StatusCode())
		requestCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", string(c.Request.URI().Path())),
				attribute.String("http.status_code", status),
			),
		)
		requestDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", string(c.Request.URI().Path())),
				attribute.String("http.status_code", status),
			),
		)
		requestsInFlight.Add(ctx, -1, attrs)
	}
}
