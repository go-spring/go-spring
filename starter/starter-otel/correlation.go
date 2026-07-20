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

package StarterOTel

import (
	"context"
	"net/http"

	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// installLogCorrelation wires trace ↔ log correlation by installing
// log.FieldsFromContext, the hook go-spring's log module calls on every record
// to lift structured fields off the context. It reads the active OTel span from
// the context and, when the span context is valid, emits trace_id and span_id as
// log fields — so a log line written during a request carries the same trace_id
// that its span reports to the tracing backend, letting operators pivot from a
// log line to its trace (and back) by id.
//
// This is the third pillar of the unified stack: the log bridges (kitex, kratos,
// goframe, ...) already forward framework logs through the ctx-aware path, but
// nothing populated trace_id/span_id until now. Installing the hook here — in the
// one starter that owns the tracing provider — means importing starter-otel
// lights up correlation everywhere with no per-component wiring, matching the
// "central-define, edge-bridge" model of the rest of the observability layer.
//
// The hook is a process global. It is installed only when tracing is enabled, so
// an app that disables tracing keeps whatever hook it set itself (or none).
func installLogCorrelation() {
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		sc := trace.SpanContextFromContext(ctx)
		if !sc.IsValid() {
			return nil
		}
		return []log.Field{
			log.String("trace_id", sc.TraceID().String()),
			log.String("span_id", sc.SpanID().String()),
		}
	}
}

// installTracePropagation fills the stdlib/discovery trace-injector seam with an
// injector backed by the global OTel propagator (the one just installed from
// ${spring.observability.trace.propagator}, W3C traceparent and/or B3). It lets
// outbound requests carry the active trace context so a downstream service — and
// any mesh sidecar (Istio/Envoy) on the path — joins the same trace instead of
// starting a new one, keeping links intact across a hop.
//
// Like installLogCorrelation, this keeps stdlib free of an OTel dependency: the
// seam is discovery.SetTraceInjector, and importing the one starter that owns
// the propagator lights up propagation everywhere with no per-component wiring.
func installTracePropagation() {
	discovery.SetTraceInjector(func(ctx context.Context, header http.Header) {
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
	})
}
