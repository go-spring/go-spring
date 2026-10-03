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
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

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
		inner:   inner,
		system:  system,
		service: service,
		ins:     executorInstruments(),
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
	inner   chain.Executor
	system  string
	service string

	// ins is the shared instrument set. The tracer is deliberately NOT held
	// alongside it: a captured otel.Tracer stops forwarding once the global
	// provider is set again, so it is looked up per use.
	ins *executorInstrumentSet
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
// comes from the operation a client declared (see [Operation]), so
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
	s.duration, _ = m.Float64Histogram(prefix+".operation.duration",
		metric.WithDescription("Duration of one logical client operation, retries and backoff included"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(DurationBuckets()...))
	s.attempts, _ = m.Float64Histogram(prefix+".attempt.duration",
		metric.WithDescription("Duration of one downstream attempt inside a client operation"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(DurationBuckets()...))
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
// A call that declared no operation (see [WithOperation]) takes the
// fallback path below, which reproduces this layer's historical signals verbatim,
// down to their names — migrating a client to the declared-operation path is what
// switches its signals over, one client at a time.
func (w *wrappedClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	start := time.Now()
	ctx, rec := WithRecorder(ctx)
	op, hasOp := OperationFrom(ctx)

	var ins *opInstrumentSet
	if hasOp {
		ins = operationInstruments(op.Metric)
	}

	ctx, span := otel.Tracer(componentName).Start(ctx, w.spanName(op, hasOp),
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
	status := ClassifyStatus(err)
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
func (w *wrappedClientExecutor) emit(ctx context.Context, start time.Time, status string, err error, op Operation, hasOp bool, rec *Recorder) {
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

// spanName names the call's span: the operation's own word when the client
// declared one, the service label otherwise — matching what this layer's spans
// have always been named in the undeclared case.
func (w *wrappedClientExecutor) spanName(op Operation, hasOp bool) string {
	if hasOp && op.Name != "" {
		return op.Name
	}
	return w.service
}

// spanAttrs is the operation's attributes plus its detail when declared, and
// this layer's own system/service labels otherwise. The span carries both: the
// unbounded detail that must stay out of metric labels is exactly what makes a
// span useful to read.
func (w *wrappedClientExecutor) spanAttrs(op Operation, hasOp bool) []attribute.KeyValue {
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

func (w *wrappedClientExecutor) Close() error { return w.inner.Close() }
