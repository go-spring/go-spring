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

package StarterNats

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Why these are call-site helpers rather than a wrapped connection:
//
//  1. nats.go has no official OTel instrumentation. Its API (Publish, Subscribe,
//     Request, QueueSubscribe, plus the JetStream surface) is broad and callback
//     driven, so a wrapper would have to shadow all of it and would still leave
//     the JetStream context — reached via the embedded *nats.Conn — uninstrumented.
//
//  2. nats.Msg carries a Header (nats.Header, since NATS 2.2) that survives the
//     broker round-trip, so W3C trace context propagates cleanly. Instrumenting
//     at the call site — where the caller already builds the *nats.Msg — is what
//     links producer to consumer across services; a connection wrapper cannot.
//
// To propagate context the publisher must send a *nats.Msg (nc.PublishMsg /
// msg.RespondMsg), not the header-less Publish(subject, data). Everything rides
// the OTel globals that starter-otel installs; without it the global
// TracerProvider and propagator are no-ops, so these helpers cost almost nothing
// and change no message bytes.

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-nats"

// StartPublishSpan starts a producer span for msg and injects the current W3C
// trace context into msg.Header so subscribers can continue the trace. Call it
// right before Conn.PublishMsg and end the returned span once the publish
// returns:
//
//	msg := &nats.Msg{Subject: subj, Data: data}
//	ctx, span := StarterNats.StartPublishSpan(ctx, msg)
//	err := conn.PublishMsg(msg)
//	StarterNats.EndSpan(span, err)
func StartPublishSpan(ctx context.Context, msg *nats.Msg) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "nats.publish "+msg.Subject,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", msg.Subject),
			attribute.String("messaging.operation", "publish"),
		),
	)
	if msg.Header == nil {
		msg.Header = nats.Header{}
	}
	otel.GetTextMapPropagator().Inject(ctx, msgCarrier{msg})
	return ctx, span
}

// StartConsumeSpan extracts the upstream trace context carried in msg.Header and
// starts a consumer span as its child. Call it when a message is received and
// end the returned span once processing finishes:
//
//	ctx, span := StarterNats.StartConsumeSpan(ctx, msg)
//	err := handle(ctx, msg)
//	StarterNats.EndSpan(span, err)
func StartConsumeSpan(ctx context.Context, msg *nats.Msg) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, msgCarrier{msg})
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "nats.consume "+msg.Subject,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", msg.Subject),
			attribute.String("messaging.operation", "receive"),
		),
	)
	return ctx, span
}

// EndSpan records err (if any) on span and ends it. It is a small convenience so
// callers do not have to import the OTel codes package themselves.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// msgCarrier adapts a nats.Msg's Header to the OTel TextMapCarrier interface.
// The same type serves both injection (publish) and extraction (consume), since
// nats.Header supports Get/Set symmetrically.
type msgCarrier struct{ msg *nats.Msg }

func (c msgCarrier) Get(key string) string { return c.msg.Header.Get(key) }

func (c msgCarrier) Set(key, value string) { c.msg.Header.Set(key, value) }

func (c msgCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Header))
	for k := range c.msg.Header {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = msgCarrier{}
