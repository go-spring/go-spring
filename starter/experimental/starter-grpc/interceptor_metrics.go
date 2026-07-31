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

// MetricsUnaryInterceptor records gRPC request metrics — total count, duration,
// and in-flight gauge — through the global MeterProvider.
func MetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
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

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		attrs := metric.WithAttributes(
			attribute.String("rpc.method", info.FullMethod),
		)
		requestsInFlight.Add(ctx, 1, attrs)
		start := time.Now()

		resp, err := handler(ctx, req)

		code := status.Code(err).String()
		requestCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.grpc.status_code", code),
			),
		)
		requestDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.grpc.status_code", code),
			),
		)
		requestsInFlight.Add(ctx, -1, attrs)
		return resp, err
	}
}

// MetricsStreamInterceptor records gRPC stream metrics — count and duration —
// through the global MeterProvider.
func MetricsStreamInterceptor() grpc.StreamServerInterceptor {
	meter := otel.GetMeterProvider().Meter(meterName)
	streamCount, _ := meter.Int64Counter(
		"rpc.server.stream_count",
		metric.WithDescription("Number of RPC stream requests received"),
		metric.WithUnit("{request}"),
	)
	streamDuration, _ := meter.Float64Histogram(
		"rpc.server.stream_duration",
		metric.WithDescription("Duration of RPC stream requests"),
		metric.WithUnit("s"),
	)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		code := status.Code(err).String()
		streamCount.Add(ss.Context(), 1,
			metric.WithAttributes(
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.grpc.status_code", code),
			),
		)
		streamDuration.Record(ss.Context(), time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.grpc.status_code", code),
			),
		)
		return err
	}
}
