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
	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-hertz"

// --- metrics ----------------------------------------------------------------

// instrumentSet holds this starter's metric instruments. It is built once per
// server (see [buildInstruments]), so nothing is allocated per request.
type instrumentSet struct {
	requestCount     metric.Int64Counter
	requestDuration  metric.Float64Histogram
	requestsInFlight metric.Int64UpDownCounter
}

// buildInstruments resolves this starter's instruments against the meter
// provider the otel starter installs before the server is built. The middleware
// owns the set, so a failed registration fails server construction instead of
// silently dropping the whole metric family.
func buildInstruments() (*instrumentSet, error) {
	m := otel.Meter(componentName)
	requestCount, err := m.Int64Counter(
		"http.server.request_count",
		metric.WithDescription("Number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "hertz: register http.server.request_count")
	}
	requestDuration, err := m.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("s"),
		// OTel HTTP semconv recommended buckets (seconds).
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...),
	)
	if err != nil {
		return nil, errutil.Explain(err, "hertz: register http.server.request.duration")
	}
	requestsInFlight, err := m.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of HTTP requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "hertz: register http.server.active_requests")
	}
	return &instrumentSet{
		requestCount:     requestCount,
		requestDuration:  requestDuration,
		requestsInFlight: requestsInFlight,
	}, nil
}

// metricsMiddleware records HTTP request metrics — total count, duration, and
// in-flight gauge — through the global MeterProvider that starter-otel installs.
// When starter-otel is not imported the global MeterProvider is a no-op, so this
// costs almost nothing. The middleware is on by default; importing starter-otel activates it.
func metricsMiddleware() (app.HandlerFunc, error) {
	ins, err := buildInstruments()
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, c *app.RequestContext) {
		method := string(c.Request.Method())

		attrs := metric.WithAttributes(
			attribute.String("http.request.method", method),
			attribute.String("http.route", string(c.Request.URI().Path())),
		)

		ins.requestsInFlight.Add(ctx, 1, attrs)
		start := time.Now()

		c.Next(ctx)

		status := strconv.Itoa(c.Response.StatusCode())
		ins.requestCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("http.request.method", method),
				attribute.String("http.route", string(c.Request.URI().Path())),
				attribute.String("http.response.status_code", status),
			),
		)
		ins.requestDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("http.request.method", method),
				attribute.String("http.route", string(c.Request.URI().Path())),
				attribute.String("http.response.status_code", status),
			),
		)
		ins.requestsInFlight.Add(ctx, -1, attrs)
	}, nil
}
