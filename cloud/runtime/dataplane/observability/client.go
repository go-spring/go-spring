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

	"sync"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/cloud/runtime/dataplane/breaker"
	"go-spring.org/cloud/runtime/dataplane/chain"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// clientInstrumentSet is the outbound layer's always-on signals: the per-call
// duration and status counter, and the circuit-breaker transition counter. One
// set per process, resolved lazily on first use so it binds to whichever
// providers are current then, and immutable afterwards. It holds no per-client
// state — the system and service labels travel with each wrapper, not here.
//
// It is a separate set from the inbound (resilience.server.*) family rather than
// one family with a direction attribute: the outcome sets genuinely differ
// (inbound has no retry, so no retry_budget_exceeded and no per-attempt timeout),
// and keeping them apart leaves existing outbound dashboards untouched.
type clientInstrumentSet struct {
	duration       metric.Float64Histogram
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
}

// clientInstruments is the one outbound instrument set this package uses for the
// whole process.
var clientInstruments = sync.OnceValue(buildClientInstruments)

func buildClientInstruments() *clientInstrumentSet {
	m := otel.Meter(executorScope)
	in := &clientInstrumentSet{}
	in.duration, _ = m.Float64Histogram("resilience.client.duration",
		metric.WithDescription("Duration of resilience-protected calls"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	// calls counts protected calls on the shared status axis (ok/error) plus,
	// where protection intervened, the resilience.error_type that says what it did
	// (rate_limited, circuit_open, bulkhead_full, retry_budget_exceeded, timeout).
	in.calls, _ = m.Int64Counter("resilience.client.calls",
		metric.WithDescription("Number of resilience-protected calls by status"),
		metric.WithUnit("{call}"))
	// breakerChanges counts circuit-breaker state transitions (from/to attrs).
	in.breakerChanges, _ = m.Int64Counter("resilience.client.breaker.state_change",
		metric.WithDescription("Circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	return in
}

// WrapClientExecutor returns an [chain.Executor] that wraps inner with an internal span
// (system/service/status attributes), the resilience metrics, and an
// access log per call. Pass the system label (e.g. "redis", "gorm", "grpc")
// so calls from several protected clients are distinguishable. A nil inner
// returns nil — no wrapper, so an unarmed client stays untouched.
//
// This is an internal composition step, not a client API: [ClientExecutorFor] applies
// it while the provider-built executor is still private (see [resolve]), so
// clients get an already-observed executor and never wrap one themselves. The
// only other caller is a client that composes its own stack around a raw
// executor it built directly (e.g. httpx's custom chain.Executor / ResilienceDriver
// paths).
//
// Attaching the breaker listener happens here, while inner is still private:
// the type assertion below is a construction-time handshake, so the executor
// never escapes without its listener and no late binding is needed. The shared
// instruments are resolved at this point — the first resolve time, not package
// init — so an SDK installed after this package's init still receives the spans
// and records. The tracer is not held at all; see [wrappedClientExecutor.Execute].
func WrapClientExecutor(inner chain.Executor, system, service string) chain.Executor {
	if inner == nil {
		return nil
	}
	w := &wrappedClientExecutor{
		inner: inner,
		callEmitter: &callEmitter{
			system:  system,
			service: service,
			ins:     clientInstruments(),
		},
	}
	// If the inner executor emits breaker state transitions, subscribe so each
	// trip / half-open / recovery emits a counter + log automatically. Drivers
	// without that capability (no BreakerEventListenerSetter) are silently
	// skipped — the call-level signals still emit.
	if setter, ok := inner.(breaker.EventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedClientExecutor struct {
	inner chain.Executor
	*callEmitter
}

// callEmitter produces the standard client-call signals for ONE client
// system/service: the call span, the call-level duration, the in-flight gauge,
// the always-on calls counter and one access-log line. It is shared by
// [wrappedClientExecutor] — which runs a governance executor around the call —
// and [OperationObserver], the self-emission path for a client whose transport
// offers no seam for that executor.
type callEmitter struct {
	system  string
	service string

	// ins is the shared outbound instrument set. The tracer is deliberately NOT
	// held alongside it: a captured otel.Tracer stops forwarding once the global
	// provider is set again, so it is looked up per use.
	ins *clientInstrumentSet
}

// OnBreakerStateChange satisfies [BreakerEventListener]. It is invoked
// synchronously from inside the breaker's transition (so it must not call back
// into the executor); it emits a state-change counter and a log line.
func (w *wrappedClientExecutor) OnBreakerStateChange(service string, from, to breaker.State) {
	w.ins.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
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
	if to == breaker.Open {
		log.Warn(context.Background(), resilienceTag, "resilience: circuit breaker opened", fields...)
	} else {
		log.Info(context.Background(), resilienceTag, "resilience: circuit breaker state changed", fields...)
	}
}

// opInstrumentSet holds the instruments one metric prefix needs. The prefix
// comes from the operation a client declared (see [chain.Operation]), so
// this set cannot be built once per process the way [instrumentSet] is — a
// process serves any number of client families, each with its own prefix. It is
// memoized per prefix on first use, which is also what binds it to whichever
// meter provider is current then.
type opInstrumentSet struct {
	duration metric.Float64Histogram
	attempts metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// opInstruments memoizes one [opInstrumentSet] per metric prefix. It holds no
// per-call state — the system/service/operation labels travel with each record.
var opInstruments = struct {
	mu sync.Mutex
	m  map[string]*opInstrumentSet
}{m: map[string]*opInstrumentSet{}}

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
	m := otel.Meter(componentName)
	s := &opInstrumentSet{}
	var err error
	s.duration, err = m.Float64Histogram(prefix+".operation.duration",
		metric.WithDescription("Duration of one logical client operation, retries and backoff included"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	reportInstrumentError(prefix, err)
	s.attempts, err = m.Float64Histogram(prefix+".attempt.duration",
		metric.WithDescription("Duration of one downstream attempt inside a client operation"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	reportInstrumentError(prefix, err)
	s.active, err = m.Int64UpDownCounter(prefix+".active_requests",
		metric.WithDescription("Number of in-flight client operations"),
		metric.WithUnit("{request}"))
	reportInstrumentError(prefix, err)
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
// A call that declared no operation (a zero [chain.Operation]) takes the
// fallback path below, which reproduces this layer's historical signals verbatim,
// down to their names — migrating a client to the declared-operation path is what
// switches its signals over, one client at a time.
func (w *wrappedClientExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	// The record belongs to the caller when it passed one. A caller that passed
	// nil declined to keep it, but this layer still MEASURES the call and emits
	// from the record, so it supplies its own and threads THAT one down — the
	// rounds the fn's peer write-back and the fault gate's fire land on are
	// recorded either way.
	if rec == nil {
		rec = &chain.Recorder{}
	}
	return w.call(ctx, op, rec, func(c context.Context) error { return w.inner.Execute(c, op, rec, fn) }, false)
}

// call runs one observed call and emits its signals. invoke is the call itself:
// for [wrappedClientExecutor] it enters the governance executor, for
// [OperationObserver] it is the client's own work. selfRound records one
// attempt-level sample equal to the call, for a caller that has no separate
// attempt producer.
func (e *callEmitter) call(ctx context.Context, op chain.Operation, rec *chain.Recorder, invoke func(context.Context) error, selfRound bool) error {
	start := time.Now()
	// A described call carries a non-empty metric prefix; a caller that declares
	// no description (the zero Operation) takes the fallback path in emit.
	hasOp := op.Described()

	var ins *opInstrumentSet
	if hasOp {
		ins = operationInstruments(op.Observe.Metric.Name)
	}

	ctx, span := otel.Tracer(componentName).Start(ctx, e.spanName(op, hasOp),
		trace.WithSpanKind(op.SpanKind()),
		trace.WithLinks(op.Observe.Span.Links...),
		trace.WithAttributes(e.spanAttrs(op, hasOp)...))
	// The in-flight gauge brackets the whole call, retries included, exactly as
	// the span does.
	if ins != nil {
		ins.active.Add(ctx, 1, metric.WithAttributes(e.callLabels(e.target(op), op.Observe.Metric.Attrs, "", "")...))
	}

	err := invoke(ctx)

	if ins != nil {
		ins.active.Add(ctx, -1, metric.WithAttributes(e.callLabels(e.target(op), op.Observe.Metric.Attrs, "", "")...))
	}
	status := chain.ClassifyStatus(err)
	// The record is written where the round happened: the governance retry loop
	// records every round it ran, and the SELF-round below covers the path with no
	// governance executor at all, whose one try never passes a loop. The emitter's
	// own signals (metric labels, span attributes, log fields) come from its local
	// status/err and never read the record.
	if selfRound {
		// No governance executor ran, so no retry loop recorded a round: the one
		// downstream try IS the call. Recording it keeps the round-level series
		// populated rather than silently absent.
		rec.AddRound(chain.Round{
			Start:     start,
			Duration:  time.Since(start),
			Status:    status,
			ErrorType: chain.ClassifyErrorType(err),
			Err:       err,
		})
	}
	// The span attributes the call PRODUCED — the caller's declared block was set
	// when the span started, and these could not be: only the call itself knows
	// what its fn found or what the client's own executor measured. Appended after
	// the declared ones, so a key on both sides reads produced-last.
	if len(rec.Observed.Span) > 0 {
		span.SetAttributes(rec.Observed.Span...)
	}
	span.SetAttributes(attribute.String("status", status))
	// The bound the call was GIVEN, so the span's own duration can be read against
	// it. It is the wall-time budget the governance layer resolved — after any
	// hot-fix override — and it is the one fact a timeout failure needs that the
	// duration alone cannot supply: "it hit its 500ms" and "the downstream was
	// slow" look identical without it. Zero means the call ran under no bound of
	// its own, so it is omitted rather than emitted as a misleading 0.
	if rec.Timeout > 0 {
		span.SetAttributes(attribute.Float64("timeout_ms", durMs(rec.Timeout)))
	}
	// An injected fault is tagged so a fire is never read as a real downstream
	// failure; the kind is bounded, so it is safe on the span. The rule it came
	// from rides along, so "which rule set this call on fire" is answerable from
	// the span itself.
	if f := rec.Fault; f != nil {
		span.SetAttributes(attribute.String("resilience.injected", f.Kind))
		span.SetAttributes(attribute.String("resilience.injected.rule", f.Rule))
	}
	if err != nil {
		// Both marks, so a span carries the same evidence whichever emitter
		// produced it: SetStatus is the span's own outcome, RecordError is the
		// exception event a trace UI lists. It records the CALL's error, never an
		// attempt's, so a retried-then-succeeded call still looks clean.
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	e.emit(ctx, start, status, err, op, hasOp, rec)
	return err
}

// OperationObserver is the self-emission entry for a client whose transport
// offers no seam for the resilience executor — a poll-driven subscriber, a
// message producer whose hook is call-site-only — but which still wants the
// standard client signals. It emits exactly what [WrapClientExecutor] emits (the
// same span, metric names, labels and access log), and applies NO governance:
// there is no rate limit, circuit breaker, retry or timeout in this path.
//
// A client that has a seam for the executor declares its operation and routes
// through it instead; this exists so a client that cannot is not left
// unobserved.
type OperationObserver struct {
	*callEmitter
}

// NewOperationObserver builds the observer for one client system/service. system
// is the backend name ("kafka", "webhook"); service is the per-instance label
// (see [label.ServiceLabel]).
func NewOperationObserver(system, service string) *OperationObserver {
	return &OperationObserver{callEmitter: &callEmitter{
		system:  system,
		service: service,
		ins:     clientInstruments(),
	}}
}

// Observe runs fn as one observed call, emitting the call span, the
// call-level duration, the in-flight gauge, the attempt-level duration and the
// access log. op is the operation's description (see [chain.Operation]); fn's
// error is returned unchanged — this is instrumentation, not error policy.
//
// rec is the same explicit record [chain.Executor.Execute] takes, for the same
// reason: whoever owns the call owns its record. A caller that wants to read back
// what the call produced — the rounds, the signals its own fn appended, the
// fire the fault layer set — passes its own; a caller that only wants the
// signals passes nil, and this layer measures the call with a record of its own.
func (o *OperationObserver) Observe(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	if rec == nil {
		rec = &chain.Recorder{}
	}
	return o.call(ctx, op, rec, fn, true)
}

// emit produces the call-level signals — span (already ended by the caller),
// counter, call histogram, attempt histograms and the one access log. It runs
// after the inner call, so the recorder is complete.
func (e *callEmitter) emit(ctx context.Context, start time.Time, status string, err error, op chain.Operation, hasOp bool, rec *chain.Recorder) {
	dur := time.Since(start)
	errorType := chain.ClassifyErrorType(err)

	// The status-classified call counter is the one signal both paths carry: it
	// answers "was this rejected, or did the downstream fail", which nothing else
	// distinguishes — `status` says it failed, `resilience.error_type` says
	// protection rejected it.
	calls := []attribute.KeyValue{
		attribute.String("system", e.system),
		attribute.String("service", e.service),
		attribute.String("status", status),
	}
	if errorType != "" {
		calls = append(calls, attribute.String("resilience.error_type", errorType))
	}
	e.ins.calls.Add(ctx, 1, metric.WithAttributes(calls...))

	tag := resilienceTag
	fields := []log.Field{
		log.String("system", e.system),
		log.String("service", e.target(op)),
	}
	quietSuccess := successQuiet(hasOp, op)
	if hasOp {
		ins := operationInstruments(op.Observe.Metric.Name)
		// Metric labels come from Attrs alone: Detail is deliberately excluded, so
		// a cache key or message subject can never become a label.
		//
		// The call's PRODUCED labels ride on the call-level duration only — an
		// attempt-level sample describes one round, and a call-level supplement does
		// not belong on it. Declared attrs first, produced after, the same order the
		// span and the log use; the merged slice is fresh, because appending in place
		// would write into the shared Operation's array.
		metricAttrs := op.Observe.Metric.Attrs
		if len(rec.Observed.Metric) > 0 {
			metricAttrs = append(append([]attribute.KeyValue{}, metricAttrs...), rec.Observed.Metric...)
		}
		ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(e.callLabels(e.target(op), metricAttrs, status, errorType)...))
		// The attempt layer: one sample per round the chain ran, including a round a
		// gate refused before the downstream was touched (zero duration, no peer,
		// its outcome naming the gate) — so the refused volume is visible here
		// rather than silently missing.
		for _, a := range rec.Rounds() {
			ins.attempts.Record(ctx, a.Duration.Seconds(),
				metric.WithAttributes(e.callLabels(e.target(op), op.Observe.Metric.Attrs, a.Status, chain.ClassifyErrorType(a.Err))...))
		}
		if op.Observe.Log.Tag != nil {
			tag = op.Observe.Log.Tag
		}
		// The log carries the operation's own vocabulary — attrs, detail and the
		// protocol's free-form fields alike — so a dashboard selecting failed
		// operations lands on the lines that explain them.
		fields = append(fields, op.Observe.Log.Fields...)
		fields = append(fields, rec.Observed.Log...)
		fields = append(fields, log.String("status", status), log.Float("duration_ms", durMs(dur)))
		// The timeout sits next to the duration on purpose: "given 500ms, took
		// 498ms" is the line an operator reads to decide whether the bound is too
		// tight or the downstream too slow. Omitted when the call had no bound.
		if rec.Timeout > 0 {
			fields = append(fields, log.Float("timeout_ms", durMs(rec.Timeout)))
		}
	} else {
		// Fallback: this layer's historical signals, names and labels unchanged —
		// plus the call's produced block, which is not a property of the declared
		// path but of the call: a client that never declared an operation can still
		// report what it found.
		labels := []attribute.KeyValue{
			attribute.String("system", e.system),
			attribute.String("service", e.service),
		}
		labels = append(labels, rec.Observed.Metric...)
		labels = append(labels, attribute.String("status", status))
		if errorType != "" {
			labels = append(labels, attribute.String("resilience.error_type", errorType))
		}
		e.ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(labels...))
		fields = append(fields, rec.Observed.Log...)
		fields = append(fields, log.Float("duration_ms", durMs(dur)), log.String("status", status))
	}
	if errorType != "" {
		fields = append(fields, log.String("resilience.error_type", errorType))
	}
	// Tag an injected fault's access line too, so the log joins the span in
	// telling an operator this failure was set on fire on purpose — and by
	// which rule.
	if f := rec.Fault; f != nil {
		fields = append(fields, log.String("resilience.injected", f.Kind),
			log.String("resilience.injected.rule", f.Rule))
	}

	if err != nil {
		log.Warn(ctx, tag, "client call failed", append(fields, log.Err(err))...)
		return
	}
	if quietSuccess {
		// Built lazily: the success record is there for troubleshooting, not
		// everyday reading.
		log.Debug(ctx, tag, "client call succeeded", func() []log.Field { return fields })
		return
	}
	log.Info(ctx, tag, "client call finished", fields...)
}

// spanName names the call's span: the operation's own word when the client
// declared one, the service label otherwise — matching what this layer's spans
// have always been named in the undeclared case.
func (e *callEmitter) spanName(op chain.Operation, hasOp bool) string {
	if hasOp && op.Request.Action != "" {
		return op.Request.Action
	}
	return e.service
}

// spanAttrs is what the span carries: the operation's declared
// [chain.Observe.Span.Attrs] when it declared a description, else this layer's
// own system/service labels.
func (e *callEmitter) spanAttrs(op chain.Operation, hasOp bool) []attribute.KeyValue {
	if hasOp {
		// The declared attrs are the protocol's vocabulary — db.system,
		// db.operation, url.full — and NONE of them says which service was
		// called, so a described span would not read as a call edge at all
		// without the target added beside them. The metric label and the log
		// field carry the same value under this layer's shared name `service`;
		// the span carries it under the conventional trace name, which is what a
		// service graph and a trace UI join on.
		//
		// The slice is fresh: the declared attrs belong to the (shared,
		// reusable) Operation, so appending in place could write into a sibling
		// call's array.
		return append([]attribute.KeyValue{attribute.String("peer.service", e.target(op))}, op.Observe.Span.Attrs...)
	}
	return []attribute.KeyValue{
		attribute.String("resilience.system", e.system),
		attribute.String("resilience.service", e.service),
	}
}

// target resolves the thing the call went TO: the operation's own target when it
// declared one, else the label this emitter was built with (the label the
// executor was built for, which is what an undeclared call resolves to). One
// resolver, so the metric label, the log field and the span attribute can never
// disagree about where the call went.
func (e *callEmitter) target(op chain.Operation) string {
	if op.Service != "" {
		return op.Service
	}
	return e.service
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
func (e *callEmitter) callLabels(service string, attrs []attribute.KeyValue, status, errorType string) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs)+3)
	out = append(out, attrs...)
	out = append(out, attribute.String("service", service))
	if errorType != "" {
		out = append(out, attribute.String("resilience.error_type", errorType))
	}
	if status == "" {
		return out
	}
	return append(out, attribute.String("status", status))
}

func (w *wrappedClientExecutor) Close() error { return w.inner.Close() }
