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

	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/stdlib/testing/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestLoadTestProcessorTagsMarkedSpans proves a request tagged as synthetic
// load carries the marker into its spans: without it, a load-test run reached
// the same dashboards as production traffic and looked identical.
func TestLoadTestProcessorTagsMarkedSpans(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	ctx := prop.WithLoadTest(context.Background())
	_, span := tp.Tracer("test").Start(ctx, "op")
	span.End()

	ended := rec.Ended()
	assert.Number(t, len(ended)).Equal(1)
	assert.String(t, attrsToString(ended[0].Attributes())).Equal("load_test=true")
}

// TestLoadTestProcessorLeavesRealTrafficAlone proves the processor is inert on
// ordinary requests -- it must not tag everything.
func TestLoadTestProcessorLeavesRealTrafficAlone(t *testing.T) {
	tp, rec := newTestRecorderProvider()

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	span.End()

	ended := rec.Ended()
	assert.Number(t, len(ended)).Equal(1)
	assert.String(t, attrsToString(ended[0].Attributes())).Equal("")
}

// alwaysBound is the shape a company supplies when its own codebase already
// records synthetic traffic on a context of its own: go-spring's convention with
// the slot swapped for one that calls every context load-test traffic.
func alwaysBound() traffic.Binding {
	b := traffic.DefaultBinding()
	b.Bind = func(ctx context.Context) context.Context { return ctx }
	b.Bound = func(context.Context) bool { return true }
	return b
}

// TestLoadTestProcessorFollowsTheContract proves the processor asks the
// installed [traffic.Propagator] rather than reading the context marker: a
// process that re-bases the convention gets the answer of its own convention.
func TestLoadTestProcessorFollowsTheContract(t *testing.T) {
	prop, err := traffic.NewDefaultPropagator(alwaysBound())
	assert.Error(t, err).Nil()
	SetLoadTestPropagator(prop)
	defer SetLoadTestPropagator(nil)

	tp, rec := newTestRecorderProvider()

	// A plain context: no marker was set, only the installed convention says
	// this is load-test traffic.
	_, span := tp.Tracer("test").Start(context.Background(), "op")
	span.End()

	assert.String(t, attrsToString(rec.Ended()[0].Attributes())).Equal("load_test=true")
}

// TestNewTracerProviderWiresLoadTest is the wiring test: it goes through
// NewTracerProvider itself, so forgetting to register the processor there fails
// here even though the processor's own tests still pass.
func TestNewTracerProviderWiresLoadTest(t *testing.T) {
	const name = "test-capture-loadtest"
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

	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	ctx := prop.WithLoadTest(context.Background())
	_, span := tp.Tracer("test").Start(ctx, "op")
	span.End()
	assert.Error(t, tp.ForceFlush(context.Background())).Nil()

	spans := exp.collected()
	assert.Number(t, len(spans)).Equal(1)
	assert.String(t, attrsToString(spans[0].Attributes())).Equal("load_test=true")
}
