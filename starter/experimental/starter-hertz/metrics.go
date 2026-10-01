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
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go-spring.org/cloud/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-hertz"

// --- metrics ----------------------------------------------------------------

// instrumentSet holds this starter's metric instruments. It is built once per
// process (see instruments), so nothing is allocated per request.
type instrumentSet struct {
	requestCount     metric.Int64Counter
	requestDuration  metric.Float64Histogram
	requestsInFlight metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before the server starts, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(meterName)
	requestCount, _ := m.Int64Counter(
		"http.server.request_count",
		metric.WithDescription("Number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := m.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("s"),
		// OTel HTTP semconv recommended buckets (seconds).
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...),
	)
	requestsInFlight, _ := m.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of HTTP requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	return &instrumentSet{
		requestCount:     requestCount,
		requestDuration:  requestDuration,
		requestsInFlight: requestsInFlight,
	}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// metricsMiddleware records HTTP request metrics — total count, duration, and
// in-flight gauge — through the global MeterProvider that starter-otel installs.
// When starter-otel is not imported the global MeterProvider is a no-op, so this
// costs almost nothing. The middleware is on by default; importing starter-otel activates it.
func metricsMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		method := string(c.Request.Method())

		attrs := metric.WithAttributes(
			attribute.String("http.request.method", method),
			attribute.String("http.route", string(c.Request.URI().Path())),
		)

		instruments().requestsInFlight.Add(ctx, 1, attrs)
		start := time.Now()

		c.Next(ctx)

		status := strconv.Itoa(c.Response.StatusCode())
		instruments().requestCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("http.request.method", method),
				attribute.String("http.route", string(c.Request.URI().Path())),
				attribute.String("http.response.status_code", status),
			),
		)
		instruments().requestDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("http.request.method", method),
				attribute.String("http.route", string(c.Request.URI().Path())),
				attribute.String("http.response.status_code", status),
			),
		)
		instruments().requestsInFlight.Add(ctx, -1, attrs)
	}
}
