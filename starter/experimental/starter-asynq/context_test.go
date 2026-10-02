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

package StarterAsynq

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// The reader is the real one now that it lives in cloud/observability: this
// module still has no starter-otel dependency, but it no longer has to mirror
// the processor to get one. What this test locks is the half that lives here —
// the enqueue span is started from the ctx the caller passed to Enqueue, so
// attributes carried on that ctx arrive on a span the caller never holds. The
// processor's own wiring into a provider is covered by starter-otel's tests.

// attrsOf flattens a span's attributes. Value.Emit, not AsString: AsString
// renders any non-STRING value as the empty string, which would make an
// assertion on a bool attribute pass vacuously.
func attrsOf(s sdktrace.ReadOnlySpan) map[string]string {
	m := make(map[string]string, len(s.Attributes()))
	for _, a := range s.Attributes() {
		m[string(a.Key)] = a.Value.Emit()
	}
	return m
}

// TestEnqueueSpanCarriesSpanAttributes locks the "reachable without holding
// the span" shape documented in cloud/observability/README.md: Enqueue declares
// the operation's identity on the ctx, and the resilience layer — the executor
// underneath — opens the span from that same ctx, so annotating the ctx is the
// only way in for a caller. The enqueue itself is pointed at a closed port — the
// span is what is under test, and it is opened and closed either way.
func TestEnqueueSpanCarriesSpanAttributes(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(observability.SpanAttributesProcessor()),
		sdktrace.WithSpanProcessor(sr),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	}()

	cp := &gs.ContextProvider{Context: context.Background()}
	c, err := newClient(cp, Config{Addr: "127.0.0.1:1"}, nil,
		resilience.NewManager(), fault.NewInjector(fault.Configs{}, nil))
	assert.Error(t, err).Nil()
	defer func() { _ = c.Client.Close() }()

	ctx := observability.WithSpanAttributes(context.Background(),
		attribute.String("tenant", "acme"))
	_, _ = c.Enqueue(ctx, asynq.NewTask("probe:task", nil))

	var producer sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Name() == "enqueue" {
			producer = s
		}
	}
	if producer == nil {
		t.Fatalf("no enqueue span recorded, got %v", spanNames(sr.Ended()))
	}
	got := attrsOf(producer)
	assert.That(t, got["tenant"]).Equal("acme")
	// The family's own attributes ride the same span.
	assert.That(t, got["messaging.system"]).Equal("asynq")
	assert.That(t, got["messaging.destination.name"]).Equal("probe:task")
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name())
	}
	return names
}
