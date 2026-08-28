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

package StarterThrift

import (
	"context"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// stubProcessor records the context it was invoked with and consumes the
// request message begin (already replayed by replayProtocol when the input is
// a THeaderProtocol) plus the message end. It writes no response, which is
// enough for span-parenting assertions.
type stubProcessor struct {
	lastCtx context.Context
	lastArg thrift.TProtocol
}

func (s *stubProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	s.lastCtx = ctx
	s.lastArg = in
	if err := in.ReadMessageEnd(ctx); err != nil {
		return false, thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
	}
	return true, nil
}

func (s *stubProcessor) ProcessorMap() map[string]thrift.TProcessorFunction { return nil }

func (s *stubProcessor) AddToProcessorMap(key string, fn thrift.TProcessorFunction) {}

// setupTestTracer installs an in-memory SDK tracer provider and the W3C
// propagator on the OTel globals, restoring the previous values on cleanup.
func setupTestTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})
	return rec
}

// writeHeaderMessage writes a single thrift CALL message carrying the given
// headers over a THeaderProtocol and flushes the frame.
func writeHeaderMessage(ctx context.Context, t *testing.T, buf thrift.TTransport, method string, headers thrift.THeaderMap) {
	t.Helper()
	proto := thrift.NewTHeaderProtocolConf(buf, nil)
	for k, v := range headers {
		proto.SetWriteHeader(k, v)
	}
	if err := proto.WriteMessageBegin(ctx, method, thrift.CALL, 1); err != nil {
		t.Fatalf("WriteMessageBegin: %v", err)
	}
	if err := proto.WriteMessageEnd(ctx); err != nil {
		t.Fatalf("WriteMessageEnd: %v", err)
	}
	if err := proto.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

// TestHeaderProtocolTracePropagation verifies that when the server protocol is
// THeaderProtocol, the server span started by WrapProcessor is parented to the
// client span whose context was injected into the message headers, and that
// the extracted span context reaches the inner processor via ctx.
func TestHeaderProtocolTracePropagation(t *testing.T) {
	rec := setupTestTracer(t)

	ctx, span := otel.Tracer("test-client").Start(context.Background(), "client-call")
	headers := thrift.THeaderMap{}
	otel.GetTextMapPropagator().Inject(ctx, tHeaderCarrier{m: headers})
	if headers["traceparent"] == "" {
		t.Fatal("expected traceparent header injected")
	}

	wire := thrift.NewTMemoryBuffer()
	writeHeaderMessage(ctx, t, wire, "Ping", headers)

	stub := &stubProcessor{}
	proc := WrapProcessor(stub)
	serverIn := thrift.NewTHeaderProtocolConf(wire, nil)
	serverOut := thrift.NewTHeaderProtocolConf(thrift.NewTMemoryBuffer(), nil)

	if ok, ex := proc.Process(context.Background(), serverIn, serverOut); !ok || ex != nil {
		t.Fatalf("Process: ok=%v ex=%v", ok, ex)
	}
	span.End()

	wantSC := span.SpanContext()
	if !trace.SpanFromContext(stub.lastCtx).SpanContext().IsValid() {
		t.Fatal("inner processor ctx does not carry a valid span")
	}

	var serverSpan sdktrace.ReadOnlySpan
	for _, s := range rec.Ended() {
		if s.Name() == "thrift.process" {
			serverSpan = s
		}
	}
	if serverSpan == nil {
		t.Fatal("thrift.process span not recorded")
	}
	gotSC := serverSpan.SpanContext()
	if gotSC.TraceID() != wantSC.TraceID() {
		t.Errorf("server span trace ID %s != client %s", gotSC.TraceID(), wantSC.TraceID())
	}
	if serverSpan.Parent().SpanID() != wantSC.SpanID() {
		t.Errorf("server span parent %s != client span ID %s", serverSpan.Parent().SpanID(), wantSC.SpanID())
	}
	if !serverSpan.Parent().IsRemote() {
		t.Error("server span parent should be marked remote")
	}
}

// TestBinaryProtocolNoPropagation verifies that a non-header protocol yields a
// root server span (documented boundary: no header channel on the wire).
func TestBinaryProtocolNoPropagation(t *testing.T) {
	rec := setupTestTracer(t)

	// Inject nothing; build a plain binary-protocol request frame.
	ctx, span := otel.Tracer("test-client").Start(context.Background(), "client-call")
	span.End()

	wire := thrift.NewTMemoryBuffer()
	sender := thrift.NewTBinaryProtocolConf(wire, nil)
	if err := sender.WriteMessageBegin(ctx, "Ping", thrift.CALL, 1); err != nil {
		t.Fatalf("WriteMessageBegin: %v", err)
	}
	if err := sender.WriteMessageEnd(ctx); err != nil {
		t.Fatalf("WriteMessageEnd: %v", err)
	}
	stub := &stubProcessor{}
	proc := WrapProcessor(stub)
	serverIn := thrift.NewTBinaryProtocolConf(wire, nil)
	serverOut := thrift.NewTBinaryProtocolConf(thrift.NewTMemoryBuffer(), nil)

	if ok, ex := proc.Process(context.Background(), serverIn, serverOut); !ok || ex != nil {
		t.Fatalf("Process: ok=%v ex=%v", ok, ex)
	}

	for _, s := range rec.Ended() {
		if s.Name() != "thrift.process" {
			continue
		}
		if s.Parent().IsValid() {
			t.Errorf("binary-protocol server span should be a root span, got parent %s", s.Parent().SpanID())
		}
		if s.SpanContext().TraceID() == span.SpanContext().TraceID() {
			t.Error("server span must not share the client trace (no propagation channel)")
		}
	}
}

// TestReplayProtocol verifies the replay wrapper returns the pre-consumed
// message begin exactly once, then delegates to the inner protocol.
func TestReplayProtocol(t *testing.T) {
	wire := thrift.NewTMemoryBuffer()
	sender := thrift.NewTBinaryProtocolConf(wire, nil)
	ctx := context.Background()
	if err := sender.WriteMessageBegin(ctx, "Ping", thrift.CALL, 7); err != nil {
		t.Fatal(err)
	}
	if err := sender.WriteMessageEnd(ctx); err != nil {
		t.Fatal(err)
	}

	inner := thrift.NewTBinaryProtocolConf(wire, nil)
	if _, _, _, err := inner.ReadMessageBegin(ctx); err != nil {
		t.Fatal(err) // consume like observedProcessor does
	}
	p := &replayProtocol{TProtocol: inner, name: "Ping", typeID: thrift.CALL, seqID: 7}

	name, typeID, seqID, err := p.ReadMessageBegin(ctx)
	if err != nil || name != "Ping" || typeID != thrift.CALL || seqID != 7 {
		t.Fatalf("replayed ReadMessageBegin = %q %v %d %v", name, typeID, seqID, err)
	}
	// Second message written to the same buffer: the next call must delegate.
	if err := sender.WriteMessageBegin(ctx, "Pong", thrift.CALL, 8); err != nil {
		t.Fatal(err)
	}
	if err := sender.WriteMessageEnd(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.ReadMessageEnd(ctx); err != nil {
		t.Fatalf("ReadMessageEnd: %v", err)
	}
	name, _, _, err = p.ReadMessageBegin(ctx)
	if err != nil || name != "Pong" {
		t.Fatalf("delegated ReadMessageBegin = %q %v", name, err)
	}
}

// TestTHeaderCarrier covers the propagation carrier adapter.
func TestTHeaderCarrier(t *testing.T) {
	m := thrift.THeaderMap{"a": "1"}
	c := tHeaderCarrier{m: m}
	if c.Get("a") != "1" || c.Get("b") != "" {
		t.Fatalf("Get: %q %q", c.Get("a"), c.Get("b"))
	}
	c.Set("b", "2")
	if m["b"] != "2" {
		t.Fatal("Set did not write through")
	}
	keys := map[string]bool{}
	for _, k := range c.Keys() {
		keys[k] = true
	}
	if !keys["a"] || !keys["b"] {
		t.Fatalf("Keys: %v", keys)
	}
}
