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
	"fmt"
	"sort"
	"strings"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanExporterFactory builds a span exporter for one backend from the trace
// config. Register one under a name (typically from an init) to add a trace
// backend beyond the built-ins; NewTracerProvider looks it up by cfg.Exporter.
type SpanExporterFactory func(cfg TraceConfig) (sdktrace.SpanExporter, error)

var (
	exporterMu  sync.RWMutex
	exporterReg = map[string]SpanExporterFactory{}
)

// RegisterSpanExporter makes a span exporter factory available under name. It
// panics on empty name, nil factory, or a duplicate - mirroring the
// driver-registry idiom used elsewhere (discovery.Register, starter-go-redis
// RegisterDriver) so a mis-wired or duplicate registration fails loudly at init.
func RegisterSpanExporter(name string, f SpanExporterFactory) {
	if name == "" {
		panic("trace: register span exporter with empty name")
	}
	if f == nil {
		panic("trace: register nil span exporter factory for " + name)
	}
	exporterMu.Lock()
	defer exporterMu.Unlock()
	if _, ok := exporterReg[name]; ok {
		panic("trace: span exporter already registered: " + name)
	}
	exporterReg[name] = f
}

func lookupSpanExporter(name string) (SpanExporterFactory, bool) {
	exporterMu.RLock()
	defer exporterMu.RUnlock()
	f, ok := exporterReg[name]
	return f, ok
}

// registeredSpanExporters returns the sorted names of all registered span
// exporters, for inclusion in "unknown exporter" error messages.
func registeredSpanExporters() []string {
	exporterMu.RLock()
	defer exporterMu.RUnlock()
	names := make([]string, 0, len(exporterReg))
	for n := range exporterReg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// unknownExporterErr builds the error returned when cfg.Exporter names no
// registered span exporter, listing the available ones so the misconfig is
// self-diagnosing.
func unknownExporterErr(name string) error {
	return fmt.Errorf("observability: unknown trace exporter %q (registered: %s)",
		name, strings.Join(registeredSpanExporters(), ", "))
}
