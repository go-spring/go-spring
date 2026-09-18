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

	"go-spring.org/cloud/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// contextAttrsProcessor applies the attributes a context carries (see
// observability.WithContextAttributes) to every span started with it.
//
// It is what makes those attributes need no cooperation from instrumentation
// points. The OTel SDK calls OnStart with the context the caller passed to
// tracer.Start -- "Use original context", sdk/trace/tracer.go -- so one
// processor installed on the provider covers every span in the process,
// including the ones the framework starts internally.
//
// The carrier itself lives in cloud/observability: what a context carries is a
// contract, and a writer must not have to depend on the OTel SDK to put
// something on a context. This processor is the reader, and the SDK is needed
// only here, where the provider is built.
type contextAttrsProcessor struct{}

// OnStart implements sdktrace.SpanProcessor.
func (contextAttrsProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if attrs := observability.ContextAttributes(parent); len(attrs) > 0 {
		s.SetAttributes(attrs...)
	}
}

// OnEnd implements sdktrace.SpanProcessor.
func (contextAttrsProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

// Shutdown implements sdktrace.SpanProcessor. It holds no resources, so there is
// nothing to release.
func (contextAttrsProcessor) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdktrace.SpanProcessor. It holds no resources, so there
// is nothing to flush.
func (contextAttrsProcessor) ForceFlush(context.Context) error { return nil }

// ContextAttributesProcessor returns the SpanProcessor that applies the
// attributes a context carries (see observability.WithContextAttributes) to
// every span started with it.
//
// [NewTracerProvider] registers one automatically, so an application that lets
// this starter build its TracerProvider needs to do nothing. It is exported for
// the other case: an application that builds its own TracerProvider must
// register this processor itself, or the carrier has no reader and is inert.
func ContextAttributesProcessor() sdktrace.SpanProcessor { return contextAttrsProcessor{} }
