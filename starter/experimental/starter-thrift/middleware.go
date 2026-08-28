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
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-thrift"

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-thrift"

// observedProcessor wraps a thrift.TProcessor, adding OTel tracing and metrics
// around every call. When starter-otel is not imported, the OTel globals are
// no-ops so the wrapper adds negligible overhead.
//
// Metric instruments are created once at WrapProcessor time (server setup, runs
// single-threaded) and stored on the struct, avoiding any lazy init on the hot
// path — the previous package-level metricsInit flag was a data race on first
// concurrent use.
type observedProcessor struct {
	inner           thrift.TProcessor
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestInflight metric.Int64UpDownCounter
}

// WrapProcessor returns a TProcessor that wraps each Process call with an OTel
// span and records request metrics (count, duration). Metric names follow the
// OTel stable RPC semantic conventions (rpc.server.request.duration); the
// request_count counter and active_requests gauge are kept as complementary
// dimensions (not redundant with the duration histogram).
func WrapProcessor(inner thrift.TProcessor) thrift.TProcessor {
	meter := otel.GetMeterProvider().Meter(meterName)
	requestCounter, _ := meter.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of Thrift RPC requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := meter.Float64Histogram(
		"rpc.server.request.duration",
		metric.WithDescription("Duration of Thrift RPC requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	requestInflight, _ := meter.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of Thrift RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	return &observedProcessor{
		inner:           inner,
		requestCounter:  requestCounter,
		requestDuration: requestDuration,
		requestInflight: requestInflight,
	}
}

// tHeaderCarrier adapts a thrift.THeaderMap to propagation.TextMapCarrier so
// the global OTel propagator (W3C trace context by default) can read the
// headers a THeaderProtocol client attached to the request message.
type tHeaderCarrier struct {
	m thrift.THeaderMap
}

func (c tHeaderCarrier) Get(key string) string { return c.m[key] }

func (c tHeaderCarrier) Set(key, value string) { c.m[key] = value }

func (c tHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.m))
	for k := range c.m {
		keys = append(keys, k)
	}
	return keys
}

// replayProtocol serves the first ReadMessageBegin call from a message begin
// already consumed by observedProcessor (which needed it to reach the frame
// headers before starting the span), then delegates everything else to the
// inner protocol. All other TProtocol methods delegate untouched.
type replayProtocol struct {
	thrift.TProtocol
	name   string
	typeID thrift.TMessageType
	seqID  int32
	used   bool
}

func (p *replayProtocol) ReadMessageBegin(ctx context.Context) (string, thrift.TMessageType, int32, error) {
	if !p.used {
		p.used = true
		return p.name, p.typeID, p.seqID, nil
	}
	return p.TProtocol.ReadMessageBegin(ctx)
}

func (p *observedProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	// Trace propagation is only possible when the request carries headers,
	// which on the wire means THeaderProtocol (protocol=header). The headers
	// of a frame are only available after its message begin has been read, so
	// the message begin is consumed here, the W3C trace context extracted, and
	// the message begin replayed to the inner processor via replayProtocol.
	// With binary/compact/json protocols there is no header channel at all;
	// each request gets a new root span (documented boundary, see USAGE.md).
	if hp, ok := in.(*thrift.THeaderProtocol); ok {
		name, typeID, seqID, err := hp.ReadMessageBegin(ctx)
		if err != nil {
			return false, thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
		}
		ctx = otel.GetTextMapPropagator().Extract(ctx, tHeaderCarrier{m: hp.GetReadHeaders()})
		in = &replayProtocol{TProtocol: hp, name: name, typeID: typeID, seqID: seqID}
	}

	ctx, span := otel.Tracer(tracerName).Start(ctx, "thrift.process",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("rpc.system", "thrift"),
		),
	)
	defer span.End()

	start, attrs := p.observeStart(ctx)
	ok, ex := p.inner.Process(ctx, in, out)
	p.observeEnd(ctx, start, attrs, ex)

	if ex != nil {
		span.SetAttributes(
			attribute.Int("thrift.error_code", int(ex.TExceptionType())),
		)
		span.SetStatus(codes.Error, ex.Error())
		span.RecordError(ex)
	}
	return ok, ex
}

func (p *observedProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
	return p.inner.ProcessorMap()
}

func (p *observedProcessor) AddToProcessorMap(key string, fn thrift.TProcessorFunction) {
	p.inner.AddToProcessorMap(key, fn)
}

// --- metrics helpers ---
//
// Instruments live on the observedProcessor (created once at WrapProcessor
// time), so these helpers require no lazy init and no synchronization.

func (p *observedProcessor) observeStart(ctx context.Context) (time.Time, metric.MeasurementOption) {
	attrs := metric.WithAttributes(
		attribute.String("rpc.system", "thrift"),
	)
	p.requestInflight.Add(ctx, 1, attrs)
	return time.Now(), attrs
}

func (p *observedProcessor) observeEnd(ctx context.Context, start time.Time, attrs metric.MeasurementOption, ex thrift.TException) {
	// Per the cross-RPC convention (rpc.grpc.status_code, rpc.trpc.status_code),
	// each RPC system names its status attribute "<rpc>.<system>.status_code".
	status := metric.WithAttributes(
		attribute.String("rpc.system", "thrift"),
		attribute.String("rpc.thrift.status_code", statusOf(ex)),
	)
	p.requestCounter.Add(ctx, 1, status)
	p.requestDuration.Record(ctx, time.Since(start).Seconds(), status)
	p.requestInflight.Add(ctx, -1, attrs)
}

// statusOf maps a thrift exception to a coarse status string for metric dims.
func statusOf(ex thrift.TException) string {
	if ex != nil {
		return "error"
	}
	return "ok"
}
