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
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-gin"

// metricsMiddleware records HTTP request metrics — total count, duration, and
// in-flight gauge — through the global MeterProvider that starter-otel installs.
// When starter-otel is not imported the global MeterProvider is a no-op, so this
// costs almost nothing. The middleware is on by default; importing starter-otel activates it.
func metricsMiddleware() gin.HandlerFunc {
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

	return func(c *gin.Context) {
		attrs := metric.WithAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", c.FullPath()),
		)

		requestsInFlight.Add(c.Request.Context(), 1, attrs)
		start := time.Now()

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		requestCount.Add(c.Request.Context(), 1,
			metric.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
				attribute.String("http.status_code", status),
			),
		)
		requestDuration.Record(c.Request.Context(), time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
				attribute.String("http.status_code", status),
			),
		)
		requestsInFlight.Add(c.Request.Context(), -1, attrs)
	}
}
