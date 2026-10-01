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

// Package stdout registers the stdout span exporter with the trace span-exporter
// registry. It is registered by the top-level starter-otel package from its
// starter.go; mainly useful for local debugging and self-contained
// examples (no collector needed).
package stdout

import (
	"go-spring.org/starter-otel/trace"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Register makes this exporter available under the name "stdout". Called from
// the starter's starter.go; the package declares no init of its own.
func Register() {
	trace.RegisterSpanExporter("stdout", newStdout)
}

// newStdout builds a stdout span exporter that prints spans to stdout.
func newStdout(_ trace.TraceConfig) (sdktrace.SpanExporter, error) {
	return stdouttrace.New()
}
