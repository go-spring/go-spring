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

// command.go is the "command/operation seam" concept of this starter: the
// per-operation declaration and protection that wraps publishes and consumes.
// Two concerns live here:
//
//	declare    — GuardedSend (and the driver's publish/consume paths in
//	             messaging.go) declare the operation's identity (see [operation])
//	             on the ctx and inject/extract the W3C trace context across the
//	             broker. The span, the metrics and the access log are NOT emitted
//	             here: declaring the identity is this layer's whole job now, and
//	             the resilience executor emits from the one point on the chain
//	             that sees a whole call, retries included.
//	resilience — Client.execute drives the backend-neutral executor (attached by
//	             the constructor, see [NewClient]) through the opt-in GuardedSend
//	             call site, since rocketmq-client-go exposes no reject-capable
//	             middleware. The executor is also the single emitter: it reads the
//	             declared operation off the ctx and opens the span, records the
//	             durations (call-level and attempt-level) and writes the one
//	             access log.
//
// The manual OTel span helpers below (StartProducerSpan / StartConsumerSpan /
// EndSpan) stay for apps that drive a raw send or consumer themselves; a call
// routed through GuardedSend or the driver DECLARES its operation instead and is
// emitted by the resilience layer, so do not wrap both around the same call.
package StarterRocketmq

import (
	"context"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-spring.org/cloud/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// -----------------------------------------------------------------------------
// Tracing (native OTel helpers)
// -----------------------------------------------------------------------------

// rocketmq-client-go has no OTel contrib and no span injection point of its
// own, so message-level tracing is done with small call-site helpers built on
// the OTel API. They ride the global TracerProvider and propagator that
// starter-otel installs; without it they are no-ops and touch no message
// bytes.
//
// RocketMQ carries the W3C trace context in the message user properties,
// which are delivered verbatim to consumers, so producer and consumer spans
// link across services the same way the HTTP/Kafka paths do.
//
// The manual helpers (StartProducerSpan / StartConsumerSpan / EndSpan) are the
// app's own path for a raw send or consumer it drives directly; a publish or
// consume that goes through GuardedSend or the driver DECLARES its operation
// instead and the resilience layer emits the span — the inject/extract helpers
// below move the W3C context inside those declare seams.

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-rocketmq"

// injectTraceContext inserts the current W3C trace context into msg's user
// properties, so a subscriber can continue the trace across the broker. It is
// called inside the declare seam with the attempt ctx the executor hands inward,
// so the traceparent carries the executor's span. With no valid span on ctx (the
// ungoverned path) it writes nothing meaningful.
func injectTraceContext(ctx context.Context, msg *primitive.Message) {
	otel.GetTextMapPropagator().Inject(ctx, msgCarrier{msg})
}

// extractTraceContext pulls the upstream trace context out of props. It returns
// the input ctx unchanged when nothing was propagated. It runs before the
// consume's operation is declared, so the executor's consumer span nests under
// the producer's rather than starting a new trace root.
func extractTraceContext(ctx context.Context, props map[string]string) context.Context {
	if len(props) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(props))
}

// StartProducerSpan starts a producer span for msg and injects the current W3C
// trace context into msg's user properties. Call it right before
// Producer.SendSync and end the returned span once the send completes:
//
//	ctx, span := StarterRocketmq.StartProducerSpan(ctx, msg)
//	_, err := producer.SendSync(ctx, msg)
//	StarterRocketmq.EndSpan(span, err)
func StartProducerSpan(ctx context.Context, msg *primitive.Message) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(componentName)
	ctx, span := tracer.Start(ctx, "rocketmq.produce",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rocketmq"),
			attribute.String("messaging.destination.name", msg.Topic),
			attribute.String("messaging.operation", "publish"),
		),
	)
	otel.GetTextMapPropagator().Inject(ctx, msgCarrier{msg})
	return ctx, span
}

// StartConsumerSpan extracts the upstream trace context carried in ext's user
// properties and starts a consumer span as its child. Call it when a message
// is received and end the returned span once processing finishes:
//
//	ctx, span := StarterRocketmq.StartConsumerSpan(ctx, ext)
//	err = handle(ctx, ext)
//	StarterRocketmq.EndSpan(span, err)
func StartConsumerSpan(ctx context.Context, ext *primitive.MessageExt) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, msgCarrier{&ext.Message})
	tracer := otel.GetTracerProvider().Tracer(componentName)
	ctx, span := tracer.Start(ctx, "rocketmq.consume "+ext.Topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rocketmq"),
			attribute.String("messaging.destination.name", ext.Topic),
			attribute.String("messaging.operation", "receive"),
		),
	)
	return ctx, span
}

// EndSpan records err (if any) on span and ends it. It is a small convenience
// so callers do not have to import the OTel codes package themselves.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// msgCarrier adapts a primitive.Message to the OTel TextMapCarrier interface
// so W3C context can be injected into / extracted from the message user
// properties, mirroring kafka-go's recordCarrier.
type msgCarrier struct {
	msg *primitive.Message
}

var _ propagation.TextMapCarrier = msgCarrier{}

// Get returns the value of the user property with key.
func (c msgCarrier) Get(key string) string { return c.msg.GetProperty(key) }

// Set writes the user property key=value.
func (c msgCarrier) Set(key, value string) { c.msg.WithProperty(key, value) }

// Keys enumerates the user property names.
func (c msgCarrier) Keys() []string {
	m := c.msg.GetProperties()
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// --- driver declare path -----------------------------------------------------
//
// The messaging.Driver's publish and consume declare their operation (see
// observe.go) and run it under the client's executor; the manual span helpers
// above remain for apps that drive the raw client and want explicit control.

// -----------------------------------------------------------------------------
// Resilience guard
// -----------------------------------------------------------------------------

// GuardedSend sends msg synchronously on producer, routed through the
// resilience executor attached to cl when governance is enabled. When
// governance is disabled this behaves exactly like producer.SendSync. On
// rejection (rate-limit or open circuit) the returned error is a resilience
// sentinel and the underlying send is never invoked.
//
// The publish declares its operation (topic as Detail, direction and system as
// Attrs) before the send, so the resilience executor — the single emitter —
// opens the span, records the durations (call-level and attempt-level) and
// writes the access log from the declaration. The W3C trace context is injected
// from the attempt ctx the executor hands inward, so the traceparent carries the
// executor's span and links the broker trace to this call; with no executor the
// injection carries whatever span the caller's ctx holds.
//
// The client (not the producer) carries the executor because producers are
// caller-created and may be recreated over a client's lifetime, while the
// executor is always scoped to the client the starter created. The
// synchronous Producer.SendSync blocks until the broker acknowledges, which
// is the path worth protecting; SendAsync and SendOneWay are intentionally
// untouched.
func GuardedSend(ctx context.Context, cl *Client, producer rocketmq.Producer, msg *primitive.Message) (*primitive.SendResult, error) {
	ctx = observability.WithOperation(ctx, operation(opPublish, msg.Topic))
	var res *primitive.SendResult
	err := cl.execute(ctx, func(attemptCtx context.Context) error {
		injectTraceContext(attemptCtx, msg)
		var serr error
		res, serr = producer.SendSync(attemptCtx, msg)
		return serr
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
