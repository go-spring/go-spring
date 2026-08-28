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

package trace

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// noopExporter is a span exporter that drops everything, for tests.
type noopExporter struct{}

func (noopExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error { return nil }
func (noopExporter) Shutdown(_ context.Context) error                               { return nil }

// TestRegisterSpanExporter proves a custom span exporter registered under a new
// name is resolved by NewTracerProvider, and that an unknown name yields an
// error listing the registered exporters (so misconfig is self-diagnosing).
func TestRegisterSpanExporter(t *testing.T) {
	const name = "test-noop"
	RegisterSpanExporter(name, func(_ TraceConfig) (sdktrace.SpanExporter, error) {
		return noopExporter{}, nil
	})
	t.Cleanup(func() { exporters.Delete(name) })

	// Registered exporter resolves: NewTracerProvider must not error on lookup.
	tp, err := NewTracerProvider(TraceConfig{Exporter: name, SamplerRatio: 1}, mustResource(t))
	assert.Error(t, err).Nil()
	assert.That(t, tp).NotNil()
	_ = tp.Shutdown(context.Background())

	// Unknown exporter yields an error naming the registered set.
	_, err = NewTracerProvider(TraceConfig{Exporter: "does-not-exist", SamplerRatio: 1}, mustResource(t))
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains(name)
}

// TestSamplerRatioNonPositiveRejected proves a non-positive sampler ratio
// (which would silently drop every trace) fails fast with remediation hints
// instead of being honored as "never sample".
func TestSamplerRatioNonPositiveRejected(t *testing.T) {
	for _, ratio := range []float64{0, -0.5} {
		_, err := NewTracerProvider(TraceConfig{Exporter: "stdout", SamplerRatio: ratio}, mustResource(t))
		assert.Error(t, err).NotNil()
		assert.String(t, err.Error()).Contains("sampler-ratio")
		assert.String(t, err.Error()).Contains("exporter=none")
	}
}

func mustResource(t *testing.T) *resource.Resource {
	t.Helper()
	res, err := NewResource("trace-test")
	assert.Error(t, err).Nil()
	return res
}
