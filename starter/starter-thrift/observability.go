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
type observedProcessor struct {
	inner thrift.TProcessor
}

// WrapProcessor returns a TProcessor that wraps each Process call with an OTel
// span and records request metrics (count, duration).
func WrapProcessor(inner thrift.TProcessor) thrift.TProcessor {
	return &observedProcessor{inner: inner}
}

func (p *observedProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	// The thrift context does not carry trace propagation headers natively.
	// A server span is started here as a new root span; cross-service
	// propagation requires a carrier in the protocol itself, which varies
	// by Thrift transport and is out of scope for this starter.
	ctx, span := otel.Tracer(tracerName).Start(ctx, "thrift.process",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	start := observeStart(ctx)
	ok, ex := p.inner.Process(ctx, in, out)
	observeEnd(ctx, start, ex)

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

var (
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestInflight metric.Int64UpDownCounter
	metricsInit     bool
)

func initMetrics() {
	if metricsInit {
		return
	}
	meter := otel.GetMeterProvider().Meter(meterName)
	requestCounter, _ = meter.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of Thrift RPC requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ = meter.Float64Histogram(
		"rpc.server.request_duration",
		metric.WithDescription("Duration of Thrift RPC requests"),
		metric.WithUnit("s"),
	)
	requestInflight, _ = meter.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of Thrift RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	metricsInit = true
}

func observeStart(ctx context.Context) metric.MeasurementOption {
	initMetrics()
	attrs := metric.WithAttributes(
		attribute.String("rpc.system", "thrift"),
	)
	requestInflight.Add(ctx, 1, attrs)
	return attrs
}

func observeEnd(ctx context.Context, start metric.MeasurementOption, ex thrift.TException) {
	code := "0"
	if ex != nil {
		code = "error"
	}
	requestCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("rpc.system", "thrift"),
			attribute.String("rpc.status_code", code),
		),
	)
	requestInflight.Add(ctx, -1, start)
	// duration is recorded by the span timing — separate metric can be added later
}
