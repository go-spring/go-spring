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

// command.go is the "command seam" concept of this starter: the per-operation
// instrumentation and protection that wraps publishes and consumes. Two layers
// live here:
//
//	observe    — Conn.PublishMsg / startConsume wrap every operation with a
//	             producer/consumer span + duration/in-flight metric + access
//	             log, and inject/extract the W3C trace context across the broker.
//	resilience — applyResilience + Conn.guard drive the backend-neutral executor
//	             through the opt-in PublishGuarded/RequestGuarded call sites,
//	             since nats exposes no reject-capable middleware.
package StarterNats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Publish-side instrumentation is transparent: Conn.PublishMsg (below) wraps
// every publish with a producer span + duration/in-flight metric + access log,
// and injects the W3C trace context into msg.Header so subscribers continue the
// trace. Both the messaging.Driver publisher and direct Conn.PublishMsg callers
// flow through it.
//
// Subscribe-side instrumentation lives in the driver callback (driver.go): the
// nats.MsgHandler signature (func(*nats.Msg), no context) means the consume span
// must wrap the handler at the point that already bridges *nats.Msg to a
// context-bearing handler — i.e. the driver. Direct Conn.Subscribe callers are
// not instrumented (documented gap, same as the JetStream surface); use the
// messaging.Driver for traced consume.

// injectW3C inserts the current trace context into msg.Header so the receiver
// can continue the trace across the broker.
func injectW3C(ctx context.Context, msg *nats.Msg) {
	if msg.Header == nil {
		msg.Header = nats.Header{}
	}
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(msg.Header))
}

// extractW3C pulls the upstream trace context out of msg.Header. Returns the
// input ctx unchanged when no context was propagated.
func extractW3C(ctx context.Context, msg *nats.Msg) context.Context {
	if len(msg.Header) == 0 {
		return ctx
	}
	return propagation.TraceContext{}.Extract(ctx, propagation.HeaderCarrier(msg.Header))
}

// PublishMsg overrides the embedded *nats.Conn.PublishMsg so every publish flows
// through the instrumentation (see observe.go). When pubObs is nil (the field
// was never set) it delegates unchanged.
//
// KNOWN LIMITATION: nats.go's PublishMsg (v1.38) carries no context parameter,
// so the producer span necessarily starts from context.Background() — it is a
// NEW ROOT, not a child of the caller's active trace. Consume-side trace
// continuation is unaffected: the span's context is injected into msg.Header
// (injectW3C below), so the driver's consumer span continues this publish span
// across the broker. Callers that need the publish linked into their own trace
// must use the manual ctx-aware path: StartPublishSpan(ctx, msg) (which takes
// the caller's ctx) followed by the embedded c.Conn.PublishMsg(msg) — note that
// escape hatch emits the span only, no metric/access log.
func (c *Conn) PublishMsg(msg *nats.Msg) error {
	if c.pubObs == nil {
		return c.Conn.PublishMsg(msg)
	}
	ctx, sp := c.pubObs.Start(context.Background(), "publish", msg.Subject)
	injectW3C(ctx, msg)
	err := c.Conn.PublishMsg(msg)
	sp.End(err)
	return err
}

// startConsume is the subscribe-side hook the driver callback calls: it extracts
// the upstream trace, opens a consumer span + metric + log, and returns a handle
// whose End records the outcome.
func (c *Conn) startConsume(ctx context.Context, subject string, msg *nats.Msg) (context.Context, *span) {
	if c.subObs == nil {
		return ctx, nil
	}
	ctx2, sp := c.subObs.Start(extractW3C(ctx, msg), "consume", subject)
	return ctx2, sp
}

// --- low-level manual helpers (kept for the example programs and for apps that
// want explicit span control outside the messaging.Driver) -------------------
//
// The auto path above (Conn.PublishMsg + driver) is preferred. These helpers are
// a thin otel-direct escape hatch: they emit a span only (no metric/log) and do
// not require an Observer, so they work with a bare *nats.Conn the app holds.

const tracerName = "go-spring.org/starter-nats"

// StartPublishSpan starts a producer span for msg and injects the current W3C
// trace context into msg.Header. Kept for backward compatibility / manual use.
func StartPublishSpan(ctx context.Context, msg *nats.Msg) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "nats.publish "+msg.Subject,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", msg.Subject),
			attribute.String("messaging.operation", "publish"),
		),
	)
	injectW3C(ctx, msg)
	return ctx, span
}

// StartConsumeSpan extracts the upstream trace from msg.Header and starts a
// consumer span as its child. Kept for backward compatibility / manual use.
func StartConsumeSpan(ctx context.Context, msg *nats.Msg) (context.Context, trace.Span) {
	ctx = extractW3C(ctx, msg)
	ctx, span := otel.Tracer(tracerName).Start(ctx, "nats.consume "+msg.Subject,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", msg.Subject),
			attribute.String("messaging.operation", "receive"),
		),
	)
	return ctx, span
}

// EndSpan records err (if any) on span and ends it.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// applyResilience builds an executor and attaches it to conn. This is the nats
// seam of resilience: because nats exposes no reject-capable middleware (unlike
// redis.Hook or http.RoundTripper), the same backend-neutral Executor is driven
// through opt-in call-site guards (PublishGuarded/RequestGuarded) rather than a
// transparent interceptor. Only the adapter shape differs — the core is reused.
//
// The executor is resolved through the neutral [resilience.ExecutorFor] seam,
// which starter-govern backs with the governance center — so this function has
// zero coupling to cloud/governance. When governance is off, ExecutorFor yields a
// transparent no-op executor; fault wraps it when enabled.
func applyResilience(c Config, conn *Conn, resource string) error {
	exec := fault.WrapExecutor(resilience.ExecutorFor("nats", resource))
	conn.exec = exec
	conn.resource = resource
	return nil
}

// guard routes call through the executor when one is attached, and otherwise
// runs it inline. Splitting this out keeps the guarded methods trivial and
// makes the pass-through / rejection paths independently testable without a
// live nats server.
func (c *Conn) guard(ctx context.Context, call func(context.Context) error) error {
	if c.exec == nil {
		return call(ctx)
	}
	return c.exec.Execute(ctx, c.resource, call)
}

// PublishGuarded publishes data on subj, routed through the resilience executor
// when governance is enabled. When governance is disabled this
// behaves exactly like the embedded Publish, so enabling protection is a
// zero-code opt-in on the caller side. The publish itself flows through the
// overridden PublishMsg, so the guarded path keeps the publish span/observer
// instrumentation; the ctx threads the caller's trace into the executor (the
// producer span still starts from context.Background() — see the PublishMsg
// limitation note above). Use RequestGuarded when a per-attempt timeout
// matters.
func (c *Conn) PublishGuarded(ctx context.Context, subj string, data []byte) error {
	return c.guard(ctx, func(context.Context) error {
		return c.PublishMsg(&nats.Msg{Subject: subj, Data: data})
	})
}

// RequestGuarded sends a request/reply on subj, routed through the resilience
// executor when governance is enabled. When governance is disabled
// this behaves exactly like the embedded Request. On rejection (rate-limit or
// open circuit) the returned error is a resilience sentinel and the reply is
// nil; the underlying Request is never invoked.
func (c *Conn) RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	var reply *nats.Msg
	err := c.guard(ctx, func(context.Context) error {
		msg, rerr := c.Conn.Request(subj, data, timeout)
		if rerr != nil {
			return rerr
		}
		reply = msg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}
