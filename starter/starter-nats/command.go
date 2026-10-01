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
// declaration and protection that wraps publishes and consumes. Two concerns
// live here:
//
//	declare    — Conn.PublishMsg / Conn.Consume declare the operation's identity
//	             (see [operation]) on the ctx and inject/extract the W3C trace
//	             context across the broker.
//	resilience — Conn.guard drives the backend-neutral executor
//	             through the opt-in PublishGuarded/RequestGuarded call sites,
//	             since nats exposes no reject-capable middleware. The executor is
//	             also the single emitter: it reads the declared operation off the
//	             ctx and opens the span, records the durations (call-level and
//	             attempt-level) and writes the one access log.
package StarterNats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/observability"
	"go.opentelemetry.io/otel/propagation"
)

// Both directions are reachable without the messaging.Driver:
//
//	publish — Conn.PublishMsgContext(ctx, msg) and its ctx-less twin
//	          Conn.PublishMsg(msg) declare the publish's identity and inject the
//	          W3C trace context into msg.Header so subscribers continue the trace.
//	consume — Conn.Consume(ctx, subject, queue, handler) subscribes with a
//	          context-bearing handler, extracting the upstream trace from the
//	          message header into that ctx and declaring the consume's identity.
//
// The span, the metrics and the access log are not emitted here: declaring the
// identity is this layer's whole job now, and the resilience executor emits from
// the one point on the chain that sees a whole call.
//
// The ctx-less PublishMsg has no caller ctx to parent its span on (nats.go v1.38
// gives it no ctx parameter), so callers with an ambient trace use
// PublishMsgContext. Conn.Consume has no such limitation: its handler receives a
// ctx, and the consume span's parent comes from the W3C header rather than from
// the caller.
//
// The messaging.Driver (messaging.go) adds only envelope conversion and routes
// its publishes through PublishMsgContext and its subscribes through Consume —
// these same entries — so the driver path is declared and emitted here too,
// with no second emitter.

// injectW3C inserts the current trace context into msg.Header so the receiver
// can continue the trace across the broker. With no valid span on ctx (the
// ungoverned path) it writes nothing.
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

// publishCtx is the publish-side core: it declares the publish's identity, then
// runs the send under the executor via [Conn.guard], which opens the span,
// records the metrics and writes the access log. send is the actual wire call,
// injected so the seam can be driven without a live connection (mirrors guard
// below).
//
// The W3C trace context is injected from the attempt ctx the executor hands
// inward, so the traceparent carries the executor's span and links the broker
// trace to this call. With no executor (a stand-alone Conn) guard runs send
// inline and the injection carries whatever span the caller's ctx holds.
func (c *Conn) publishCtx(ctx context.Context, subject string, msg *nats.Msg, send func() error) error {
	ctx = observability.WithOperation(ctx, operation(opPublish, subject))
	return c.guard(ctx, func(attemptCtx context.Context) error {
		injectW3C(attemptCtx, msg)
		return send()
	})
}

// PublishMsg publishes msg. Every publish is declared and routed through the
// resilience executor, which is where its span, metrics and access log are
// emitted. Because nats.go's PublishMsg (v1.38) carries no context parameter,
// the call starts from context.Background(), so the span is a NEW ROOT rather
// than a child of the caller's active trace; use PublishMsgContext when the
// caller has a ctx to link against.
func (c *Conn) PublishMsg(msg *nats.Msg) error {
	return c.publishCtx(context.Background(), msg.Subject, msg,
		func() error { return c.conn.PublishMsg(msg) })
}

// PublishMsgContext publishes msg with the caller's context, so the call's span
// is a child of the caller's active span instead of a new root. The W3C trace
// context is still injected into msg.Header, so subscribers continue the trace
// across the broker either way.
func (c *Conn) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	return c.publishCtx(ctx, msg.Subject, msg,
		func() error { return c.conn.PublishMsg(msg) })
}

// ContextHandler processes one consumed message. Unlike a bare nats.MsgHandler
// it receives a ctx carrying the upstream trace (extracted from the message
// header) and returns an error that flows into the executor's outcome.
type ContextHandler func(ctx context.Context, msg *nats.Msg) error

// Consume subscribes to subject and runs every delivery declared and under the
// resilience executor, extracting the producer's W3C trace context from the
// message header into the ctx passed to handler. A non-empty queue joins a NATS
// queue group (competing consumers); an empty queue is a plain subscription, so
// every Consume caller receives every message.
//
// ctx bounds subscription setup only — the consume span's parent comes from the
// message header, never from this ctx.
func (c *Conn) Consume(ctx context.Context, subject, queue string, handler ContextHandler) (*nats.Subscription, error) {
	cb := c.wrapConsume(subject, handler)
	if queue != "" {
		return c.conn.QueueSubscribe(subject, queue, cb)
	}
	return c.conn.Subscribe(subject, cb)
}

// wrapConsume adapts a ContextHandler to the ctx-less nats.MsgHandler, which is
// where the consume-side declaration has to live. It extracts the upstream trace
// into a fresh ctx, declares the consume's identity, and runs the handler under
// the executor — which emits the span, metrics and access log.
func (c *Conn) wrapConsume(subject string, handler ContextHandler) nats.MsgHandler {
	return func(nm *nats.Msg) {
		ctx := extractW3C(context.Background(), nm)
		ctx = observability.WithOperation(ctx, operation(opConsume, subject))
		_ = c.guard(ctx, func(attemptCtx context.Context) error {
			return handler(attemptCtx, nm)
		})
	}
}

// guard routes call through the executor when one is attached, and otherwise
// runs it inline. Splitting this out keeps the guarded methods trivial and
// makes the pass-through / rejection paths independently testable without a
// live nats server.
func (c *Conn) guard(ctx context.Context, call func(context.Context) error) error {
	if c.exec == nil {
		return call(ctx)
	}
	return c.exec.Execute(ctx, call)
}

// PublishGuarded publishes data on subj, routed through the resilience executor.
// The publish itself flows through PublishMsgContext, so it is declared and
// emitted exactly like any other publish — see that method for the trace
// linkage. Use RequestGuarded when a per-attempt timeout matters.
func (c *Conn) PublishGuarded(ctx context.Context, subj string, data []byte) error {
	return c.PublishMsgContext(ctx, &nats.Msg{Subject: subj, Data: data})
}

// RequestGuarded sends a request/reply on subj, routed through the resilience
// executor when governance is enabled. When governance is disabled
// this behaves exactly like the embedded Request. On rejection (rate-limit or
// open circuit) the returned error is a resilience sentinel and the reply is
// nil; the underlying Request is never invoked.
func (c *Conn) RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	var reply *nats.Msg
	err := c.guard(ctx, func(context.Context) error {
		msg, rerr := c.conn.Request(subj, data, timeout)
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
