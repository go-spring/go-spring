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

package observability_test

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// attrsToString renders attributes as "k=v" pairs in slice order, which is the
// order the package promises and therefore the order a duplicate key resolves
// in (later wins). It uses Emit rather than AsString: AsString returns the raw
// string for a STRING value and an empty string for every other type, so a
// non-string assertion written against it would be silently meaningless.
func attrsToString(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestWithSpanAttributesAccumulates proves attributes accumulate down the
// derivation chain rather than replacing one another, so several sources can
// contribute.
func TestWithSpanAttributesAccumulates(t *testing.T) {
	ctx := context.Background()
	ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", "t1"))
	ctx = observability.WithSpanAttributes(ctx, attribute.String("user", "u1"))
	assert.String(t, attrsToString(observability.SpanAttributes(ctx))).Equal("tenant=t1,user=u1")
}

// TestWithSpanAttributesLeavesSiblingsAlone proves a derivation does not
// mutate the context it came from: two branches off one parent stay
// independent.
func TestWithSpanAttributesLeavesSiblingsAlone(t *testing.T) {
	parent := observability.WithSpanAttributes(context.Background(), attribute.String("tenant", "t1"))
	_ = observability.WithSpanAttributes(parent, attribute.String("branch", "b1"))
	assert.String(t, attrsToString(observability.SpanAttributes(parent))).Equal("tenant=t1")
}

// TestSetSpanAttributesReachesTheRunningSpan proves the helper writes to the
// span the context carries, which is the case WithSpanAttributes cannot
// serve: the span already exists, so nothing has to be handed to a reader.
func TestSetSpanAttributesReachesTheRunningSpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	observability.SetSpanAttributes(ctx, attribute.String("tenant", "t1"))
	span.End()

	ended := rec.Ended()
	assert.That(t, len(ended)).Equal(1)
	assert.String(t, attrsToString(ended[0].Attributes())).Equal("tenant=t1")
}

// TestSetSpanAttributesWithoutSpanIsANoOp locks the documented no-op: a context
// outside any span carries a non-recording span, not nil, so the write is
// discarded rather than fatal.
func TestSetSpanAttributesWithoutSpanIsANoOp(t *testing.T) {
	ctx := context.Background()
	assert.That(t, oteltrace.SpanFromContext(ctx).IsRecording()).False()
	observability.SetSpanAttributes(ctx, attribute.String("tenant", "t1"))
}

// TestWithSpanAttributesNoAttrsIsSameContext proves the call is free when
// there is nothing to add, so a call site that may or may not have attributes
// does not pay for a wrapping.
func TestWithSpanAttributesNoAttrsIsSameContext(t *testing.T) {
	ctx := context.Background()
	if observability.WithSpanAttributes(ctx) != ctx {
		t.Fatal("WithSpanAttributes with no attributes should return the context unchanged")
	}
}

// TestWithSpanAttributesDuplicateKeyKeepsEvaluationOrder pins the tie-break
// rule: both copies survive in order, so the later one wins wherever duplicates
// collapse.
func TestWithSpanAttributesDuplicateKeyKeepsEvaluationOrder(t *testing.T) {
	ctx := context.Background()
	ctx = observability.WithSpanAttributes(ctx, attribute.String("k", "outer"))
	ctx = observability.WithSpanAttributes(ctx, attribute.String("k", "inner"))
	assert.String(t, attrsToString(observability.SpanAttributes(ctx))).Equal("k=outer,k=inner")
}

// TestSpanAttributesOnPlainContextIsNil proves a context that carries
// nothing reports so, which is what lets the reader stay inert.
func TestSpanAttributesOnPlainContextIsNil(t *testing.T) {
	assert.String(t, attrsToString(observability.SpanAttributes(context.Background()))).Equal("")
}
