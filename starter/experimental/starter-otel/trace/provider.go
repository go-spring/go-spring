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

// Package trace builds the OTel TracerProvider from observability configuration.
// It is consumed by the top-level starter-otel package which wires the provider
// into the OTel process globals.
package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewResource builds a schemaless resource carrying just service.name. Being
// schemaless avoids coupling the whole starter to a single semconv version
// (the same choice made in contrib/go-kratos/provider/observability.go), while
// still giving backends a stable service dimension to group traces/metrics by.
func NewResource(serviceName string) (*resource.Resource, error) {
	return resource.NewSchemaless(
		attribute.String("service.name", serviceName),
	), nil
}

// NewTracerProvider builds a batching TracerProvider for the configured
// exporter. Endpoint is required for the otlp exporters; an empty endpoint
// falls back to the exporter's own default (localhost:4317 / :4318).
func NewTracerProvider(cfg TraceConfig, res *resource.Resource) (*sdktrace.TracerProvider, error) {
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
		sdktrace.WithSampler(NewSampler(cfg.SamplerRatio)),
	), nil
}

// NewSampler maps a ratio to a ParentBased sampler: >=1 always sample, <=0
// never, otherwise a TraceID ratio sampler. ParentBased keeps a trace's
// sampling decision consistent once an upstream service has decided.
func NewSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio >= 1:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case ratio <= 0:
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

// NewPropagator returns the text-map propagator for cross-service context
// propagation. "w3c" is the W3C TraceContext + Baggage combination; "none"
// leaves the process default untouched (returns nil).
func NewPropagator(name string) (propagation.TextMapPropagator, error) {
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

// TraceConfig configures the shared TracerProvider under
// ${spring.observability.trace}. Exporter selects the span backend; Endpoint is
// required for the otlp exporters. SamplerRatio drives a ParentBased ratio
// sampler (>=1 always, <=0 never). Empty/zero values keep OTel SDK defaults.
type TraceConfig struct {
	Enable       bool    `value:"${enable:=true}"`
	Exporter     string  `value:"${exporter:=otlp-grpc}"` // otlp-grpc|otlp-http|stdout|none
	Endpoint     string  `value:"${endpoint:=}"`
	Insecure     bool    `value:"${insecure:=true}"`
	SamplerRatio float64 `value:"${sampler-ratio:=1.0}"`
	Propagator   string  `value:"${propagator:=w3c}"` // w3c|none
}
