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
	"strings"
	"sync"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newTestRecorderProvider builds a provider carrying the same processors the
// real one registers, plus a recorder, so a test can start spans and inspect
// what they ended up with. The processors come first, matching how
// NewTracerProvider wires them relative to the exporter's own processor.
func newTestRecorderProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(ContextAttributesProcessor()),
		sdktrace.WithSpanProcessor(LoadTestProcessor()),
		sdktrace.WithSpanProcessor(rec),
	)
	return tp, rec
}

// attrsToString renders attributes as "k=v" pairs in slice order. It uses Emit
// rather than AsString: AsString returns the raw string for a STRING value and
// an empty string for everything else, so a bool attribute would render as
// "load_test=" and an assertion on it would be silently meaningless.
func attrsToString(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestContextAttributesReachSpansStartedBelow is the contract this whole
// mechanism exists for: a span the FRAMEWORK starts internally -- from a
// context it received, never handing the span-carrying context back -- still
// picks up the attributes. The test plays the framework's part by starting the
// span from the carried context and never exposing it upward.
func TestContextAttributesReachSpansStartedBelow(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	ctx := observability.WithContextAttributes(context.Background(), attribute.String("tenant", "t1"))
	_, span := tp.Tracer("test").Start(ctx, "framework-op")
	span.End()

	ended := rec.Ended()
	assert.Number(t, len(ended)).Equal(1)
	assert.String(t, attrsToString(ended[0].Attributes())).Equal("tenant=t1")
}

// TestContextAttributesReachChildSpans proves inheritance: a child span is
// started from a context derived from the carried one, so it gets the
// attributes too.
func TestContextAttributesReachChildSpans(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	ctx := observability.WithContextAttributes(context.Background(), attribute.String("tenant", "t1"))
	parentCtx, parent := tp.Tracer("test").Start(ctx, "parent")
	_, child := tp.Tracer("test").Start(parentCtx, "child")
	child.End()
	parent.End()

	ended := rec.Ended()
	assert.Number(t, len(ended)).Equal(2)
	for _, s := range ended {
		assert.String(t, attrsToString(s.Attributes())).Equal("tenant=t1")
	}
}

// TestSpansWithoutCarrierAreUntouched proves the processor is inert when the
// context carries nothing -- it must not add attributes of its own.
func TestSpansWithoutCarrierAreUntouched(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	span.End()

	ended := rec.Ended()
	assert.Number(t, len(ended)).Equal(1)
	assert.String(t, attrsToString(ended[0].Attributes())).Equal("")
}

// TestContextAttributesKeepStaticOnes proves the carried attributes are applied
// on top of, not instead of, the span's own static attributes.
func TestContextAttributesKeepStaticOnes(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	ctx := observability.WithContextAttributes(context.Background(), attribute.String("tenant", "t1"))
	_, span := tp.Tracer("test").Start(ctx, "op",
		oteltrace.WithAttributes(attribute.String("db.system", "redis")))
	span.End()

	assert.String(t, attrsToString(rec.Ended()[0].Attributes())).Equal("db.system=redis,tenant=t1")
}

// TestContextAttributesLaterWins proves the tie-break rule matches the rest of
// the design: on a duplicate key the later source is the effective one. The
// carried attributes are applied after the span's own, so they win there.
func TestContextAttributesLaterWins(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	ctx := observability.WithContextAttributes(context.Background(), attribute.String("k", "carried"))
	_, span := tp.Tracer("test").Start(ctx, "op",
		oteltrace.WithAttributes(attribute.String("k", "static")))
	span.End()

	set := attribute.NewSet(rec.Ended()[0].Attributes()...)
	v, _ := set.Value(attribute.Key("k"))
	assert.String(t, v.AsString()).Equal("carried")
}

// captureExporter records the spans handed to it, for tests that have to go
// through the real provider construction rather than a hand-built one.
type captureExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *captureExporter) ExportSpans(_ context.Context, s []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, s...)
	return nil
}

func (e *captureExporter) Shutdown(context.Context) error { return nil }

func (e *captureExporter) collected() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.spans...)
}

// TestNewTracerProviderWiresContextAttributes is the wiring test: it goes
// through NewTracerProvider itself, so forgetting to register the processor
// there fails here even though the processor's own tests still pass.
func TestNewTracerProviderWiresContextAttributes(t *testing.T) {
	const name = "test-capture"
	exp := &captureExporter{}
	RegisterSpanExporter(name, func(TraceConfig) (sdktrace.SpanExporter, error) {
		return exp, nil
	})

	tp, err := NewTracerProvider(
		TraceConfig{Exporter: name, SamplerRatio: 1.0},
		mustResource(t),
	)
	assert.Error(t, err).Nil()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx := observability.WithContextAttributes(context.Background(), attribute.String("tenant", "t1"))
	_, span := tp.Tracer("test").Start(ctx, "op")
	span.End()
	assert.Error(t, tp.ForceFlush(context.Background())).Nil()

	spans := exp.collected()
	assert.Number(t, len(spans)).Equal(1)
	assert.String(t, attrsToString(spans[0].Attributes())).Equal("tenant=t1")
}
