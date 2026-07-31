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

// Package metric builds the OTel MeterProvider from observability configuration.
// It is consumed by the top-level starter-otel package which wires the provider
// into the OTel process globals.
//
// Metric backends are pluggable via a driver registry: the built-in exporters
// (otlp-grpc, otlp-http, prometheus, stdout) live in subpackages and
// self-register at init, and an application can add its own by calling
// RegisterMeterExporter from an init. NewMeterProvider looks the configured
// exporter up by name.
package metric

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// MetricsConfig configures the shared MeterProvider under
// ${spring.observability.metrics}. The prometheus exporter is pull-based and
// serves Path on Port; the otlp/stdout exporters are push-based on Interval.
// Empty/zero values keep OTel SDK defaults.
type MetricsConfig struct {
	Enable   bool               `value:"${enable:=true}"`
	Exporter string             `value:"${exporter:=otlp-grpc}"` // otlp-grpc|otlp-http|prometheus|stdout|none
	Endpoint string             `value:"${endpoint:=}"`
	Insecure bool               `value:"${insecure:=true}"`
	Port     int                `value:"${port:=9090}"`
	Path     string             `value:"${path:=/metrics}"`
	Interval time.Duration      `value:"${interval:=10s}"` // push interval for otlp/stdout readers
	Runtime  RuntimeMetricsConfig `value:"${runtime}"`
}

// RuntimeMetricsConfig controls Go runtime instrumentation under
// ${spring.observability.metrics.runtime}. When enabled the starter feeds Go
// runtime metrics (GC, heap/alloc, goroutine count, GOMAXPROCS, scheduling)
// into the shared MeterProvider, so they surface through whichever metrics
// exporter is configured without any per-project wiring. This is continuous
// metrics, complementing starter-pprof's on-demand profile dumps.
type RuntimeMetricsConfig struct {
	Enable bool `value:"${enable:=true}"`
	// MinReadMemStatsInterval caps how often runtime.ReadMemStats is called,
	// which is stop-the-world; zero keeps the instrumentation's own default.
	MinReadMemStatsInterval time.Duration `value:"${min-read-mem-stats-interval:=15s}"`
}

// PromServe carries the pull-based Prometheus artifacts produced by the
// metrics exporter. Handler renders the registry (always set for the prometheus
// exporter) and is contributed to the actuator as an endpoint.Endpoint so
// /metrics is served on the shared management port. Server is the optional
// dedicated scrape server, started only when a positive port is configured; set
// ${spring.observability.metrics.port}=0 to serve /metrics solely through the
// actuator.
type PromServe struct {
	Handler http.Handler
	Server  *http.Server
}

// NewMeterProvider builds a MeterProvider for the configured exporter. The
// exporter is looked up by name in the meter-exporter registry; the built-in
// names (otlp-grpc, otlp-http, prometheus, stdout) are registered by subpackages
// the starter blank-imports, and applications may add their own via
// RegisterMeterExporter. A pull-based exporter returns a *PromServe carrying
// the scrape handler (and optional dedicated server); a push-based exporter
// returns a nil *PromServe.
func NewMeterProvider(cfg MetricsConfig, res *resource.Resource) (*sdkmetric.MeterProvider, *PromServe, error) {
	f, ok := lookupMeterExporter(cfg.Exporter)
	if !ok {
		return nil, nil, unknownExporterErr(cfg.Exporter)
	}
	reader, ps, err := f(cfg)
	if err != nil {
		return nil, nil, err
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	), ps, nil
}

// NewPeriodicReader wraps a push exporter in a PeriodicReader. A zero/negative
// interval keeps the reader's own default cadence. The resource is added by the
// caller's MeterProvider, so this returns only the reader.
func NewPeriodicReader(exp sdkmetric.Exporter, interval time.Duration) sdkmetric.Reader {
	opts := []sdkmetric.PeriodicReaderOption{}
	if interval > 0 {
		opts = append(opts, sdkmetric.WithInterval(interval))
	}
	return sdkmetric.NewPeriodicReader(exp, opts...)
}
