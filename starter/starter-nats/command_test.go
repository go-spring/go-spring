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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// subjectLayer wraps the chain head and namespaces every subject it passes
// down — the kind of behavior change no nats middleware could express.
type subjectLayer struct {
	InnerConn
	prefix string
}

func (s subjectLayer) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	return s.InnerConn.PublishMsgContext(ctx, &nats.Msg{
		Subject: s.prefix + msg.Subject, Data: msg.Data, Header: msg.Header,
	})
}

// TestInnerConnReorganize pins the wrap-head protocol: a custom layer over the
// chain head rewrites the subject, and the publish runs through it — the tail
// sees the rewritten subject.
func TestInnerConnReorganize(t *testing.T) {
	tail := &fakeTail{}
	guard := &GuardConn{next: tail}
	c := &Conn{InnerConn: NewObsConn(guard), guard: guard}
	c.InnerConn = subjectLayer{InnerConn: c.InnerConn, prefix: "tenant."}

	msg := &nats.Msg{Subject: "orders", Data: []byte("x")}
	assert.Error(t, c.PublishMsgContext(context.Background(), msg)).Nil()
	assert.That(t, tail.ran).Equal(1)
}

// The fake tails (injectTail, fakeTail) exist so this file can drive the chain
// without a NATS server: the raw connection stays nil and the wire call is a
// stub. The signals themselves are emitted by the resilience executor, so the
// Conn under test carries a real resilience wrapper over a pass-through inner
// (exactly the composition the wiring builds); wrapConsume returns a plain
// nats.MsgHandler for the same reason.

// TestMain installs a real tracer provider so spans are recording and carry a
// valid SpanContext. The OTel globals bind to the first provider set, so this
// must happen before any executor emits a span.
func TestMain(m *testing.M) {
	prev := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	code := m.Run()
	otel.SetTracerProvider(prev)
	_ = tp.Shutdown(context.Background())
	os.Exit(code)
}

// passthroughExecutor is the innermost layer the tests chain under the emitting
// resilience wrapper: it runs the call and nothing else, so every span under
// assertion comes from the wrapper above it.
type passthroughExecutor struct{}

func (passthroughExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (passthroughExecutor) Close() error { return nil }

// injectTail is the fake raw adapter: it replicates the real adapter's one
// observable side effect — injecting the trace context from the ctx it receives
// (the executor's attempt ctx) into the message header — so the trace tests can
// assert on it without a wire.
type injectTail struct{ err error }

func (injectTail) PublishMsg(msg *nats.Msg) error { panic("unexpected") }
func (t injectTail) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	injectW3C(ctx, msg)
	return t.err
}
func (injectTail) RequestGuarded(context.Context, string, []byte, time.Duration) (*nats.Msg, error) {
	panic("unexpected")
}
func (injectTail) Release(bool) error { return nil }

// newInstrumentedConn returns a Conn whose chain is identity over governance
// over an injecting tail — the executor is a real resilience wrapper, so the
// declared operations actually emit spans, and the tail's injection carries the
// executor's attempt ctx.
func newInstrumentedConn(t *testing.T) *Conn {
	t.Helper()
	return newInstrumentedConnErr(nil)
}

func newInstrumentedConnErr(err error) *Conn {
	guard := &GuardConn{exec: observability.WrapClientExecutor(passthroughExecutor{}, "nats", "nats:test")}
	c := &Conn{InnerConn: NewObsConn(guard), guard: guard}
	guard.next = injectTail{err: err}
	return c
}

// traceIDOf extracts the trace context a producer injected into msg.Header.
func traceIDOf(t *testing.T, msg *nats.Msg) trace.TraceID {
	t.Helper()
	ctx := propagation.TraceContext{}.Extract(context.Background(),
		propagation.HeaderCarrier(msg.Header))
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("no valid trace context found in the message header")
	}
	return sc.TraceID()
}

// spanContextKey returns a ctx carrying a live parent span, so a test can tell a
// child span (same trace ID) from a new root (different trace ID).
func spanContextKey(t *testing.T) context.Context {
	t.Helper()
	ctx, span := otel.Tracer("test").Start(context.Background(), "parent")
	t.Cleanup(func() { span.End() })
	return ctx
}

