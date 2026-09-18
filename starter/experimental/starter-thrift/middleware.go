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

// rpcSystem is the value the RPC family's rpc.system label carries for this
// backend. Together with rpc.method and status it is one of the three keys
// every RPC backend emits under the same name, because a query spanning
// frameworks has nothing else to join on.
const rpcSystem = "thrift"

// observer holds one server's instruments. They are built once, when the
// middleware is created (server setup, single-threaded), so nothing is
// allocated or initialized on the hot path — an earlier package-level init flag
// was a data race on first concurrent use.
type observer struct {
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestInflight metric.Int64UpDownCounter
}

// Observe returns the middleware that wraps every method call with an OTel
// span and request metrics. The per-call access log is a separate middleware
// ([AccessLog]) because it is always installed, while this one is behind
// [ObserverConfig].
//
// It is a [thrift.ProcessorMiddleware] rather than a TProcessor wrapper because
// the library hands a middleware the METHOD NAME as its first argument. That is
// the one thing a wrapper around TProcessor.Process cannot see: at that level
// the name exists only inside the protocol frame, and recovering it means
// reading the frame and replaying it by hand. Wrapping the per-method functions
// gives the method to the span name and to the rpc.method label, so traces and
// metrics both identify the method instead of an undifferentiated
// "thrift.process".
//
// Install it OUTERMOST, so a call the admission middleware rejects is still
// traced and counted:
//
//	proc = thrift.WrapProcessor(proc, Observe(), AccessLog(), Admit(label, system))
func Observe() thrift.ProcessorMiddleware {
	m := otel.GetMeterProvider().Meter(meterName)
	requestCount, _ := m.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of Thrift RPC requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := m.Float64Histogram(
		"rpc.server.request.duration",
		metric.WithDescription("Duration of Thrift RPC requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	requestInflight, _ := m.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of Thrift RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	o := &observer{
		requestCount:    requestCount,
		requestDuration: requestDuration,
		requestInflight: requestInflight,
	}
	return o.observe
}

// observe wraps one method's TProcessorFunction.
func (o *observer) observe(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thrift.WrappedTProcessorFunction{
		Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
			// Trace propagation needs request headers, which on the wire means
			// THeaderProtocol. They are readable here without consuming the
			// frame: the generated processor read the message begin before
			// dispatching to this function, so the headers are already parsed.
			// With binary/compact/json protocols there is no header channel at
			// all, so each call becomes a new root span (documented boundary,
			// see USAGE.md).
			if hp, ok := in.(*thrift.THeaderProtocol); ok {
				ctx = otel.GetTextMapPropagator().Extract(ctx, tHeaderCarrier{m: hp.GetReadHeaders()})
			}

			inflight := metric.WithAttributes(
				attribute.String("rpc.system", rpcSystem),
				attribute.String("rpc.method", name),
			)
			o.requestInflight.Add(ctx, 1, inflight)
			start := time.Now()

			ctx, span := otel.Tracer(tracerName).Start(ctx, name,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("rpc.system", rpcSystem),
					attribute.String("rpc.method", name),
				),
			)

			ok, ex := next.Process(ctx, seqID, in, out)

			dur := time.Since(start)
			status := statusOf(ex)
			o.requestInflight.Add(ctx, -1, inflight)
			// status is the family's shared result axis (ok|error). The
			// transport-specific detail is thrift.error_code, on the span —
			// not a second metric label carrying the same two words under a
			// different name.
			done := metric.WithAttributes(
				attribute.String("rpc.system", rpcSystem),
				attribute.String("rpc.method", name),
				attribute.String("status", status),
			)
			o.requestCount.Add(ctx, 1, done)
			o.requestDuration.Record(ctx, dur.Seconds(), done)

			if ex != nil {
				span.SetAttributes(
					attribute.Int("thrift.error_code", int(ex.TExceptionType())),
				)
				span.SetStatus(codes.Error, ex.Error())
				span.RecordError(ex)
			}
			span.End()
			return ok, ex
		},
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
