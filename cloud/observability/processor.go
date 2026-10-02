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

package observability

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// spanAttrsProcessor applies the attributes a context carries (see
// [WithSpanAttributes]) to every span started with it.
//
// It is what makes those attributes need no cooperation from instrumentation
// points. The OTel SDK calls OnStart with the context the caller passed to
// tracer.Start -- "Use original context", sdk/trace/tracer.go -- so one
// processor installed on the provider covers every span in the process,
// including the ones the framework starts internally.
//
// The writer half of this contract needs no OTel SDK — see context.go. The
// reader does, and it lives in the same package because the two are halves of
// one contract; the cost of that choice is that importing the package links the
// SDK even for a caller that only ever writes.
type spanAttrsProcessor struct{}

// OnStart implements sdktrace.SpanProcessor.
func (spanAttrsProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if attrs := SpanAttributes(parent); len(attrs) > 0 {
		s.SetAttributes(attrs...)
	}
}

// OnEnd implements sdktrace.SpanProcessor.
func (spanAttrsProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

// Shutdown implements sdktrace.SpanProcessor. It holds no resources, so there is
// nothing to release.
func (spanAttrsProcessor) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdktrace.SpanProcessor. It holds no resources, so there
// is nothing to flush.
func (spanAttrsProcessor) ForceFlush(context.Context) error { return nil }

// SpanAttributesProcessor returns the [sdktrace.SpanProcessor] that applies the
// attributes a context carries (see [WithSpanAttributes]) to every span started
// with it.
//
// starter-otel's NewTracerProvider registers one automatically, so an
// application that lets it build the TracerProvider needs to do nothing. It is
// exported for the other case: an application that builds its own
// TracerProvider must register this processor itself, or the carrier has no
// reader and is inert.
func SpanAttributesProcessor() sdktrace.SpanProcessor { return spanAttrsProcessor{} }
