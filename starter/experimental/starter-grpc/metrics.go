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

package StarterGrpc

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-grpc"

// durationBuckets are the explicit histogram bucket boundaries (in seconds) for
// RPC request duration metrics. They match the family-wide convention used by
// starter-gin (observe.go) and the shared observability kit
// (observe/observer.go).
var durationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10,
}

// --- metrics ----------------------------------------------------------------

// MetricsUnaryInterceptor records gRPC request metrics — total count, duration,
// and in-flight gauge — through the global MeterProvider.
//
// The duration histogram follows the OTel STABLE RPC server semconv:
// "rpc.server.request.duration" (dotted). request_count and active_requests are
// kept as starter-specific extras (the stable convention only standardizes the
// duration histogram).
func MetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	meter := otel.GetMeterProvider().Meter(meterName)
	requestCount, _ := meter.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of RPC requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := meter.Float64Histogram(
		"rpc.server.request.duration",
		metric.WithDescription("Duration of RPC requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	requestsInFlight, _ := meter.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		attrs := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", info.FullMethod),
		)
		requestsInFlight.Add(ctx, 1, attrs)
		start := time.Now()

		resp, err := handler(ctx, req)

		code := status.Code(err).String()
		status := statusOf(err)
		done := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("status", status),
			attribute.String("rpc.grpc.status_code", code),
		)
		requestCount.Add(ctx, 1, done)
		dur := time.Since(start)
		requestDuration.Record(ctx, dur.Seconds(), done)
		requestsInFlight.Add(ctx, -1, attrs)
		return resp, err
	}
}

// MetricsStreamInterceptor records gRPC stream metrics — count and duration —
// through the global MeterProvider.
//
// Streams are not covered by the stable rpc.server.request.duration semconv
// here; instead a starter-specific "rpc.server.stream.duration" (dotted) is
// emitted alongside stream_count. Both histograms use the family-wide duration
// bucket boundaries.
func MetricsStreamInterceptor() grpc.StreamServerInterceptor {
	meter := otel.GetMeterProvider().Meter(meterName)
	streamCount, _ := meter.Int64Counter(
		"rpc.server.stream_count",
		metric.WithDescription("Number of RPC stream requests received"),
		metric.WithUnit("{request}"),
	)
	streamDuration, _ := meter.Float64Histogram(
		"rpc.server.stream.duration",
		metric.WithDescription("Duration of RPC stream requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		code := status.Code(err).String()
		status := statusOf(err)
		done := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("status", status),
			attribute.String("rpc.grpc.status_code", code),
		)
		streamCount.Add(ss.Context(), 1, done)
		dur := time.Since(start)
		streamDuration.Record(ss.Context(), dur.Seconds(), done)
		return err
	}
}
