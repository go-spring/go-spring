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

package StarterTrpc

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/filter"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-trpc"

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-trpc"

// rpcName extracts the fully qualified RPC name from the tRPC context.
func rpcName(ctx context.Context) string {
	msg := trpc.Message(ctx)
	return msg.CalleeServiceName() + "/" + msg.CalleeMethod()
}

// TracingServerFilter is a tRPC ServerFilter that starts and ends an OTel
// server span for each RPC request. It uses the global TracerProvider that
// starter-otel installs; without starter-otel this is a no-op pass-through.
func TracingServerFilter() filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		name := rpcName(ctx)
		ctx, span := otel.Tracer(tracerName).Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", "trpc"),
				attribute.String("rpc.service", trpc.Message(ctx).CalleeServiceName()),
				attribute.String("rpc.method", name),
			),
		)
		rsp, err := next(ctx, req)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		span.End()
		return rsp, err
	}
}

// MetricsServerFilter is a tRPC ServerFilter that records request count,
// duration, and in-flight gauge through the global MeterProvider.
func MetricsServerFilter() filter.ServerFilter {
	meter := otel.GetMeterProvider().Meter(meterName)
	requestCount, _ := meter.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of RPC requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := meter.Float64Histogram(
		"rpc.server.request_duration",
		metric.WithDescription("Duration of RPC requests"),
		metric.WithUnit("s"),
	)
	requestsInFlight, _ := meter.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)

	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		name := rpcName(ctx)
		attrs := metric.WithAttributes(
			attribute.String("rpc.method", name),
		)
		requestsInFlight.Add(ctx, 1, attrs)
		start := time.Now()

		rsp, err := next(ctx, req)

		requestsInFlight.Add(ctx, -1, attrs)
		requestCount.Add(ctx, 1, attrs)
		requestDuration.Record(ctx, time.Since(start).Seconds(), attrs)
		return rsp, err
	}
}
