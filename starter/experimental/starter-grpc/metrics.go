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
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-grpc"

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

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before the server starts, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(meterName)
	requestCount, _ := m.Int64Counter(
		"rpc.server.request_count",
		metric.WithDescription("Number of RPC requests received"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := m.Float64Histogram(
		"rpc.server.request.duration",
		metric.WithDescription("Duration of RPC requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...),
	)
	requestsInFlight, _ := m.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of RPC requests currently in-flight"),
		metric.WithUnit("{request}"),
	)
	streamCount, _ := m.Int64Counter(
		"rpc.server.stream_count",
		metric.WithDescription("Number of RPC stream requests received"),
		metric.WithUnit("{request}"),
	)
	streamDuration, _ := m.Float64Histogram(
		"rpc.server.stream.duration",
		metric.WithDescription("Duration of RPC stream requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...),
	)
	return &instrumentSet{
		requestCount:     requestCount,
		requestDuration:  requestDuration,
		requestsInFlight: requestsInFlight,
		streamCount:      streamCount,
		streamDuration:   streamDuration,
	}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// MetricsUnaryInterceptor records gRPC request metrics — total count, duration,
// and in-flight gauge — through the global MeterProvider.
//
// The duration histogram follows the OTel STABLE RPC server semconv:
// "rpc.server.request.duration" (dotted). request_count and active_requests are
// kept as starter-specific extras (the stable convention only standardizes the
// duration histogram).
func MetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		attrs := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", info.FullMethod),
		)
		instruments().requestsInFlight.Add(ctx, 1, attrs)
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
		instruments().requestCount.Add(ctx, 1, done)
		dur := time.Since(start)
		instruments().requestDuration.Record(ctx, dur.Seconds(), done)
		instruments().requestsInFlight.Add(ctx, -1, attrs)
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
		instruments().streamCount.Add(ss.Context(), 1, done)
		dur := time.Since(start)
		instruments().streamDuration.Record(ss.Context(), dur.Seconds(), done)
		return err
	}
}
