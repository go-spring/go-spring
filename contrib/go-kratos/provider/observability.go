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

package main

import (
	"context"
	"net/http"
	"time"

	kmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/otlptranslator"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// This file is the crux of kratos observability: unlike starter-dubbo (metrics
// and tracing on by config) or go-zero (its ServiceConf wires the three pillars
// natively), the kratos example has NO starter — the kratos.App is assembled by
// hand in server.go. So metrics and tracing must be wired in code here, using
// kratos' own middleware plus the OTel SDK, and exposed through a standalone
// Prometheus endpoint (the built-in HTTP server is disabled). Logs are the
// exception: business logs are emitted via go-spring's log module straight to a
// JSON file (see handler.go / provider conf), not bridged from kratos' logger.

// metricRequestsName is the kratos request counter instrument name. The OTel
// Prometheus exporter is configured with WithoutCounterSuffixes/WithoutUnits so
// the exported series name equals this string verbatim (no "_total"/unit suffix
// appended), which is what scripts/smoke-test.sh greps for.
const (
	metricRequestsName = "server_requests_code_total"
	metricSecondsName  = "server_requests_seconds"
)

// setupTracing builds an OTel TracerProvider that batches spans and exports them
// over OTLP/gRPC to the collector (Jaeger's :4317 in docker-compose.yml), and
// installs it as the process-global provider + W3C propagator so kratos'
// tracing.Server() middleware picks it up. AlwaysSample guarantees even a single
// smoke-test call produces a span in the Jaeger UI. The returned provider must
// be Shutdown on teardown to flush buffered spans.
func setupTracing(ctx context.Context, serviceName, endpoint string, insecure bool) (*sdktrace.TracerProvider, error) {
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create OTLP trace exporter for %s", endpoint)
	}

	// Schemaless resource with just service.name — enough for Jaeger to group
	// spans under "kratos-greeter" without coupling to a semconv version.
	res := resource.NewSchemaless(attribute.String("service.name", serviceName))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp, nil
}

// setupMetrics builds an OTel MeterProvider backed by a Prometheus exporter that
// registers into a fresh registry, then constructs the request counter and
// latency histogram kratos' metrics.Server() middleware records into. The
// registry is returned so serveMetrics can render it on the scrape endpoint.
func setupMetrics(serviceName string) (metric.Int64Counter, metric.Float64Histogram, *prometheus.Registry, error) {
	reg := prometheus.NewRegistry()

	// NoTranslation keeps the exported series name equal to the OTel instrument
	// name (no "_total"/unit suffix mangling), so the metric name is predictable
	// for querying and for the smoke-test assertion.
	exporter, err := otelprom.New(
		otelprom.WithRegisterer(reg),
		otelprom.WithTranslationStrategy(otlptranslator.NoTranslation),
	)
	if err != nil {
		return nil, nil, nil, errutil.Explain(err, "failed to create prometheus exporter")
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
		sdkmetric.WithView(kmetrics.DefaultSecondsHistogramView(metricSecondsName)),
	)
	meter := mp.Meter(serviceName)

	requests, err := kmetrics.DefaultRequestsCounter(meter, metricRequestsName)
	if err != nil {
		return nil, nil, nil, errutil.Explain(err, "failed to create requests counter")
	}
	seconds, err := kmetrics.DefaultSecondsHistogram(meter, metricSecondsName)
	if err != nil {
		return nil, nil, nil, errutil.Explain(err, "failed to create seconds histogram")
	}
	return requests, seconds, reg, nil
}

// serveMetrics starts a standalone HTTP server that renders the Prometheus
// registry on /metrics. It runs in its own listener (not the disabled built-in
// go-spring HTTP server, and not a kratos transport) so the scrape endpoint is
// decoupled from the RPC transports — mirroring the dedicated :9090 the
// dubbo-go examples expose. The returned *http.Server is Shutdown on teardown.
func serveMetrics(addr string, reg *prometheus.Registry) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// The metrics endpoint is auxiliary; log via kratos-independent path
			// is unnecessary here — a bind failure surfaces on the next scrape.
			_ = err
		}
	}()
	return srv
}
