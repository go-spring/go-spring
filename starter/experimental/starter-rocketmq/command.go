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
// observe layer (OTel tracing helpers + the driver's per-message observers)
// and the resilience guard (GuardedSend on the client's executor).
// rocketmq-client-go exposes no reject-capable middleware, so the guard is an
// opt-in call-site wrapper on the synchronous Producer.SendSync path.
package StarterRocketmq

import (
	"context"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// -----------------------------------------------------------------------------
// Tracing (native OTel helpers)
// -----------------------------------------------------------------------------

// rocketmq-client-go has no OTel contrib and no span injection point of its
// own, so message-level tracing is done here with small call-site helpers
// built on the OTel API. They ride the global TracerProvider and propagator
// that starter-otel installs; without it they are no-ops and touch no message
// bytes.
//
// RocketMQ carries the W3C trace context in the message user properties,
// which are delivered verbatim to consumers, so producer and consumer spans
// link across services the same way the HTTP/Kafka paths do.

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-rocketmq"

// StartProducerSpan starts a producer span for msg and injects the current W3C
// trace context into msg's user properties. Call it right before
// Producer.SendSync and end the returned span once the send completes:
//
//	ctx, span := StarterRocketmq.StartProducerSpan(ctx, msg)
//	_, err := producer.SendSync(ctx, msg)
//	StarterRocketmq.EndSpan(span, err)
func StartProducerSpan(ctx context.Context, msg *primitive.Message) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
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
	tracer := otel.GetTracerProvider().Tracer(tracerName)
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

// --- driver auto path --------------------------------------------------------
//
// The messaging.Driver drives produce/consume through the observers in
// observe.go; the manual span helpers above remain for apps that want explicit
// control.

// -----------------------------------------------------------------------------
// Resilience guard
// -----------------------------------------------------------------------------

// applyResilience builds an executor and attaches it to the Client wrapper.
// This is the RocketMQ seam of resilience: rocketmq-client-go exposes no
// reject-capable middleware, so the executor is driven through an opt-in
// call-site guard (GuardedSend) on the synchronous Producer.SendSync path.
//
// Both halves of the stack come from the container: the executor from the
// injected *resilience.Manager, the fault layer from the injected
// *fault.Injector. The manager's executor already carries the resilience observe
// layer (span + outcome counter + histogram + access log) and the
// limiter/breaker/retry core, so there is nothing to wrap around it here; fault
// wraps the operation fn that executor runs, which is what makes an injected
// fault flow through retry/breaker/timeout exactly as a downstream failure
// would, instead of short-circuiting where none of those mechanisms are in play.
//
// A nil manager is the unwired case — a container without starter-governance
// (the wiring injects it nullably, so it is nil there too), or a standalone
// caller with no container. A fresh unarmed manager is exactly "governance
// off": every resolve is a
// pass-through, so [GuardedSend] runs the send inline. Normalizing here keeps
// every caller free of nil branches. inj is nil-safe: with no injector
// WrapClientExecutor returns the inner executor unchanged, so the fault layer is a
// transparent pass-through.
//
// Resolution is deferred to call time, so the order of this arming relative to
// starter-governance's wiring is irrelevant.
func applyResilience(c Config, cl *Client, service string, mgr *resilience.Manager, inj *fault.Injector) error {
	if mgr == nil {
		mgr = resilience.NewManager()
	}
	exec := fault.WrapClientExecutor(mgr.ClientExecutorFor("rocketmq", service), service, inj)
	cl.exec = exec
	cl.service = service
	return nil
}

// closeResilience closes and detaches the executor behind cl, if any.
func closeResilience(cl *Client) {
	if cl.exec != nil {
		_ = cl.exec.Close()
		cl.exec = nil
	}
}

// GuardedSend sends msg synchronously on producer, routed through the
// resilience executor attached to cl when governance is enabled. When
// governance is disabled this behaves exactly like producer.SendSync. On
// rejection (rate-limit or open circuit) the returned error is a resilience
// sentinel and the underlying send is never invoked.
//
// The client (not the producer) carries the executor because producers are
// caller-created and may be recreated over a client's lifetime, while the
// executor is always scoped to the client the starter created. The
// synchronous Producer.SendSync blocks until the broker acknowledges, which
// is the path worth protecting; SendAsync and SendOneWay are intentionally
// untouched.
func GuardedSend(ctx context.Context, cl *Client, producer rocketmq.Producer, msg *primitive.Message) (*primitive.SendResult, error) {
	var res *primitive.SendResult
	err := cl.execute(ctx, func(ctx context.Context) error {
		var serr error
		res, serr = producer.SendSync(ctx, msg)
		return serr
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
