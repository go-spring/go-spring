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

// recordingFunc is a service implementation: it records the context it ran
// with and consumes the message end.
type recordingFunc struct {
	lastCtx context.Context
	runs    int
}

func (f *recordingFunc) Process(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
	f.runs++
	f.lastCtx = ctx
	if err := in.ReadMessageEnd(ctx); err != nil {
		return false, thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
	}
	return true, nil
}

// stubProcessor mimics what a generated thrift processor does: read the message
// begin, look the method up in its map, dispatch to it. The observation
// middleware rides exactly that dispatch — which is why the map must be
// populated here: thrift.WrapProcessor wraps the functions present in it, and a
// processor with an empty map gets no observation at all.
type stubProcessor struct {
	methods map[string]thrift.TProcessorFunction
}

func newStubProcessor(method string, fn thrift.TProcessorFunction) *stubProcessor {
	return &stubProcessor{methods: map[string]thrift.TProcessorFunction{method: fn}}
}

func (s *stubProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	name, _, seqID, err := in.ReadMessageBegin(ctx)
	if err != nil {
		return false, thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
	}
	fn, ok := s.methods[name]
	if !ok {
		return false, thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, name)
	}
	return fn.Process(ctx, seqID, in, out)
}

func (s *stubProcessor) ProcessorMap() map[string]thrift.TProcessorFunction { return s.methods }

func (s *stubProcessor) AddToProcessorMap(key string, fn thrift.TProcessorFunction) {
	s.methods[key] = fn
}

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

// spanNamed returns the single recorded span with the given name.
func spanNamed(t *testing.T, rec *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, s := range rec.Ended() {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

// TestHeaderProtocolTracePropagation verifies that when the server protocol is
// THeaderProtocol, the server span is parented to the client span whose context
// was injected into the message headers, and that the extracted span context
// reaches the service function via ctx.
//
// It also pins the span NAME to the method. That is the point of instrumenting
// on the per-method seam: a single Process-level span cannot know the method and
// would have to call every span "thrift.process".
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

	svc := &recordingFunc{}
	proc := thrift.WrapProcessor(newStubProcessor("Ping", svc), Observe())
	serverIn := thrift.NewTHeaderProtocolConf(wire, nil)
	serverOut := thrift.NewTHeaderProtocolConf(thrift.NewTMemoryBuffer(), nil)

	if ok, ex := proc.Process(context.Background(), serverIn, serverOut); !ok || ex != nil {
		t.Fatalf("Process: ok=%v ex=%v", ok, ex)
	}
	span.End()

	wantSC := span.SpanContext()
	if !trace.SpanFromContext(svc.lastCtx).SpanContext().IsValid() {
		t.Fatal("the service function's ctx does not carry a valid span")
	}

	serverSpan := spanNamed(t, rec, "Ping")
	if serverSpan == nil {
		t.Fatal(`span named after the method ("Ping") not recorded`)
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
	proc := thrift.WrapProcessor(newStubProcessor("Ping", &recordingFunc{}), Observe())
	serverIn := thrift.NewTBinaryProtocolConf(wire, nil)
	serverOut := thrift.NewTBinaryProtocolConf(thrift.NewTMemoryBuffer(), nil)

	if ok, ex := proc.Process(context.Background(), serverIn, serverOut); !ok || ex != nil {
		t.Fatalf("Process: ok=%v ex=%v", ok, ex)
	}

	serverSpan := spanNamed(t, rec, "Ping")
	if serverSpan == nil {
		t.Fatal("server span not recorded")
	}
	if serverSpan.Parent().IsValid() {
		t.Errorf("binary-protocol server span should be a root span, got parent %s", serverSpan.Parent().SpanID())
	}
	if serverSpan.SpanContext().TraceID() == span.SpanContext().TraceID() {
		t.Error("server span must not share the client trace (no propagation channel)")
	}
}

// rejecting denies every call without reaching the service — the shape of an
// inbound admission rejection.
var rejecting thrift.ProcessorMiddleware = func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thrift.WrappedTProcessorFunction{
		Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
			return false, thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "rejected")
		},
	}
}

// TestRejectionIsStillObserved guards the ORDER promise: with Observe listed
// first it is the outermost middleware, so a call a later middleware rejects
// still returns through it and is traced. Put admission one layer above the
// per-method functions instead and a rejection short-circuits before any of
// them runs, leaving no trace at all.
func TestRejectionIsStillObserved(t *testing.T) {
	rec := setupTestTracer(t)

	wire := thrift.NewTMemoryBuffer()
	sender := thrift.NewTBinaryProtocolConf(wire, nil)
	ctx := context.Background()
	if err := sender.WriteMessageBegin(ctx, "Ping", thrift.CALL, 1); err != nil {
		t.Fatalf("WriteMessageBegin: %v", err)
	}
	if err := sender.WriteMessageEnd(ctx); err != nil {
		t.Fatalf("WriteMessageEnd: %v", err)
	}

	svc := &recordingFunc{}
	proc := thrift.WrapProcessor(newStubProcessor("Ping", svc), Observe(), rejecting)
	serverIn := thrift.NewTBinaryProtocolConf(wire, nil)
	serverOut := thrift.NewTBinaryProtocolConf(thrift.NewTMemoryBuffer(), nil)

	ok, ex := proc.Process(context.Background(), serverIn, serverOut)
	if ok || ex == nil {
		t.Fatalf("a rejected call must fail: ok=%v ex=%v", ok, ex)
	}
	if svc.runs != 0 {
		t.Fatalf("a rejected call must not reach the service, ran %d times", svc.runs)
	}
	if spanNamed(t, rec, "Ping") == nil {
		t.Fatal("a rejected call must still be traced — Observe has to be the outermost middleware")
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