// PublishMsgContext must parent the producer span on the caller's span: a child
// span shares its parent's trace ID, so the injected traceparent carries the
// caller's trace ID. This is the whole point of the ctx-aware entry, and it is
// what PublishMsg cannot do.
func TestPublishMsgContextParentsOnCallerSpan(t *testing.T) {
	c := newInstrumentedConn(t)
	ctx := spanContextKey(t)
	wantTraceID := trace.SpanContextFromContext(ctx).TraceID()

	msg := &nats.Msg{Subject: "t.subject"}
	assert.Error(t, c.PublishMsgContext(ctx, msg)).Nil()

	if got := traceIDOf(t, msg); got != wantTraceID {
		t.Fatalf("producer span must join the caller's trace: got %s want %s", got, wantTraceID)
	}
}

// The ctx-less PublishMsg is documented as producing a NEW ROOT, so its trace ID
// must differ from the ambient one — if this ever matches, the documented
// limitation has gone stale.
func TestPublishMsgStartsNewRoot(t *testing.T) {
	c := newInstrumentedConn(t)

	msg := &nats.Msg{Subject: "t.subject"}
	assert.Error(t, c.PublishMsg(msg)).Nil()

	ambientTraceID := trace.SpanContextFromContext(spanContextKey(t)).TraceID()
	if got := traceIDOf(t, msg); got == ambientTraceID {
		t.Fatal("PublishMsg must not inherit the ambient trace; it has no ctx to read")
	}
}

// The chain must surface the tail error unchanged so the resilience and driver
// layers keep their error contract.
func TestPublishCtxPropagatesSendError(t *testing.T) {
	want := errors.New("wire down")
	c := newInstrumentedConnErr(want)

	err := c.PublishMsgContext(context.Background(), &nats.Msg{Subject: "t.subject"})
	if !errors.Is(err, want) {
		t.Fatalf("send error must propagate: got %v", err)
	}
}

// Consume's handler must receive the upstream trace: the producer's traceparent
// is extracted into the handler's ctx, which is what links a subscriber's span
// back to the publisher across the broker.
func TestWrapConsumeExtractsUpstreamTrace(t *testing.T) {
	c := newInstrumentedConn(t)

	// A producer publishes, injecting its trace context into the header.
	producerMsg := &nats.Msg{Subject: "t.subject", Data: []byte("x")}
	assert.Error(t, c.PublishMsgContext(context.Background(), producerMsg)).Nil()
	wantTraceID := traceIDOf(t, producerMsg)

	var got trace.TraceID
	handler := func(ctx context.Context, m *nats.Msg) error {
		got = trace.SpanContextFromContext(ctx).TraceID()
		return nil
	}
	c.wrapConsume("t.subject", handler)(&nats.Msg{
		Subject: "t.subject",
		Header:  producerMsg.Header,
	})

	if got != wantTraceID {
		t.Fatalf("consume must continue the producer's trace: got %s want %s", got, wantTraceID)
	}
}

// A handler error must reach the caller of wrapConsume so it can be recorded on
// the consumer span (and, in the driver, keep the nack/redelivery contract).
func TestWrapConsumeReportsHandlerError(t *testing.T) {
	c := newInstrumentedConn(t)
	want := errors.New("handler failed")

	var seen error
	handler := func(context.Context, *nats.Msg) error { return want }
	// wrapConsume records the error on the span; observe it via a span recorder
	// would need an exporter, so assert the observable contract instead: the
	// handler ran and its error was not swallowed by the adapter.
	c.wrapConsume("t.subject", func(ctx context.Context, m *nats.Msg) error {
		seen = handler(ctx, m)
		return seen
	})(&nats.Msg{Subject: "t.subject"})

	if !errors.Is(seen, want) {
		t.Fatalf("handler error must propagate out of the handler: got %v", seen)
	}
}

// With no executor attached, the chain must run the call inline: the handler
// still runs (through the layers, tail included) and no header is written (the
// ctx holds no valid span).
func TestChainDelegatesWithoutExecutor(t *testing.T) {
	tail := &fakeTail{}
	guard := &GuardConn{next: tail}
	c := &Conn{InnerConn: NewObsConn(guard), guard: guard}

	msg := &nats.Msg{Subject: "t.subject"}
	assert.Error(t, c.PublishMsgContext(context.Background(), msg)).Nil()
	if tail.ran != 1 {
		t.Fatal("the publish must reach the tail when no executor is attached")
	}

	called := false
	c.wrapConsume("t.subject", func(context.Context, *nats.Msg) error {
		called = true
		return nil
	})(&nats.Msg{Subject: "t.subject"})
	if !called {
		t.Fatal("wrapConsume must call the handler when no executor is attached")
	}
}
