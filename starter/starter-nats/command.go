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

// command.go is the "command seam" concept of this starter: the InnerConn
// chain wraps publishes and requests, and the hand-written consume wrapper.
//
//	chain    — ObsConn declares the publish's identity, GuardConn runs it under
//	            the resilience executor (the single emitter: span, metrics,
//	            access log), RawConn injects the W3C trace context and makes the
//	            wire call.
//	consume  — Consume keeps a hand-written wrapper: its pipeline runs
//	            PER DELIVERY and in the inverted order (extract the upstream
//	            trace, declare, run under the executor), which the chain's
//	            outside-in composition cannot express — the declare would land
//	            inside the executor's attempt or the extraction outside it. A
//	            custom layer on the head therefore intercepts publishes and
//	            requests, not deliveries.
//
// nats exposes no reject-capable middleware, so the chain is driven at the call
// site rather than threaded in as an interceptor.
package StarterNats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go.opentelemetry.io/otel/propagation"
)

// InnerConn is the seam the connection's publish/request traffic runs through.
// nats.go offers no hook or plugin point and delivers a concrete type, so this
// interface is the ONLY way to modify what happens under the promoted
// PublishMsg/PublishMsgContext/RequestGuarded.
//
// The default chain is the identity layer over the governance layer over a raw
// adapter, and the embedded InnerConn is where a custom layer goes: implement
// this interface (embed the head you found to inherit the methods you do not
// care about), then assign your layer over it. The chain under the layer keeps
// doing its job — the subjects a layer rewrites are what the identity layer
// declares, and the executor still protects every call.
//
// Release follows the chain protocol: every layer takes away its OWN resources
// and passes the flag to the layer under it; [Conn.Close] is the head's
// Release(true).
type InnerConn interface {
	// PublishMsg publishes msg (root span — see the promoted method's doc).
	PublishMsg(msg *nats.Msg) error
	// PublishMsgContext publishes msg with the caller's context.
	PublishMsgContext(ctx context.Context, msg *nats.Msg) error
	// RequestGuarded sends a request/reply under governance.
	RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error)
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawConn is the adapter layer at the tail: it injects the W3C trace context
// into the message (from the attempt ctx the layers above handed down, so the
// traceparent carries the executor's span) and makes the wire call. It holds
// the connection — the instance every call bottoms out in. [NewRawConn] builds
// it.
type RawConn struct {
	conn *nats.Conn
}

// NewRawConn wraps a raw connection as the chain's tail.
func NewRawConn(conn *nats.Conn) *RawConn { return &RawConn{conn: conn} }

// Release drains the connection — with releaseRaw only: draining lets in-flight
// subscriptions finish before the underlying socket closes, which only the
// full teardown wants.
func (r *RawConn) Release(releaseRaw bool) error {
	if !releaseRaw {
		return nil
	}
	return r.conn.Drain()
}

func (r *RawConn) PublishMsg(msg *nats.Msg) error {
	return r.PublishMsgContext(context.Background(), msg)
}

func (r *RawConn) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	injectW3C(ctx, msg)
	return r.conn.PublishMsg(msg)
}

func (r *RawConn) RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return r.conn.RequestWithContext(ctx, subj, data)
}

// GuardConn is the governance layer: it runs every publish/request under the
// resilience executor, which applies rate limiting, breaking and retrying — and
// emits the call's span, metrics and access log from the one point that sees
// the whole call, attempts included. [NewGuardConn] builds it, executor
// included: the executor is the layer's own business end to end — built, used
// and closed inside it.
type GuardConn struct {
	exec chain.Executor
	next InnerConn
}

// NewGuardConn builds the governance layer over next, constructing its own
// executor: params carries the container's facilities (see [cloud.ClientParams])
// and url fixes the governance label its limiter and breaker state scope by.
func NewGuardConn(next InnerConn, url string, params cloud.ClientParams) *GuardConn {
	return &GuardConn{exec: params.ExecutorFor("nats", resilience.ServiceLabel("nats", url)), next: next}
}

