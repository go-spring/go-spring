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

package StarterOTel

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newResource builds a schemaless resource carrying just service.name. Being
// schemaless avoids coupling the whole starter to a single semconv version
// (the same choice made in contrib/go-kratos/provider/observability.go), while
// still giving backends a stable service dimension to group traces/metrics by.
func newResource(serviceName string) (*resource.Resource, error) {
	return resource.NewSchemaless(
		attribute.String("service.name", serviceName),
	), nil
}

// newTracerProvider builds a batching TracerProvider for the configured
// exporter. Endpoint is required for the otlp exporters; an empty endpoint
// falls back to the exporter's own default (localhost:4317 / :4318).
func newTracerProvider(cfg TraceConfig, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	var exp sdktrace.SpanExporter
	var err error
	switch cfg.Exporter {
	case "otlp-grpc":
		opts := []otlptracegrpc.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exp, err = otlptracegrpc.New(ctx, opts...)
	case "otlp-http":
		opts := []otlptracehttp.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exp, err = otlptracehttp.New(ctx, opts...)
	case "stdout":
		exp, err = stdouttrace.New()
	default:
		return nil, fmt.Errorf("observability: unknown trace exporter %q (want otlp-grpc|otlp-http|stdout|none)", cfg.Exporter)
	}
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(newSampler(cfg.SamplerRatio)),
	), nil
}

// newSampler maps a ratio to a ParentBased sampler: >=1 always sample, <=0
// never, otherwise a TraceID ratio sampler. ParentBased keeps a trace's
// sampling decision consistent once an upstream service has decided.
func newSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio >= 1:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case ratio <= 0:
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

// newPropagator returns the text-map propagator for cross-service context
// propagation. "w3c" is the W3C TraceContext + Baggage combination; "none"
// leaves the process default untouched (returns nil).
func newPropagator(name string) (propagation.TextMapPropagator, error) {
	switch name {
	case "", "w3c":
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		), nil
	case "none":
		return nil, nil
	default:
		return nil, fmt.Errorf("observability: unknown propagator %q (want w3c|none)", name)
	}
}

// promServe carries the pull-based Prometheus artifacts produced by the
// metrics exporter. handler renders the registry (always set for the prometheus
// exporter) and is contributed to the actuator as an endpoint.Endpoint so
// /metrics is served on the shared management port. server is the optional
// dedicated scrape server, started only when a positive port is configured; set
// ${spring.observability.metrics.port}=0 to serve /metrics solely through the
// actuator.
type promServe struct {
	handler http.Handler
	server  *http.Server
}

// newMeterProvider builds a MeterProvider for the configured exporter. The
// prometheus exporter is pull-based and returns a *promServe carrying the scrape
// handler (and an optional dedicated server); the otlp/stdout exporters are
// push-based via a PeriodicReader and return a nil *promServe.
func newMeterProvider(cfg MetricsConfig, res *resource.Resource) (*sdkmetric.MeterProvider, *promServe, error) {
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
		ps := &promServe{handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{})}
		if cfg.Port > 0 {
			ps.server = serveMetrics(fmt.Sprintf(":%d", cfg.Port), cfg.Path, ps.handler)
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
		return newPushMeterProvider(exp, cfg.Interval, res), nil, nil

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
		return newPushMeterProvider(exp, cfg.Interval, res), nil, nil

	case "stdout":
		exp, err := stdoutmetric.New()
		if err != nil {
			return nil, nil, err
		}
		return newPushMeterProvider(exp, cfg.Interval, res), nil, nil

	default:
		return nil, nil, fmt.Errorf("observability: unknown metrics exporter %q (want otlp-grpc|otlp-http|prometheus|stdout|none)", cfg.Exporter)
	}
}

// newPushMeterProvider wraps a push exporter in a PeriodicReader. A zero/negative
// interval keeps the reader's own default cadence.
func newPushMeterProvider(exp sdkmetric.Exporter, interval time.Duration, res *resource.Resource) *sdkmetric.MeterProvider {
	readerOpts := []sdkmetric.PeriodicReaderOption{}
	if interval > 0 {
		readerOpts = append(readerOpts, sdkmetric.WithInterval(interval))
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, readerOpts...)),
		sdkmetric.WithResource(res),
	)
}

// serveMetrics starts a standalone HTTP server rendering the Prometheus scrape
// handler on path. It runs on its own listener (decoupled from any component's
// transport), mirroring the dedicated :9090 the dubbo/kratos examples expose.
// The same handler is also contributed to the actuator (see metricsEndpoint), so
// this dedicated server is optional and skipped when port<=0.
func serveMetrics(addr, path string, handler http.Handler) *http.Server {
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

// metricsEndpoint adapts the Prometheus scrape handler to endpoint.Endpoint so
// the actuator mounts /metrics on its management port. It is the seam that lets
// starter-otel expose metrics through the actuator without either starter
// importing the other — both depend only on stdlib.
type metricsEndpoint struct {
	path    string
	handler http.Handler
}

func newMetricsEndpoint(path string, handler http.Handler) *metricsEndpoint {
	return &metricsEndpoint{path: path, handler: handler}
}

func (m *metricsEndpoint) Path() string { return m.path }

func (m *metricsEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.handler.ServeHTTP(w, r)
}
