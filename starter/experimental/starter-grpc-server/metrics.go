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

package StarterGrpcServer

import (
	"context"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-grpc-server"

// --- metrics ----------------------------------------------------------------

// instrumentSet holds this starter's RPC instruments. It is built once per
// process (see instruments), so nothing is allocated per call.
type instrumentSet struct {
	requestCount     metric.Int64Counter
	requestDuration  metric.Float64Histogram
	requestsInFlight metric.Int64UpDownCounter
	streamCount      metric.Int64Counter
	streamDuration   metric.Float64Histogram
}

// buildInstruments resolves this starter's instruments against the meter
// provider the otel starter installs before the server is built. The
// interceptors own the set, so a failed registration fails server construction
// instead of silently dropping the whole metric family.
func buildInstruments() (*instrumentSet, error) {
	m := otel.Meter(componentName)
	requestCount, err := m.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of RPC requests received"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "grpc: register rpc.server.request_count")
	}
	requestDuration, err := m.Float64Histogram(
		"rpc.server.request.duration",
		metric.WithDescription("Duration of RPC requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...),
	)
	if err != nil {
		return nil, errutil.Explain(err, "grpc: register rpc.server.request.duration")
	}
	requestsInFlight, err := m.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "grpc: register rpc.server.active_requests")
	}
	streamCount, err := m.Int64Counter(
		"rpc.server.stream_count",
		metric.WithDescription("Number of RPC stream requests received"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "grpc: register rpc.server.stream_count")
	}
	streamDuration, err := m.Float64Histogram(
		"rpc.server.stream.duration",
		metric.WithDescription("Duration of RPC stream requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...),
	)
	if err != nil {
		return nil, errutil.Explain(err, "grpc: register rpc.server.stream.duration")
	}
	return &instrumentSet{
		requestCount:     requestCount,
		requestDuration:  requestDuration,
		requestsInFlight: requestsInFlight,
		streamCount:      streamCount,
		streamDuration:   streamDuration,
	}, nil
}

// MetricsUnaryInterceptor records gRPC request metrics — total count, duration,
// and in-flight gauge — through the global MeterProvider.
//
// The duration histogram follows the OTel STABLE RPC server semconv:
// "rpc.server.request.duration" (dotted). request_count and active_requests are
// kept as starter-specific extras (the stable convention only standardizes the
// duration histogram).
func MetricsUnaryInterceptor() (grpc.UnaryServerInterceptor, error) {
	ins, err := buildInstruments()
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		attrs := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", info.FullMethod),
		)
		ins.requestsInFlight.Add(ctx, 1, attrs)
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
		ins.requestCount.Add(ctx, 1, done)
		dur := time.Since(start)
		ins.requestDuration.Record(ctx, dur.Seconds(), done)
		ins.requestsInFlight.Add(ctx, -1, attrs)
		return resp, err
	}, nil
}

// MetricsStreamInterceptor records gRPC stream metrics — count and duration —
// through the global MeterProvider.
//
// Streams are not covered by the stable rpc.server.request.duration semconv
// here; instead a starter-specific "rpc.server.stream.duration" (dotted) is
// emitted alongside stream_count. Both histograms use the family-wide duration
// bucket boundaries.
func MetricsStreamInterceptor() (grpc.StreamServerInterceptor, error) {
	ins, err := buildInstruments()
	if err != nil {
		return nil, err
	}
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
		ins.streamCount.Add(ss.Context(), 1, done)
		dur := time.Since(start)
		ins.streamDuration.Record(ss.Context(), dur.Seconds(), done)
		return err
	}, nil
}
