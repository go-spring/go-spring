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
	"go-spring.org/cloud/chain"
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

// InnerProducer is the seam a producer's synchronous send runs through, one
// chain per producer. rocketmq-client-go offers no hook or plugin point, so
// this interface is the ONLY way to modify what happens under [GuardedSend].
//
// The default chain is the identity layer over the governance layer over a raw
// adapter, and it is where a custom layer goes: implement this interface (embed
// the head you found to inherit the methods you do not care about), then wrap
// the head [NewGuardedProducer] returns. The chain under the layer keeps doing
// its job — the topics a layer rewrites are what the identity layer declares,
// and the executor still protects every send.
//
// Release follows the chain protocol, with one rocketmq-specific note: the
// producer's own lifecycle belongs to the [Client] registry that created it
// (Close shuts every registered producer down), so the raw layer's Release is a
// pass-through at either depth.
type InnerProducer interface {
	// SendSync sends msg synchronously, blocking until the broker acknowledges.
	SendSync(ctx context.Context, msg *primitive.Message) (*primitive.SendResult, error)
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawProducer is the adapter layer: it injects the W3C trace context into the
// message (from the attempt ctx the layers above handed down, so the
// traceparent carries the executor's span) and makes the synchronous send.
// [NewRawProducer] builds it.
type RawProducer struct{ p rocketmq.Producer }

// NewRawProducer wraps a producer as the chain's tail.
func NewRawProducer(p rocketmq.Producer) *RawProducer { return &RawProducer{p: p} }

// Release is the protocol's pass-through: the producer's shutdown belongs to
// the [Client] registry that created it, not to the chain.
func (r *RawProducer) Release(bool) error { return nil }

func (r *RawProducer) SendSync(ctx context.Context, msg *primitive.Message) (*primitive.SendResult, error) {
	injectTraceContext(ctx, msg)
	return r.p.SendSync(ctx, msg)
}

// GuardProducer is the governance layer: it runs every send under the
// resilience executor (the client's, because producers are caller-created and
// recreated over a client's lifetime while the executor stays scoped to the
// client), which is also the single emitter of the send's span, metrics and
// access log. [NewGuardProducer] builds it.
type GuardProducer struct {
	exec chain.Executor
	next InnerProducer
}

// NewGuardProducer builds the governance layer over next, running every send
// under exec.
func NewGuardProducer(next InnerProducer, exec chain.Executor) *GuardProducer {
	return &GuardProducer{exec: exec, next: next}
}

// Release hands releaseRaw to the layer under it — the executor belongs to the
// client and is closed by the client's Close, not here.
func (g *GuardProducer) Release(releaseRaw bool) error { return g.next.Release(releaseRaw) }

func (g *GuardProducer) SendSync(ctx context.Context, msg *primitive.Message) (*primitive.SendResult, error) {
	var res *primitive.SendResult
	var err error
	if g.exec == nil {
		res, err = g.next.SendSync(ctx, msg)
		return res, err
	}
	err = g.exec.Execute(ctx, func(attemptCtx context.Context) error {
		var serr error
		res, serr = g.next.SendSync(attemptCtx, msg)
		return serr
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ObsProducer is the identity layer at the head: it names the send — the
// publish, with its topic — and hands the context down. It emits nothing
// itself: emission happens in the governance layer under it. [NewObsProducer]
// builds it.
type ObsProducer struct {
	next InnerProducer
}

// NewObsProducer builds the identity layer over next.
func NewObsProducer(next InnerProducer) *ObsProducer { return &ObsProducer{next: next} }

// Release hands releaseRaw to the layer under it — this layer holds no
// resource.
func (o *ObsProducer) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

func (o *ObsProducer) SendSync(ctx context.Context, msg *primitive.Message) (*primitive.SendResult, error) {
	return o.next.SendSync(observability.WithOperation(ctx, operation(opPublish, msg.Topic)), msg)
}

// GuardedSend sends msg synchronously on producer, routed through the
// resilience executor attached to cl when governance is enabled. When
// governance is disabled this behaves exactly like producer.SendSync. On
// rejection (rate-limit or open circuit) the returned error is a resilience
// sentinel and the underlying send is never invoked.
//
// It is a thin convenience over the per-producer chain ([NewGuardedProducer]);
// the publish declares its operation (topic as Detail) and the executor emits
// the span, metrics and access log from that declaration.
//
// The synchronous Producer.SendSync blocks until the broker acknowledges, which
// is the path worth protecting; SendAsync and SendOneWay are intentionally
// untouched.
func GuardedSend(ctx context.Context, cl *Client, producer rocketmq.Producer, msg *primitive.Message) (*primitive.SendResult, error) {
	return cl.GuardedProducer(producer).SendSync(ctx, msg)
}
