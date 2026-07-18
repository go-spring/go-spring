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

package StarterMongoDB

import (
	"context"
	"strconv"
	"sync"

	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-mongodb"

// newCommandMonitor returns an event.CommandMonitor that emits one client span
// per MongoDB command through the OpenTelemetry global TracerProvider, bridging
// MongoDB into go-spring's unified observability.
//
// Why hand-rolled: the official otelmongo instrumentation
// (go.opentelemetry.io/contrib/.../mongo/otelmongo) targets the v1 mongo driver
// and its event.CommandMonitor type is incompatible with the v2 driver this
// starter uses, so SetMonitor(otelmongo.NewMonitor()) does not compile. The
// bridge is therefore implemented directly here against the v2 event API.
//
// It keeps the same zero-config contract as the other starters: spans go
// through the OTel global provider that starter-otel installs, and when
// starter-otel is absent that global is a no-op, so this costs nothing and
// needs no per-app wiring.
//
// A command's span is opened in Started and closed in Succeeded/Failed. Events
// are correlated by (connection id, request id), which the driver guarantees is
// unique for an in-flight command.
func newCommandMonitor() *event.CommandMonitor {
	var spans sync.Map // string(spanKey) -> trace.Span

	return &event.CommandMonitor{
		Started: func(ctx context.Context, e *event.CommandStartedEvent) {
			tracer := otel.GetTracerProvider().Tracer(tracerName)
			_, span := tracer.Start(ctx, e.CommandName,
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					attribute.String("db.system", "mongodb"),
					attribute.String("db.name", e.DatabaseName),
					attribute.String("db.operation", e.CommandName),
				),
			)
			spans.Store(spanKey(e.ConnectionID, e.RequestID), span)
		},
		Succeeded: func(_ context.Context, e *event.CommandSucceededEvent) {
			if v, ok := spans.LoadAndDelete(spanKey(e.ConnectionID, e.RequestID)); ok {
				v.(trace.Span).End()
			}
		},
		Failed: func(_ context.Context, e *event.CommandFailedEvent) {
			if v, ok := spans.LoadAndDelete(spanKey(e.ConnectionID, e.RequestID)); ok {
				span := v.(trace.Span)
				span.SetStatus(codes.Error, e.Failure.Error())
				span.End()
			}
		},
	}
}

// spanKey uniquely identifies an in-flight command by its connection and
// request id, so the Succeeded/Failed event can find the span Started opened.
func spanKey(connID string, requestID int64) string {
	return connID + "/" + strconv.FormatInt(requestID, 10)
}