// Release hands releaseRaw to the layer under it, and on the FULL teardown
// takes down the executor — the layer's own resource. The executor drains
// first: once it is gone the connection is no longer protected, and there is
// no reason to keep it draining past that point.
func (g *GuardConn) Release(releaseRaw bool) error {
	var execErr error
	if releaseRaw && g.exec != nil {
		execErr = g.exec.Close()
	}
	if err := g.next.Release(releaseRaw); err != nil {
		return err
	}
	return execErr
}

func (g *GuardConn) PublishMsg(msg *nats.Msg) error {
	return g.PublishMsgContext(context.Background(), msg)
}

func (g *GuardConn) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	if g.exec == nil {
		return g.next.PublishMsgContext(ctx, msg)
	}
	return g.exec.Execute(ctx, func(attemptCtx context.Context) error {
		return g.next.PublishMsgContext(attemptCtx, msg)
	})
}

func (g *GuardConn) RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	var reply *nats.Msg
	var err error
	if g.exec == nil {
		reply, err = g.next.RequestGuarded(ctx, subj, data, timeout)
		return reply, err
	}
	err = g.exec.Execute(ctx, func(attemptCtx context.Context) error {
		msg, rerr := g.next.RequestGuarded(attemptCtx, subj, data, timeout)
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

// ObsConn is the identity layer at the head: it names each call — the publish
// or request, with its subject, declared as the call's semantic identity — and
// hands the context down. It emits nothing itself: emission happens in the
// governance layer under it. [NewObsConn] builds it.
type ObsConn struct {
	next InnerConn
}

// NewObsConn builds the identity layer over next.
func NewObsConn(next InnerConn) *ObsConn { return &ObsConn{next: next} }

// Release hands releaseRaw to the layer under it — this layer holds no
// resource.
func (o *ObsConn) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

func (o *ObsConn) PublishMsg(msg *nats.Msg) error {
	return o.PublishMsgContext(context.Background(), msg)
}

func (o *ObsConn) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	return o.next.PublishMsgContext(observability.WithOperation(ctx, operation(opPublish, msg.Subject)), msg)
}

func (o *ObsConn) RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return o.next.RequestGuarded(observability.WithOperation(ctx, operation(opPublish, subj)), subj, data, timeout)
}

// --- Consume: the hand-written per-delivery wrapper (see the file doc) ---

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
		return c.Conn.QueueSubscribe(subject, queue, cb)
	}
	return c.Conn.Subscribe(subject, cb)
}

// wrapConsume adapts a ContextHandler to the ctx-less nats.MsgHandler: it
// extracts the upstream trace into a fresh ctx, declares the consume's
// identity, and runs the handler under the executor — which emits the span,
// metrics and access log. The three steps are fused here because they must run
// in this order INSIDE each delivery (see the file doc).
func (c *Conn) wrapConsume(subject string, handler ContextHandler) nats.MsgHandler {
	guard := c.guard
	return func(nm *nats.Msg) {
		ctx := extractW3C(context.Background(), nm)
		ctx = observability.WithOperation(ctx, operation(opConsume, subject))
		if guard == nil || guard.exec == nil {
			_ = handler(ctx, nm)
			return
		}
		_ = guard.exec.Execute(ctx, func(attemptCtx context.Context) error {
			return handler(attemptCtx, nm)
		})
	}
}

// PublishGuarded publishes data on subj, routed through the resilience
// executor. The publish itself flows through the chain's PublishMsgContext, so
// it is declared and emitted exactly like any other publish. Use RequestGuarded
// when a reply is wanted.
func (c *Conn) PublishGuarded(ctx context.Context, subj string, data []byte) error {
	return c.InnerConn.PublishMsgContext(ctx, &nats.Msg{Subject: subj, Data: data})
}
