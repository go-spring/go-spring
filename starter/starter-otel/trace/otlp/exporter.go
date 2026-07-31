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

// Package otlp registers the OTLP span exporters (otlp-grpc, otlp-http) with
// the trace span-exporter registry. It is blank-imported by the top-level
// starter-otel package so the default exporter is available with no per-app
// wiring; an application that wants neither OTLP exporter can drop this import.
package otlp

import (
	"context"

	"go-spring.org/starter-otel/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func init() {
	trace.RegisterSpanExporter("otlp-grpc", newGRPC)
	trace.RegisterSpanExporter("otlp-http", newHTTP)
}

// newGRPC builds an OTLP/gRPC span exporter from the trace config. Endpoint is
// optional - empty falls back to the exporter's localhost:4317 default.
func newGRPC(cfg trace.TraceConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	return otlptracegrpc.New(context.Background(), opts...)
}

// newHTTP builds an OTLP/HTTP span exporter from the trace config. Endpoint is
// optional - empty falls back to the exporter's localhost:4318 default.
func newHTTP(cfg trace.TraceConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return otlptracehttp.New(context.Background(), opts...)
}
