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

package observability

import (
	"context"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/resilience"
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// WrapServerExecutor returns a [chain.Executor] that wraps inner with an
// internal span, the inbound metrics, and an access log per request —
// the server-side counterpart of [WrapClientExecutor], applied by [Manager.ServerExecutorFor]
// at the same point and for the same reason (the executor is still private when it
// is wrapped, so the breaker-listener handshake below can reach it).
//
// The inbound instruments are a separate family (resilience.server.*); see
// [instrumentSet] for why. The access log reuses the resilience tag, with
// system/service naming the route.
func WrapServerExecutor(inner chain.Executor, system, service string) chain.Executor {
	if inner == nil {
		return nil
	}
	w := &wrappedServerExecutor{
		inner: inner,
		serverEmitter: &serverEmitter{
			system:  system,
			service: service,
			ins:     executorInstruments(),
		},
	}
	// Construction-time handshake, exactly as in WrapClientExecutor: the breaker does
	// not exist yet, so the listener is installed on the still-private executor
	// and is in place for every transition of its life.
	if setter, ok := inner.(resilience.BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedServerExecutor struct {
	inner chain.Executor
	*serverEmitter
}

// serverEmitter produces the standard inbound signals for ONE route:
// the request span, the request-level duration, the in-flight gauge, the
// always-on resilience.server.* counter and one access log. It is shared by
// [wrappedServerExecutor] — which runs a governance executor around the request —
// and [ServerObserver], the self-emission path for an inbound route whose server
// has no seam for that executor.
type serverEmitter struct {
	system  string
	service string

	// ins is the shared instrument set; the tracer is looked up per use, for the
	// reason given on [wrappedClientExecutor].
	ins *executorInstrumentSet
}

// OnBreakerStateChange satisfies [BreakerEventListener] for the inbound breaker. A
// trip (→open) is a route-level degradation worth flagging at Warn; recovery and
// half-open trial are Info — the same levelling the outbound listener uses.
func (w *wrappedServerExecutor) OnBreakerStateChange(service string, from, to resilience.BreakerState) {
	w.ins.serverBreakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	fields := []log.Field{
		log.String("system", w.system),
		log.String("service", w.service),
		log.String("from", from.String()),
		log.String("to", to.String()),
	}
	if to == resilience.BreakerOpen {
		log.Warn(context.Background(), resilienceTag, "resilience: circuit breaker opened", fields...)
	} else {
		log.Info(context.Background(), resilienceTag, "resilience: circuit breaker state changed", fields...)
	}
}

// opServerInstrumentSet is the INBOUND counterpart: the two signals a declared
// server route reports under, named the way the HTTP and RPC families already
// name them. There is no attempt histogram — inbound serving has no retry stage
// to measure, by construction (see [ServerPolicy]).
type opServerInstrumentSet struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// opServerInstruments memoizes one [opServerInstrumentSet] per prefix, for the
// same reason and with the same lifetime as [opInstruments].
var opServerInstruments = struct {
	mu sync.Mutex
	m  map[string]*opServerInstrumentSet
}{m: map[string]*opServerInstrumentSet{}}

// serverOperationInstruments returns the inbound instruments a declared server
// route reports under. The suffixes are `.request.duration` and
// `.active_requests` — the families' established names (http.server.request.
// duration, rpc.server.request.duration) — rather than the client side's
// `.operation.duration`: an inbound signal measures a request, and renaming it
// would break every dashboard that already groups by those names.
func serverOperationInstruments(prefix string) *opServerInstrumentSet {
	opServerInstruments.mu.Lock()
	defer opServerInstruments.mu.Unlock()
	if s, ok := opServerInstruments.m[prefix]; ok {
		return s
	}
	m := otel.Meter(componentName)
	s := &opServerInstrumentSet{}
	s.duration, _ = m.Float64Histogram(prefix+".request.duration",
		metric.WithDescription("Duration of one inbound request"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(DurationBuckets()...))
	s.active, _ = m.Int64UpDownCounter(prefix+".active_requests",
		metric.WithDescription("Number of inbound requests currently being served"),
		metric.WithUnit("{request}"))
	opServerInstruments.m[prefix] = s
	return s
}

// Execute wraps the inner call in an internal span, records the duration histogram
// and the status-classified request counter, and writes the access log: a
// rejection or error at Warn, a success at Debug.
func (w *wrappedServerExecutor) Execute(ctx context.Context, op chain.Operation, fn func(context.Context) error) error {
	return w.call(ctx, op, func(c context.Context) error { return w.inner.Execute(c, op, fn) })
}

// call runs one observed inbound request and emits its signals. invoke is the
// request itself: for [wrappedServerExecutor] it enters the governance executor,
// for [ServerObserver] it is the handler chain.
func (e *serverEmitter) call(ctx context.Context, op chain.Operation, invoke func(context.Context) error) error {
	start := time.Now()
	// A described route carries a non-empty metric prefix; a route that declares
	// no description (the zero Operation) takes the fallback path below.
	hasOp := op.Metric != ""
	// The half of an inbound operation that does not exist yet: the handler records
	// what only it knows once it has answered — the response status code — and the
	// emitter reads it back below. A client declares its whole operation up front;
	// a server cannot, and this is the holder that closes that gap.
	ctx, resp := WithResponse(ctx)

	ctx, span := otel.Tracer(componentName).Start(ctx, e.serverSpanName(op, hasOp),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(e.serverSpanAttrs(op, hasOp)...))

	var ins *opServerInstrumentSet
	if hasOp {
		ins = serverOperationInstruments(op.Metric)
		ins.active.Add(ctx, 1, metric.WithAttributes(e.serverLabels(op.Attrs, "", nil)...))
	}

	err := invoke(ctx)
	status := resilience.ClassifyStatus(err)
	respAttrs := resp.Attributes()

	// The response half travels the same two channels the declaration's Detail
	// does — span and log — while the bounded part of it (a status code) also
	// labels the family metric on the line below.
	span.SetAttributes(attribute.String("status", status))
	if len(respAttrs) > 0 {
		span.SetAttributes(respAttrs...)
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	dur := time.Since(start)
	durMs := float64(dur.Nanoseconds()) / 1e6

	// resilience.server.* is the inbound layer's own signal and keeps its shape
	// whether or not the route declared an operation.
	labels := []attribute.KeyValue{
		attribute.String("system", e.system),
		attribute.String("service", e.service),
		attribute.String("status", status),
	}
	if outcome := classifyOutcome(err); outcome != "" {
		labels = append(labels, attribute.String("resilience.outcome", outcome))
	}
	e.ins.serverDuration.Record(ctx, dur.Seconds(), metric.WithAttributes(labels...))
	e.ins.serverCalls.Add(ctx, 1, metric.WithAttributes(labels...))

	if ins != nil {
		// The declared family metrics: the route's own bounded vocabulary, the
		// response half, and the shared status axis. The family names are the HTTP
		// family's — `<prefix>.request.duration` / `<prefix>.active_requests`, not
		// the client side's `.operation.duration` — because a server's signal is a
		// request, and those are the names every dashboard already reads.
		opLabels := e.serverLabels(op.Attrs, status, respAttrs)
		ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(opLabels...))
		ins.active.Add(ctx, -1, metric.WithAttributes(e.serverLabels(op.Attrs, "", nil)...))
	}

	tag := resilienceTag
	fields := []log.Field{
		log.String("system", e.system),
		log.String("service", e.service),
	}
	if hasOp {
		if op.LogTag != nil {
			tag = op.LogTag
		}
		fields = append(attrsToLogFields(op.Attrs), attrsToLogFields(op.Detail)...)
		fields = append(fields, attrsToLogFields(respAttrs)...)
	}
	fields = append(fields, log.String("status", status), log.Float("duration_ms", durMs))

	if err != nil {
		log.Warn(ctx, tag, "server call failed", append(fields, log.Err(err))...)
		return err
	}
	if successQuiet(hasOp, op) {
		log.Debug(ctx, tag, "server call succeeded", func() []log.Field { return fields })
		return nil
	}
	log.Info(ctx, tag, "server call finished", fields...)
	return nil
}

// serverSpanName names an inbound span: the route's own word when it declared
// one, the route service otherwise — which is what this layer's spans have
// always been named in the undeclared case.
func (e *serverEmitter) serverSpanName(op chain.Operation, hasOp bool) string {
	if hasOp && op.Name != "" {
		return op.Name
	}
	return e.service
}

// serverSpanAttrs is the route's attributes and detail when it declared an
// operation, and this layer's own inbound labels otherwise.
func (e *serverEmitter) serverSpanAttrs(op chain.Operation, hasOp bool) []attribute.KeyValue {
	if hasOp {
		out := make([]attribute.KeyValue, 0, len(op.Attrs)+len(op.Detail))
		out = append(out, op.Attrs...)
		return append(out, op.Detail...)
	}
	return []attribute.KeyValue{
		attribute.String("resilience.system", e.system),
		attribute.String("resilience.service", e.service),
		attribute.String("resilience.direction", "inbound"),
	}
}

// serverLabels builds an inbound family metric's labels: the declared bounded
// attrs, then whatever the handler recorded as it answered, then the shared
// status axis. Unlike the client side there is no service label — a server's
// identity is its route, and adding one would change the label set every
// existing HTTP dashboard groups by.
//
// A fresh slice, for the same reason the client side needs one: attrs comes off
// the context and is shared.
func (e *serverEmitter) serverLabels(attrs []attribute.KeyValue, status string, resp []attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs)+len(resp)+1)
	out = append(out, attrs...)
	out = append(out, resp...)
	if status == "" {
		return out
	}
	return append(out, attribute.String("status", status))
}

// ServerObserver is the self-emission entry for an inbound route whose server has
// no seam for the resilience executor. It emits exactly what
// [WrapServerExecutor] emits (the same request span, metric names, labels and
// access log), and applies NO governance: there is no rate limit, circuit
// breaker or bulkhead on this path.
type ServerObserver struct {
	*serverEmitter
}

// NewServerObserver builds the inbound observer for one route system/service.
// system is the server's name ("http-server", "echo"); service is the route label.
func NewServerObserver(system, service string) *ServerObserver {
	return &ServerObserver{serverEmitter: &serverEmitter{
		system:  system,
		service: service,
		ins:     executorInstruments(),
	}}
}

// Observe runs fn as one observed inbound request, emitting the request span, the
// request-level duration, the in-flight gauge, the resilience.server.* counter and
// the access log. The response half is read back from [ResponseFrom]'s recorder,
// which the handler fills as it answers.
func (o *ServerObserver) Observe(ctx context.Context, op chain.Operation, fn func(context.Context) error) error {
	return o.call(ctx, op, fn)
}

func (w *wrappedServerExecutor) Close() error { return w.inner.Close() }
