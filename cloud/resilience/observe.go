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

package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// resilienceTag is the static log tag for the resilience access log; the
// protected client's system is a log field, not part of the tag.
var resilienceTag = log.RegisterAppTag("resilience", "")

// scope names the OTel scope this package's spans and instruments
// register under.
const scope = "go-spring.org/cloud/resilience"

// instrumentSet is this package's instruments: one per process, resolved lazily
// on first use so it binds to whichever providers are current then, and immutable
// afterwards. It holds no per-client state — the system and service labels travel
// with each wrapper, not here.
//
// The outbound (resilience.client.*) and inbound (resilience.server.*) families
// are separate instruments rather than one family with a direction attribute: the
// outcome sets genuinely differ (inbound has no retry, so no retry_budget_exceeded
// and no per-attempt timeout), and keeping them apart leaves existing outbound
// dashboards untouched.
type instrumentSet struct {
	clientDuration       metric.Float64Histogram
	clientCalls          metric.Int64Counter
	clientBreakerChanges metric.Int64Counter

	serverDuration       metric.Float64Histogram
	serverCalls          metric.Int64Counter
	serverBreakerChanges metric.Int64Counter
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

func buildInstruments() *instrumentSet {
	m := otel.Meter(scope)
	in := &instrumentSet{}
	in.clientDuration, _ = m.Float64Histogram("resilience.client.duration",
		metric.WithDescription("Duration of resilience-protected calls"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	// calls counts protected calls on the shared status axis (ok/error) plus,
	// where protection intervened, the resilience.outcome that says what it did
	// (rate_limited, circuit_open, bulkhead_full, retry_budget_exceeded, timeout).
	in.clientCalls, _ = m.Int64Counter("resilience.client.calls",
		metric.WithDescription("Number of resilience-protected calls by status"),
		metric.WithUnit("{call}"))
	// breakerChanges counts circuit-breaker state transitions (from/to attrs).
	in.clientBreakerChanges, _ = m.Int64Counter("resilience.client.breaker.state_change",
		metric.WithDescription("Circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))

	in.serverDuration, _ = m.Float64Histogram("resilience.server.duration",
		metric.WithDescription("Duration of inbound requests under resilience admission"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	in.serverCalls, _ = m.Int64Counter("resilience.server.calls",
		metric.WithDescription("Number of inbound requests by admission status"),
		metric.WithUnit("{request}"))
	in.serverBreakerChanges, _ = m.Int64Counter("resilience.server.breaker.state_change",
		metric.WithDescription("Inbound circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	return in
}

// WrapClientExecutor returns an [ClientExecutor] that wraps inner with an internal span
// (system/service/status attributes), the resilience metrics, and an
// access log per call. Pass the system label (e.g. "redis", "gorm", "grpc")
// so calls from several protected clients are distinguishable. A nil inner
// returns nil — no wrapper, so an unarmed client stays untouched.
//
// This is an internal composition step, not a client API: [ClientExecutorFor] applies
// it while the provider-built executor is still private (see [resolve]), so
// clients get an already-observed executor and never wrap one themselves. The
// only other caller is a client that composes its own stack around a raw
// executor it built directly (e.g. httpx's custom ClientExecutor / ResilienceDriver
// paths).
//
// Attaching the breaker listener happens here, while inner is still private:
// the type assertion below is a construction-time handshake, so the executor
// never escapes without its listener and no late binding is needed. The shared
// instruments are resolved at this point — the first resolve time, not package
// init — so an SDK installed after this package's init still receives the spans
// and records. The tracer is not held at all; see [wrappedClientExecutor.Execute].
func WrapClientExecutor(inner ClientExecutor, system, service string) ClientExecutor {
	if inner == nil {
		return nil
	}
	w := &wrappedClientExecutor{
		inner:   inner,
		system:  system,
		service: service,
		ins:     instruments(),
	}
	// If the inner executor emits breaker state transitions, subscribe so each
	// trip / half-open / recovery emits a counter + log automatically. Drivers
	// without that capability (no BreakerEventListenerSetter) are silently
	// skipped — the call-level signals still emit.
	if setter, ok := inner.(BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedClientExecutor struct {
	inner   ClientExecutor
	system  string
	service string

	// ins is the shared instrument set. The tracer is deliberately NOT held
	// alongside it: a captured otel.Tracer stops forwarding once the global
	// provider is set again, so it is looked up per use.
	ins *instrumentSet
}

// OnBreakerStateChange satisfies [BreakerEventListener]. It is invoked
// synchronously from inside the breaker's transition (so it must not call back
// into the executor); it emits a state-change counter and a log line.
func (w *wrappedClientExecutor) OnBreakerStateChange(service string, from, to BreakerState) {
	w.ins.clientBreakerChanges.Add(context.Background(), 1, metric.WithAttributes(
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
	// A trip (→open) is a service-level degradation worth flagging at Warn;
	// recovery and half-open trial are Info.
	if to == BreakerOpen {
		log.Warn(context.Background(), resilienceTag, fields...)
	} else {
		log.Info(context.Background(), resilienceTag, fields...)
	}
}

// opInstrumentSet holds the instruments one metric prefix needs. The prefix
// comes from the operation a client declared (see [observability.Operation]), so
// this set cannot be built once per process the way [instrumentSet] is — a
// process serves any number of client families, each with its own prefix. It is
// memoized per prefix on first use, which is also what binds it to whichever
// meter provider is current then.
type opInstrumentSet struct {
	duration metric.Float64Histogram
	attempts metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// opServerInstrumentSet is the INBOUND counterpart: the two signals a declared
// server route reports under, named the way the HTTP and RPC families already
// name them. There is no attempt histogram — inbound serving has no retry stage
// to measure, by construction (see [ServerPolicy]).
type opServerInstrumentSet struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// opInstruments memoizes one [opInstrumentSet] per metric prefix. It holds no
// per-call state — the system/service/operation labels travel with each record.
var opInstruments = struct {
	mu sync.Mutex
	m  map[string]*opInstrumentSet
}{m: map[string]*opInstrumentSet{}}

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
	m := otel.Meter(scope)
	s := &opServerInstrumentSet{}
	s.duration, _ = m.Float64Histogram(prefix+".request.duration",
		metric.WithDescription("Duration of one inbound request"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	s.active, _ = m.Int64UpDownCounter(prefix+".active_requests",
		metric.WithDescription("Number of inbound requests currently being served"),
		metric.WithUnit("{request}"))
	opServerInstruments.m[prefix] = s
	return s
}

// resetOperationInstruments drops the memoized sets so the next use resolves a
// fresh one. It exists for tests that install their own MeterProvider, for the
// same reason [resetInstruments] does.
func resetOperationInstruments() {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	opInstruments.m = map[string]*opInstrumentSet{}
	opServerInstruments.mu.Lock()
	defer opServerInstruments.mu.Unlock()
	opServerInstruments.m = map[string]*opServerInstrumentSet{}
}

// operationInstruments returns the instrument set for a metric prefix, building
// it on first use. Two names per prefix carry the two data grains: the call-level
// ".operation.duration" (attempts and backoff included — what this service
// promised its caller) and the attempt-level ".attempt.duration" (what the
// downstream itself took, per try). Keeping them separate is the point: mixing
// retry cost into downstream latency makes "raise the backoff" read as "the
// downstream got slower". The active-requests gauge is in-flight CALLS, so it
// lives here beside the call level, not on the attempt.
func operationInstruments(prefix string) *opInstrumentSet {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	if s, ok := opInstruments.m[prefix]; ok {
		return s
	}
	m := otel.Meter(scope)
	s := &opInstrumentSet{}
	s.duration, _ = m.Float64Histogram(prefix+".operation.duration",
		metric.WithDescription("Duration of one logical client operation, retries and backoff included"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	s.attempts, _ = m.Float64Histogram(prefix+".attempt.duration",
		metric.WithDescription("Duration of one downstream attempt inside a client operation"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	s.active, _ = m.Int64UpDownCounter(prefix+".active_requests",
		metric.WithDescription("Number of in-flight client operations"),
		metric.WithUnit("{request}"))
	opInstruments.m[prefix] = s
	return s
}

// Execute is the single emission point of a protected client call. It installs
// the per-call accumulator the retry loop inside the executor writes into, opens
// the call's span BEFORE handing the call inward — so every span the downstream
// opens (an otelhttp transport, a driver's own) nests under it — and, once the
// call returns, emits the call's signals from the two grains the accumulator
// separates: what each attempt cost the downstream, and what the whole call cost
// this caller.
//
// The slice of the chain is what makes this correct: the span is opened outside
// [defaultExecutor.Execute]'s retry loop, so it covers every attempt, and the
// emit runs after the loop, so it reads the attempts the loop recorded. The
// governance layer emits nothing itself; it only writes facts.
//
// A call that declared no operation (see [observability.WithOperation]) takes the
// fallback path below, which reproduces this layer's historical signals verbatim,
// down to their names — migrating a client to the declared-operation path is what
// switches its signals over, one client at a time.
func (w *wrappedClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	start := time.Now()
	ctx, rec := observability.WithRecorder(ctx)
	op, hasOp := observability.OperationFrom(ctx)

	var ins *opInstrumentSet
	if hasOp {
		ins = operationInstruments(op.Metric)
	}

	ctx, span := otel.Tracer(scope).Start(ctx, w.spanName(op, hasOp),
		trace.WithSpanKind(spanKind(op, hasOp)),
		trace.WithAttributes(w.spanAttrs(op, hasOp)...))
	// The in-flight gauge brackets the whole call, retries included, exactly as
	// the span does.
	if ins != nil {
		ins.active.Add(ctx, 1, metric.WithAttributes(w.callLabels(op.Attrs, "", "")...))
	}

	err := w.inner.Execute(ctx, fn)

	if ins != nil {
		ins.active.Add(ctx, -1, metric.WithAttributes(w.callLabels(op.Attrs, "", "")...))
	}
	status := classifyStatus(err)
	span.SetAttributes(attribute.String("status", status))
	if err != nil {
		// Both marks, so a span carries the same evidence whichever emitter
		// produced it: SetStatus is the span's own outcome, RecordError is the
		// exception event a trace UI lists. It records the CALL's error, never an
		// attempt's, so a retried-then-succeeded call still looks clean.
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	w.emit(ctx, start, status, err, op, hasOp, rec)
	return err
}

// emit produces the call-level signals — span (already ended by the caller),
// counter, call histogram, attempt histograms and the one access log. It runs
// after the inner call, so the recorder is complete.
func (w *wrappedClientExecutor) emit(ctx context.Context, start time.Time, status string, err error, op observability.Operation, hasOp bool, rec *observability.Recorder) {
	dur := time.Since(start)
	outcome := classifyOutcome(err)

	// The status-classified call counter is the one signal both paths carry: it
	// answers "was this rejected, or did the downstream fail", which nothing else
	// distinguishes — `status` says it failed, `resilience.outcome` says
	// protection rejected it.
	calls := []attribute.KeyValue{
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("status", status),
	}
	if outcome != "" {
		calls = append(calls, attribute.String("resilience.outcome", outcome))
	}
	w.ins.clientCalls.Add(ctx, 1, metric.WithAttributes(calls...))

	tag := resilienceTag
	fields := []log.Field{
		log.String("system", w.system),
		log.String("service", w.service),
	}
	quietSuccess := successQuiet(hasOp, op)
	if hasOp {
		ins := operationInstruments(op.Metric)
		// Metric labels come from Attrs alone: Detail is deliberately excluded, so
		// a cache key or message subject can never become a label.
		ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(w.callLabels(op.Attrs, status, outcome)...))
		// The attempt layer: the downstream's own latency and outcome, per try.
		// Attempts a rejection returned before running are simply absent — an
		// empty recorder is the call's own signal that the downstream was never
		// touched.
		for _, a := range rec.Attempts() {
			ins.attempts.Record(ctx, a.Duration.Seconds(),
				metric.WithAttributes(w.callLabels(op.Attrs, a.Status, classifyOutcome(a.Err))...))
		}
		if op.LogTag != nil {
			tag = op.LogTag
		}
		// The log carries the operation's own vocabulary — attrs and detail
		// alike — so a dashboard selecting failed operations lands on the lines
		// that explain them.
		fields = append(attrsToLogFields(op.Attrs), attrsToLogFields(op.Detail)...)
		fields = append(fields, log.String("status", status), log.Float("duration_ms", durMs(dur)))
	} else {
		// Fallback: this layer's historical signals, names and labels unchanged.
		labels := []attribute.KeyValue{
			attribute.String("system", w.system),
			attribute.String("service", w.service),
			attribute.String("status", status),
		}
		if outcome != "" {
			labels = append(labels, attribute.String("resilience.outcome", outcome))
		}
		w.ins.clientDuration.Record(ctx, dur.Seconds(), metric.WithAttributes(labels...))
		fields = append(fields, log.Float("duration_ms", durMs(dur)), log.String("status", status))
	}
	if outcome != "" {
		fields = append(fields, log.String("resilience.outcome", outcome))
	}

	if err != nil {
		log.Warn(ctx, tag, append(fields, log.Err(err))...)
		return
	}
	if quietSuccess {
		// Built lazily: the success record is there for troubleshooting, not
		// everyday reading.
		log.Debug(ctx, tag, func() []log.Field { return fields })
		return
	}
	log.Info(ctx, tag, fields...)
}

// successQuiet reports whether a successful call's access line is logged at
// Debug (true) or Info (false).
//
// An UNDECLARED call is this layer's historical quiet case: it has always logged
// success at Debug, and a client that declares nothing (redigo's skipped PING,
// say) declares nothing precisely to keep the noise down — so it stays quiet.
// A declared call is levelled by its detail: one carrying a key or a subject is
// frequent and uninteresting until it fails, one carrying none is worth a line.
func successQuiet(hasOp bool, op observability.Operation) bool {
	if !hasOp {
		return true
	}
	return len(op.Detail) > 0
}

// spanName names the call's span: the operation's own word when the client
// declared one, the service label otherwise — matching what this layer's spans
// have always been named in the undeclared case.
func (w *wrappedClientExecutor) spanName(op observability.Operation, hasOp bool) string {
	if hasOp && op.Name != "" {
		return op.Name
	}
	return w.service
}

// spanKind is the span kind for a call: the one the client declared, so a
// publish and a consume keep the producer/consumer edge they add to the trace,
// and Internal — the kind an in-process client call has always had — when the
// client declared none or left the kind unspecified.
func spanKind(op observability.Operation, hasOp bool) trace.SpanKind {
	if hasOp && op.SpanKind != trace.SpanKindUnspecified {
		return op.SpanKind
	}
	return trace.SpanKindInternal
}

// spanAttrs is the operation's attributes plus its detail when declared, and
// this layer's own system/service labels otherwise. The span carries both: the
// unbounded detail that must stay out of metric labels is exactly what makes a
// span useful to read.
func (w *wrappedClientExecutor) spanAttrs(op observability.Operation, hasOp bool) []attribute.KeyValue {
	if hasOp {
		out := make([]attribute.KeyValue, 0, len(op.Attrs)+len(op.Detail))
		out = append(out, op.Attrs...)
		return append(out, op.Detail...)
	}
	return []attribute.KeyValue{
		attribute.String("resilience.system", w.system),
		attribute.String("resilience.service", w.service),
	}
}

// callLabels builds the metric labels for a declared call: the operation's
// bounded attrs, this instance's service label, and — where the metric is
// outcome-classified — the status.
//
// The service label is what keeps two instances of one backend (two memcached
// pools, two redis instances) from collapsing into a single series: the
// operation's own attrs cannot express it, and it is the dimension the
// undeclared fallback has always carried. Detail is never included, so a key or
// a statement can never become a label.
//
// The slice is fresh: attrs comes off the context and is shared, so appending in
// place could write into a sibling call's array.
func (w *wrappedClientExecutor) callLabels(attrs []attribute.KeyValue, status, outcome string) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs)+3)
	out = append(out, attrs...)
	out = append(out, attribute.String("service", w.service))
	if outcome != "" {
		out = append(out, attribute.String("resilience.outcome", outcome))
	}
	if status == "" {
		return out
	}
	return append(out, attribute.String("status", status))
}

// attrsToLogFields renders the operation's attributes as log fields, so the log
// keys equal the metric labels' names and a dashboard's selector joins the two.
func attrsToLogFields(attrs []attribute.KeyValue) []log.Field {
	fields := make([]log.Field, 0, len(attrs))
	for _, a := range attrs {
		key := string(a.Key)
		switch a.Value.Type() {
		case attribute.BOOL:
			fields = append(fields, log.Bool(key, a.Value.AsBool()))
		case attribute.INT64:
			fields = append(fields, log.Any(key, a.Value.AsInt64()))
		case attribute.FLOAT64:
			fields = append(fields, log.Float(key, a.Value.AsFloat64()))
		default:
			fields = append(fields, log.String(key, a.Value.AsString()))
		}
	}
	return fields
}

// durMs renders a duration in milliseconds for the access log, the unit the log
// field name promises.
func durMs(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func (w *wrappedClientExecutor) Close() error { return w.inner.Close() }

// Refresh forwards p to the inner executor, keeping the wrapper's
// metrics/listener intact. The inner driver always implements Refresh.
func (w *wrappedClientExecutor) Refresh(p ClientPolicy) error {
	return w.inner.Refresh(p)
}

// WrapServerExecutor returns a [ServerExecutor] that wraps inner with an
// internal span, the inbound admission metrics, and an access log per request —
// the server-side counterpart of [WrapClientExecutor], applied by [Manager.ServerExecutorFor]
// at the same point and for the same reason (the executor is still private when it
// is wrapped, so the breaker-listener handshake below can reach it).
//
// The inbound instruments are a separate family (resilience.server.*); see
// [instrumentSet] for why. The access log reuses the resilience tag, with
// system/service naming the route.
func WrapServerExecutor(inner ServerExecutor, system, service string) ServerExecutor {
	if inner == nil {
		return nil
	}
	w := &wrappedServerExecutor{
		inner:   inner,
		system:  system,
		service: service,
		ins:     instruments(),
	}
	// Construction-time handshake, exactly as in WrapClientExecutor: the breaker does
	// not exist yet, so the listener is installed on the still-private executor
	// and is in place for every transition of its life.
	if setter, ok := inner.(BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedServerExecutor struct {
	inner   ServerExecutor
	system  string
	service string

	// ins is the shared instrument set; the tracer is looked up per use, for the
	// reason given on [wrappedClientExecutor].
	ins *instrumentSet
}

// OnBreakerStateChange satisfies [BreakerEventListener] for the inbound breaker. A
// trip (→open) is a route-level degradation worth flagging at Warn; recovery and
// half-open trial are Info — the same levelling the outbound listener uses.
func (w *wrappedServerExecutor) OnBreakerStateChange(service string, from, to BreakerState) {
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
	if to == BreakerOpen {
		log.Warn(context.Background(), resilienceTag, fields...)
	} else {
		log.Info(context.Background(), resilienceTag, fields...)
	}
}

// Execute wraps the inner call in an internal span, records the duration histogram
// and the status-classified request counter, and writes the access log: a
// rejection or error at Warn, a success at Debug.
func (w *wrappedServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	start := time.Now()
	op, hasOp := observability.OperationFrom(ctx)
	// The half of an inbound operation that does not exist yet: the handler records
	// what only it knows once it has answered — the response status code — and the
	// emitter reads it back below. A client declares its whole operation up front;
	// a server cannot, and this is the holder that closes that gap.
	ctx, resp := observability.WithResponse(ctx)

	ctx, span := otel.Tracer(scope).Start(ctx, w.serverSpanName(op, hasOp),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(w.serverSpanAttrs(op, hasOp)...))

	var ins *opServerInstrumentSet
	if hasOp {
		ins = serverOperationInstruments(op.Metric)
		ins.active.Add(ctx, 1, metric.WithAttributes(w.serverLabels(op.Attrs, "", nil)...))
	}

	err := w.inner.Execute(ctx, fn)
	status := classifyStatus(err)
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

	// resilience.server.* is the admission layer's own signal and keeps its shape
	// whether or not the route declared an operation.
	labels := []attribute.KeyValue{
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("status", status),
	}
	if outcome := classifyOutcome(err); outcome != "" {
		labels = append(labels, attribute.String("resilience.outcome", outcome))
	}
	w.ins.serverDuration.Record(ctx, dur.Seconds(), metric.WithAttributes(labels...))
	w.ins.serverCalls.Add(ctx, 1, metric.WithAttributes(labels...))

	if ins != nil {
		// The declared family metrics: the route's own bounded vocabulary, the
		// response half, and the shared status axis. The family names are the HTTP
		// family's — `<prefix>.request.duration` / `<prefix>.active_requests`, not
		// the client side's `.operation.duration` — because a server's signal is a
		// request, and those are the names every dashboard already reads.
		opLabels := w.serverLabels(op.Attrs, status, respAttrs)
		ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(opLabels...))
		ins.active.Add(ctx, -1, metric.WithAttributes(w.serverLabels(op.Attrs, "", nil)...))
	}

	tag := resilienceTag
	fields := []log.Field{
		log.String("system", w.system),
		log.String("service", w.service),
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
		log.Warn(ctx, tag, append(fields, log.Err(err))...)
		return err
	}
	if successQuiet(hasOp, op) {
		log.Debug(ctx, tag, func() []log.Field { return fields })
		return nil
	}
	log.Info(ctx, tag, fields...)
	return nil
}

// serverSpanName names an inbound span: the route's own word when it declared
// one, the admission service otherwise — which is what this layer's spans have
// always been named in the undeclared case.
func (w *wrappedServerExecutor) serverSpanName(op observability.Operation, hasOp bool) string {
	if hasOp && op.Name != "" {
		return op.Name
	}
	return w.service
}

// serverSpanAttrs is the route's attributes and detail when it declared an
// operation, and this layer's own admission labels otherwise.
func (w *wrappedServerExecutor) serverSpanAttrs(op observability.Operation, hasOp bool) []attribute.KeyValue {
	if hasOp {
		out := make([]attribute.KeyValue, 0, len(op.Attrs)+len(op.Detail))
		out = append(out, op.Attrs...)
		return append(out, op.Detail...)
	}
	return []attribute.KeyValue{
		attribute.String("resilience.system", w.system),
		attribute.String("resilience.service", w.service),
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
func (w *wrappedServerExecutor) serverLabels(attrs []attribute.KeyValue, status string, resp []attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs)+len(resp)+1)
	out = append(out, attrs...)
	out = append(out, resp...)
	if status == "" {
		return out
	}
	return append(out, attribute.String("status", status))
}

func (w *wrappedServerExecutor) Close() error { return w.inner.Close() }

// Refresh forwards a to the inner executor, keeping the wrapper's
// metrics/listener intact.
func (w *wrappedServerExecutor) Refresh(p ServerPolicy) error {
	return w.inner.Refresh(p)
}

// classifyStatus maps a ClientExecutor's return error onto the status axis
// go-spring uses everywhere else — `ok` / `error`, the two words a cross-family
// query joins on. A protected call is not a special case: it reports the same
// two values as an inbound request or a discovery lookup.
//
// What protection decided is a different question, and it has its own key (see
// [classifyOutcome]). Keeping them apart is the same split as `status` beside
// `rpc.grpc.status_code`: the shared axis carries the outcome, the
// family-specific key carries the detail.
func classifyStatus(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

// classifyOutcome names what the resilience stages did to a call, when they did
// anything: they rejected it (rate limit, open circuit, bulkhead) or it ran out
// of time. It is the resilience-side detail of `status`'s shared axis, so the
// protection's own vocabulary stays out of the key every family shares.
//
// It is empty when nothing out of the ordinary happened — a success, or a plain
// downstream error — and an empty outcome is dropped rather than labelled, so
// the label is absent instead of meaningless for the common case.
func classifyOutcome(err error) string {
	switch {
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, ErrRetryBudgetExceeded):
		return "retry_budget_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return ""
	}
}
