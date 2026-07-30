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
package metric

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
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
// prometheus exporter is pull-based and returns a *PromServe carrying the scrape
// handler (and an optional dedicated server); the otlp/stdout exporters are
// push-based via a PeriodicReader and return a nil *PromServe.
func NewMeterProvider(cfg MetricsConfig, res *resource.Resource) (*sdkmetric.MeterProvider, *PromServe, error) {
	ctx := context.Background()

	switch cfg.Exporter {
	case "prometheus":
		reg := prometheus.NewRegistry()
		exp, err := otelprom.New(otelprom.WithRegisterer(reg))
		if err != nil {
			return nil, nil, err
		}
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(exp),
			sdkmetric.WithResource(res),
		)
		ps := &PromServe{Handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{})}
		if cfg.Port > 0 {
			ps.Server = ServeMetrics(fmt.Sprintf(":%d", cfg.Port), cfg.Path, ps.Handler)
		}
		return mp, ps, nil

	case "otlp-grpc":
		opts := []otlpmetricgrpc.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		exp, err := otlpmetricgrpc.New(ctx, opts...)
		if err != nil {
			return nil, nil, err
		}
		return NewPushMeterProvider(exp, cfg.Interval, res), nil, nil

	case "otlp-http":
		opts := []otlpmetrichttp.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		exp, err := otlpmetrichttp.New(ctx, opts...)
		if err != nil {
			return nil, nil, err
		}
		return NewPushMeterProvider(exp, cfg.Interval, res), nil, nil

	case "stdout":
		exp, err := stdoutmetric.New()
		if err != nil {
			return nil, nil, err
		}
		return NewPushMeterProvider(exp, cfg.Interval, res), nil, nil

	default:
		return nil, nil, fmt.Errorf("observability: unknown metrics exporter %q (want otlp-grpc|otlp-http|prometheus|stdout|none)", cfg.Exporter)
	}
}

// NewPushMeterProvider wraps a push exporter in a PeriodicReader. A zero/negative
// interval keeps the reader's own default cadence.
func NewPushMeterProvider(exp sdkmetric.Exporter, interval time.Duration, res *resource.Resource) *sdkmetric.MeterProvider {
	readerOpts := []sdkmetric.PeriodicReaderOption{}
	if interval > 0 {
		readerOpts = append(readerOpts, sdkmetric.WithInterval(interval))
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, readerOpts...)),
		sdkmetric.WithResource(res),
	)
}

// ServeMetrics starts a standalone HTTP server rendering the Prometheus scrape
// handler on path. It runs on its own listener (decoupled from any component's
// transport), mirroring the dedicated :9090 the dubbo/kratos examples expose.
// The same handler is also contributed to the actuator (see NewEndpoint), so
// this dedicated server is optional and skipped when port<=0.
func ServeMetrics(addr, path string, handler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = err
		}
	}()
	return srv
}