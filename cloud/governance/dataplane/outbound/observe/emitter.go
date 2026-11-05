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

package observe

import (
	"context"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// callEmitter produces the standard client-call signals for ONE client
// system/service: the call span, the call-level duration, the in-flight gauge,
// the always-on calls counter and one access-log line. It is shared by
// [observingExecutor] — which runs a governance executor around the call —
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

// successQuiet reports whether a successful call's access line is logged at
// Debug (true) or Info (false).
//
// An UNDECLARED call is this layer's historical quiet case: it has always logged
// success at Debug, and a client that declares nothing (redigo's skipped PING,
// say) declares nothing precisely to keep the noise down — so it stays quiet.
// A declared call says so itself, via [chain.Observe.Log.QuietSuccess]: a producer
// whose calls carry high-frequency per-call detail sets it, one whose success is
// worth a line does not.
func successQuiet(hasOp bool, op chain.Operation) bool {
	if !hasOp {
		return true
	}
	return op.Observe.Log.QuietSuccess
}

// call runs one observed call and emits its signals. invoke is the call itself:
// for [observingExecutor] it enters the governance executor, for
// [OperationObserver] it is the client's own work. selfRound records one
// attempt-level sample equal to the call, for a caller that has no separate
// attempt producer.
func (e *callEmitter) call(ctx context.Context, op chain.Operation, rec *chain.Recorder, invoke func(context.Context) error, selfRound bool) error {
	start := time.Now()
	// A described call carries a non-empty metric prefix; a caller that declares
	// no description (the zero Operation) takes the fallback path in emit.
	hasOp := op.Observe.Metric.Name != ""

	var ins *opInstrumentSet
	if hasOp {
		ins = operationInstruments(op.Observe.Metric.Name)
	}

	// An unspecified kind is the CLIENT edge: the governed calls this emitter
	// describes reach a database, a broker or another service, and a backend
	// that pairs client and server edges (the service graph of any trace UI)
	// can only do so if the outbound edge says it is one. A genuinely
	// in-process call states [trace.SpanKindInternal] itself.
	kind := op.Observe.Span.Kind
	if kind == trace.SpanKindUnspecified {
		kind = trace.SpanKindClient
	}
	ctx, span := otel.Tracer(instrument.ComponentName).Start(ctx, e.spanName(op, hasOp),
		trace.WithSpanKind(kind),
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
			Error:     err,
		})
	}
	// The span attributes the call PRODUCED — the caller's declared block was set
	// when the span started, and these could not be: only the call itself knows
	// what its fn found or what the client's own executor measured. Appended after
	// the declared ones, so a key on both sides reads produced-last.
	if len(rec.Observed.SpanAttrs) > 0 {
		span.SetAttributes(rec.Observed.SpanAttrs...)
	}
	span.SetAttributes(attribute.String("status", status))
	// The bound the call was GIVEN, so the span's own duration can be read against
	// it. It is the wall-time budget the governance layer resolved — after any
	// hot-fix override — and it is the one fact a timeout failure needs that the
	// duration alone cannot supply: "it hit its 500ms" and "the downstream was
	// slow" look identical without it. Zero means the call ran under no bound of
	// its own, so it is omitted rather than emitted as a misleading 0.
	if rec.Timeout > 0 {
		span.SetAttributes(attribute.Float64("timeout_ms", instrument.DurMs(rec.Timeout)))
	}
	// An injected fault is tagged so a fire is never read as a real downstream
	// failure; the kind is bounded, so it is safe on the span. The rule it came
	// from rides along, so "which rule set this call on fire" is answerable from
	// the span itself.
	if f := rec.Fault; f != nil {
		attrs := []attribute.KeyValue{
			attribute.String("resilience.injected", f.Error),
			attribute.String("resilience.injected.rule", f.Name),
			attribute.Float64("resilience.injected.delay_ms", float64(f.Delay)/float64(time.Millisecond)),
		}
		if f.RunID != "" {
			attrs = append(attrs, attribute.String("resilience.injected.run", f.RunID))
		}
		span.SetAttributes(attrs...)
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

	// The fault result, per rule: the experiment's outcome at call granularity.
	// Only the rule that actually fired is counted, and only the effect it applied.
	if rec != nil {
		if f := rec.Fault; f != nil && f.Fired() {
			base := []attribute.KeyValue{
				attribute.String("system", e.system),
				attribute.String("service", e.target(op)),
				attribute.String("rule", f.Name),
			}
			if f.Inject {
				labels := append(append([]attribute.KeyValue{}, base...),
					attribute.String("kind", f.Error),
					attribute.String("class", f.Class))
				e.ins.faultInjected.Add(ctx, 1, metric.WithAttributes(labels...))
			}
			if f.Delay > 0 {
				e.ins.faultDelayed.Add(ctx, 1, metric.WithAttributes(base...))
			}
		}
	}

	tag := instrument.Tag
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
		if len(rec.Observed.MetricAttrs) > 0 {
			metricAttrs = append(append([]attribute.KeyValue{}, metricAttrs...), rec.Observed.MetricAttrs...)
		}
		ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(e.callLabels(e.target(op), metricAttrs, status, errorType)...))
		// The attempt layer: one sample per round the chain ran, including a round a
		// gate refused before the downstream was touched (zero duration, no peer,
		// its outcome naming the gate) — so the refused volume is visible here
		// rather than silently missing.
		for _, a := range rec.Rounds {
			ins.attempts.Record(ctx, a.Duration.Seconds(),
				metric.WithAttributes(e.callLabels(e.target(op), op.Observe.Metric.Attrs, a.Status, chain.ClassifyErrorType(a.Error))...))
		}
		if op.Observe.Log.Tag != nil {
			tag = op.Observe.Log.Tag
		}
		// The log carries the operation's own vocabulary — attrs, detail and the
		// protocol's free-form fields alike — so a dashboard selecting failed
		// operations lands on the lines that explain them.
		fields = append(fields, op.Observe.Log.Fields...)
		fields = append(fields, rec.Observed.LogFields...)
		fields = append(fields, log.String("status", status), log.Float("duration_ms", instrument.DurMs(dur)))
		// The timeout sits next to the duration on purpose: "given 500ms, took
		// 498ms" is the line an operator reads to decide whether the bound is too
		// tight or the downstream too slow. Omitted when the call had no bound.
		if rec.Timeout > 0 {
			fields = append(fields, log.Float("timeout_ms", instrument.DurMs(rec.Timeout)))
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
		labels = append(labels, rec.Observed.MetricAttrs...)
		labels = append(labels, attribute.String("status", status))
		if errorType != "" {
			labels = append(labels, attribute.String("resilience.error_type", errorType))
		}
		e.ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(labels...))
		fields = append(fields, rec.Observed.LogFields...)
		fields = append(fields, log.Float("duration_ms", instrument.DurMs(dur)), log.String("status", status))
	}
	if errorType != "" {
		fields = append(fields, log.String("resilience.error_type", errorType))
	}
	// Tag an injected fault's access line too, so the log joins the span in
	// telling an operator this failure was set on fire on purpose — and by
	// which rule.
	if f := rec.Fault; f != nil {
		fields = append(fields,
			log.String("resilience.injected", f.Error),
			log.String("resilience.injected.rule", f.Name),
			log.Float("resilience.injected.delay_ms", float64(f.Delay)/float64(time.Millisecond)))
		if f.RunID != "" {
			fields = append(fields, log.String("resilience.injected.run", f.RunID))
		}
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
