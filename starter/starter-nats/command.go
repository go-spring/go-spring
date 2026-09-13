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
//	observe    — Conn.PublishMsg / Conn.Consume wrap every operation with a
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
	"go.opentelemetry.io/otel/propagation"
)

// Both directions are reachable without the messaging.Driver:
//
//	publish — Conn.PublishMsgContext(ctx, msg) and its ctx-less twin
//	          Conn.PublishMsg(msg) wrap the publish with a producer span +
//	          duration/in-flight metric + access log, and inject the W3C trace
//	          context into msg.Header so subscribers continue the trace.
//	consume — Conn.Consume(ctx, subject, queue, handler) subscribes with a
//	          context-bearing handler, extracting the upstream trace from the
//	          message header into that ctx and wrapping the delivery in the
//	          consumer span + metric + access log.
//
// The ctx-less PublishMsg cannot parent its span on the caller (nats.go v1.38
// gives it no ctx parameter), so callers with an ambient trace use
// PublishMsgContext. Conn.Consume has no such limitation: its handler receives a
// ctx, and the consume span's parent comes from the W3C header rather than from
// the caller.
//
// The messaging.Driver (messaging.go) is built on these two entries and adds
// only envelope conversion; it holds no instrumentation of its own.

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

// publishCtx is the publish-side core: it opens the producer span from ctx,
// injects the trace context into msg, and records the outcome. send is the
// actual wire call, injected so the instrumentation can be driven without a
// live connection (mirrors guard below).
//
// When pubObs is nil (the field was never set) it delegates unchanged and pays
// nothing.
func (c *Conn) publishCtx(ctx context.Context, subject string, msg *nats.Msg, send func() error) error {
	if c.pubObs == nil {
		return send()
	}
	ctx, sp := c.pubObs.Start(ctx, "publish", subject)
	injectW3C(ctx, msg)
	err := send()
	sp.End(err)
	return err
}

// PublishMsg overrides the embedded *nats.Conn.PublishMsg so every publish flows
// through the instrumentation (see observe.go). Because nats.go's PublishMsg
// (v1.38) carries no context parameter, the producer span is a NEW ROOT rather
// than a child of the caller's active trace; use PublishMsgContext when the
// caller has a ctx to link against.
func (c *Conn) PublishMsg(msg *nats.Msg) error {
	return c.publishCtx(context.Background(), msg.Subject, msg,
		func() error { return c.Conn.PublishMsg(msg) })
}

// PublishMsgContext publishes msg with the caller's context, so the producer
// span is a child of the caller's active span instead of a new root. The W3C
// trace context is still injected into msg.Header, so subscribers continue the
// trace across the broker either way.
func (c *Conn) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	return c.publishCtx(ctx, msg.Subject, msg,
		func() error { return c.Conn.PublishMsg(msg) })
}

// ContextHandler processes one consumed message. Unlike a bare nats.MsgHandler
// it receives a ctx carrying the upstream trace (extracted from the message
// header) and returns an error that is recorded on the consumer span.
type ContextHandler func(ctx context.Context, msg *nats.Msg) error

// Consume subscribes to subject and runs every delivery through the consumer
// span + duration/in-flight metric + access log, extracting the producer's W3C
// trace context from the message header into the ctx passed to handler. A
// non-empty queue joins a NATS queue group (competing consumers); an empty
// queue is a plain subscription, so every Consume caller receives every message.
//
// ctx bounds subscription setup only — the consumer span's parent comes from the
// message header, never from this ctx.
func (c *Conn) Consume(ctx context.Context, subject, queue string, handler ContextHandler) (*nats.Subscription, error) {
	cb := c.wrapConsume(subject, handler)
	if queue != "" {
		return c.Conn.QueueSubscribe(subject, queue, cb)
	}
	return c.Conn.Subscribe(subject, cb)
}

// wrapConsume adapts a ContextHandler to the ctx-less nats.MsgHandler, which is
// where the consume-side instrumentation has to live. When subObs is nil it
// delegates without parsing the header, so a bare Conn pays nothing.
func (c *Conn) wrapConsume(subject string, handler ContextHandler) nats.MsgHandler {
	return func(nm *nats.Msg) {
		if c.subObs == nil {
			_ = handler(context.Background(), nm)
			return
		}
		octx, sp := c.subObs.Start(extractW3C(context.Background(), nm), "consume", subject)
		err := handler(octx, nm)
		sp.End(err)
	}
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
// zero-code opt-in on the caller side. The publish itself flows through
// PublishMsgContext, so the guarded path keeps the publish span/observer
// instrumentation and the producer span is a child of the caller's trace. Use
// RequestGuarded when a per-attempt timeout matters.
func (c *Conn) PublishGuarded(ctx context.Context, subj string, data []byte) error {
	return c.guard(ctx, func(ctx context.Context) error {
		return c.PublishMsgContext(ctx, &nats.Msg{Subject: subj, Data: data})
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
