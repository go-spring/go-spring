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
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/filter"
)

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-trpc"

// rpcSystem is the value the RPC family's rpc.system label carries for this
// backend. Together with rpc.method and status it is one of the three keys
// every RPC backend emits under the same name, because a query spanning
// frameworks has nothing else to join on.
const rpcSystem = "trpc"

// accessTag is the static log tag for the tRPC access log. It is distinct from
// the tag the framework's own logs are bridged under (internal/logger): that
// one carries whatever tRPC logs, this one carries one line per call.
var accessTag = log.RegisterAppTag("trpc", "access")

// rpcName extracts the fully qualified RPC name from the tRPC context.
func rpcName(ctx context.Context) string {
	msg := trpc.Message(ctx)
	return msg.CalleeServiceName() + "/" + msg.CalleeMethod()
}

// statusOf names the outcome the way the family's status label and log field
// expect — the same two words the other RPC backends use, so a dashboard can
// span frameworks.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// logCall writes the per-call access log. Its identity keys are the ones the
// metrics carry (rpc.system / rpc.method / status), so selecting a failing
// method on a dashboard lands on the lines that explain it. duration_ms and
// error are the line's own payload.
func logCall(ctx context.Context, method, status string, dur time.Duration, err error) {
	fields := []log.Field{
		log.String("rpc.system", rpcSystem),
		log.String("rpc.method", method),
		log.String("status", status),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if err != nil {
		log.Warn(ctx, accessTag, append(fields, log.Err(err))...)
		return
	}
	log.Info(ctx, accessTag, fields...)
}

// TracingServerFilter is a tRPC ServerFilter that starts and ends an OTel
// server span for each RPC request. It uses the global TracerProvider that
// starter-otel installs; without starter-otel this is a no-op pass-through.
func TracingServerFilter() filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		name := rpcName(ctx)
		ctx, span := otel.Tracer(componentName).Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", rpcSystem),
				attribute.String("rpc.service", trpc.Message(ctx).CalleeServiceName()),
				attribute.String("rpc.method", name),
			),
		)
		rsp, err := next(ctx, req)
		span.SetAttributes(attribute.String("status", statusOf(err)))
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		span.End()
		return rsp, err
	}
}

// instrumentSet holds this starter's RPC instruments. It is built once per
// process (see instruments), so nothing is allocated per call.
type instrumentSet struct {
	requestCount     metric.Int64Counter
	requestDuration  metric.Float64Histogram
	requestsInFlight metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before the server starts, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(componentName)
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
	return &instrumentSet{
		requestCount:     requestCount,
		requestDuration:  requestDuration,
		requestsInFlight: requestsInFlight,
	}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// MetricsServerFilter is a tRPC ServerFilter that records request count,
// duration, and in-flight gauge through the global MeterProvider. Metric names
// follow the OTel stable RPC semantic conventions (rpc.server.request.duration);
// the request_count counter and active_requests gauge are kept as complementary
// dimensions (not redundant with the duration histogram).
func MetricsServerFilter() filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		name := rpcName(ctx)
		// The in-flight gauge is recorded before the call, so it cannot carry a
		// status; the completed signals carry the family's shared result axis.
		inflight := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", name),
		)
		instruments().requestsInFlight.Add(ctx, 1, inflight)
		start := time.Now()

		rsp, err := next(ctx, req)

		dur := time.Since(start)
		status := statusOf(err)
		done := metric.WithAttributes(
			attribute.String("rpc.system", rpcSystem),
			attribute.String("rpc.method", name),
			attribute.String("status", status),
		)
		instruments().requestsInFlight.Add(ctx, -1, inflight)
		instruments().requestCount.Add(ctx, 1, done)
		instruments().requestDuration.Record(ctx, dur.Seconds(), done)
		logCall(ctx, name, status, dur, err)
		return rsp, err
	}
}
