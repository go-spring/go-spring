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
//
// Span backends are pluggable via a driver registry: the built-in exporters
// (otlp-grpc, otlp-http, stdout) live in subpackages and self-register at init,
// and an application can add its own by calling RegisterSpanExporter from an
// init. NewTracerProvider looks the configured exporter up by name.
package trace

import (
	"go-spring.org/starter-otel/internal/registry"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanExporterFactory builds a span exporter for one backend from the trace
// config. Register one under a name (typically from an init) to add a trace
// backend beyond the built-ins; NewTracerProvider looks it up by cfg.Exporter.
type SpanExporterFactory func(cfg TraceConfig) (sdktrace.SpanExporter, error)

// exporters is the shared generic registry (see internal/registry). It holds
// the bookkeeping that used to be duplicated byte-for-byte with the metric
// registry; only the factory type and category string differ.
var exporters = registry.New[SpanExporterFactory]("trace")

// RegisterSpanExporter makes a span exporter factory available under name. It
// panics on empty name, nil factory, or a duplicate - mirroring the
// driver-registry idiom used elsewhere (starter-otel's own registries, cloud/cache
// RegisterDriver) so a mis-wired or duplicate
// registration fails loudly at init.
func RegisterSpanExporter(name string, f SpanExporterFactory) {
	if f == nil {
		panic("trace: register nil span exporter factory for " + name)
	}
	exporters.Register(name, f)
}

func lookupSpanExporter(name string) (SpanExporterFactory, bool) {
	return exporters.Lookup(name)
}

// unknownExporterErr builds the error returned when cfg.Exporter names no
// registered span exporter, listing the available ones so the misconfig is
// self-diagnosing.
func unknownExporterErr(name string) error { return exporters.UnknownErr(name) }
