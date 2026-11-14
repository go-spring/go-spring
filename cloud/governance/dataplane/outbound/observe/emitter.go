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
	"strings"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

	// metrics is the metric seam; nil disables the metrics. The tracer is
	// deliberately NOT held: a captured otel.Tracer stops forwarding once the
	// global provider is set again, so it is looked up per use.
	metrics Metrics
}

// successQuiet reports whether a successful call's access line is logged at
// Debug (true) or Info (false). Every call declares an operation, so the level
// is the producer's own decision: a high-frequency call (a cache GET, a health
// PING) sets [chain.Observe.Log.QuietSuccess], one whose success is worth a line
// does not.
func successQuiet(op chain.Operation) bool {
	return op.Observe.Log.QuietSuccess
}

// call runs one observed call and emits its signals. invoke is the call itself:
// for [observingExecutor] it enters the governance executor, for
// [OperationObserver] it is the client's own work. selfRound records one
// attempt-level sample equal to the call, for a caller that has no separate
// attempt producer.
func (e *callEmitter) call(ctx context.Context, op chain.Operation, rec *chain.Recorder, invoke func(context.Context) error, selfRound bool) error {
	start := time.Now()

	// An unspecified kind is the CLIENT edge: the governed calls this emitter
	// describes reach a database, a broker or another service, and a backend
	// that pairs client and server edges (the service graph of any trace UI)
	// can only do so if the outbound edge says it is one. A genuinely
	// in-process call states [trace.SpanKindInternal] itself.
	kind := op.Observe.Span.Kind
	if kind == trace.SpanKindUnspecified {
		kind = trace.SpanKindClient
	}
	ctx, span := otel.Tracer(instrument.ComponentName).Start(ctx, e.spanName(op),
		trace.WithSpanKind(kind),
		trace.WithLinks(op.Observe.Span.Links...),
		trace.WithAttributes(e.spanAttrs(op)...))
	// The in-flight gauge brackets the whole call, retries included, exactly as
	// the span does.
	if e.metrics != nil {
		e.metrics.Inflight(ctx, e.callOf(op, op.Observe.Metric.Attrs), 1)
	}

	// The SELF-round covers the path with no governance executor at all, whose
	// one try never passes a loop: the round is CREATED before the try — so what
	// the fn writes (its produced signals) lands on it — and SEALED after, the
	// same open/seal pair the retry loop runs per attempt. The emitter's own
	// signals (metric labels, span attributes, log fields) come from its local
	// status/err and never read the record.
	var rd *chain.Round
	if selfRound {
		rd = rec.NewRound()
		rd.Start = start
	}

	err := invoke(ctx)

	if e.metrics != nil {
		e.metrics.Inflight(ctx, e.callOf(op, op.Observe.Metric.Attrs), -1)
	}
	status := chain.ClassifyStatus(err)
	if rd != nil {
		rd.Duration = time.Since(start)
		rd.SetError(err)
	}
	// The span attributes the call PRODUCED — the caller's declared block was set
	// when the span started, and these could not be: only the call itself knows
	// what its fn found or what the client's own executor measured. The rounds'
	// blocks are appended in round order, after the declared ones, so a key on
	// both sides reads produced-last and a later round's value reads last of all.
	for i := range rec.Rounds {
		span.SetAttributes(rec.Rounds[i].Observed.SpanAttrs...)
	}
	span.SetAttributes(attribute.String("status", status))
	// The bound the call was GIVEN, so the span's own duration can be read against
	// it. It is the wall-time budget the governance layer resolved — after any
	// hot-fix override — and it is the one fact a timeout failure needs that the
	// duration alone cannot supply: "it hit its 500ms" and "the downstream was
	// slow" look identical without it. Zero means the call ran under no bound of
	// its own, so it is omitted rather than emitted as a misleading 0.
	if rd := rec.LastRound(); rd.Timeout > 0 {
		span.SetAttributes(attribute.Float64("timeout_ms", instrument.DurMs(rd.Timeout)))
	}
	// A fire is an EVENT at a point in the call, and a retried call can fire on
	// more than one attempt, so each fired round becomes its own span event —
	// carrying the attempt it happened in. A single attribute pair would have to
	// pick one fire and lose the rest; the event stream keeps the whole process.
	// The attributes keep the same names as before, so a trace UI reads a fire
	// the same way.
	for i, rd := range rec.Rounds {
		f := rd.Fault
		if f == nil {
			continue
		}
		attrs := []attribute.KeyValue{
			attribute.Int("attempt", i),
			attribute.String("resilience.injected", f.Error),
			attribute.String("resilience.injected.rule", f.Name),
			attribute.Float64("resilience.injected.delay_ms", float64(f.Delay)/float64(time.Millisecond)),
		}
		if f.Status > 0 {
			attrs = append(attrs, attribute.Int("resilience.injected.status", f.Status))
		}
		if f.RunID != "" {
			attrs = append(attrs, attribute.String("resilience.injected.run", f.RunID))
		}
		span.AddEvent("fault.injected", trace.WithAttributes(attrs...))
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

	e.emit(ctx, start, status, err, op, rec)
	return err
}

// emit produces the call-level signals — span (already ended by the caller),
// counter, call histogram, attempt histograms and the one access log. It runs
// after the inner call, so the recorder is complete.
func (e *callEmitter) emit(ctx context.Context, start time.Time, status string, err error, op chain.Operation, rec *chain.Recorder) {
	dur := time.Since(start)
	// The call's ONE outcome word, projected onto every signal below. It is
	// total (see [chain.CallErrorType]): a failed call is always attributable
	// to a gate, the downstream, or the caller, so no reader has to infer
	// "downstream" from an absent label.
	errorType := chain.CallErrorType(ctx, err)

	// The fault result is counted per EVENT: every attempt that fired is one
	// sample, labelled with the attempt it happened in. A retry re-rolls, so a
	// call that fired twice is two fires, not one call-level fact.
	if e.metrics != nil {
		// Declared attrs first, the rounds' produced after in round order, the
		// same order the span and the log use; the merged slice is fresh,
		// because appending in place would write into the shared Operation's
		// array.
		metricAttrs := op.Observe.Metric.Attrs
		for i := range rec.Rounds {
			if len(rec.Rounds[i].Observed.MetricAttrs) > 0 {
				metricAttrs = append(append([]attribute.KeyValue{}, metricAttrs...), rec.Rounds[i].Observed.MetricAttrs...)
			}
		}
		call := e.callOf(op, metricAttrs)
		e.metrics.Call(ctx, call, status, errorType, dur)
		// The attempt layer: one sample per round the chain ran, including a
		// round a gate refused before the downstream was touched (zero duration,
		// no peer, its outcome naming the gate) — so the refused volume is
		// visible here rather than silently missing.
		attemptCall := e.callOf(op, op.Observe.Metric.Attrs)
		for i, a := range rec.Rounds {
			e.metrics.Attempt(ctx, attemptCall, a.Status, a.ErrorType, a.Duration)
			if f := a.Fault; f != nil {
				e.metrics.Fault(ctx, call, Fault{
					Kind:    f.Error,
					Class:   f.Class,
					Status:  f.Status,
					Rule:    f.Name,
					Attempt: i,
					Inject:  f.Inject,
					Delay:   f.Delay,
				})
			}
		}
	}

	tag := instrument.Tag
	fields := []log.Field{
		log.String("system", e.system),
		log.String("service", e.target(op)),
	}
	quietSuccess := successQuiet(op)
	if op.Observe.Log.Tag != nil {
		tag = op.Observe.Log.Tag
	}
	// The log carries the operation's own vocabulary — attrs, detail and the
	// protocol's free-form fields alike — so a dashboard selecting failed
	// operations lands on the lines that explain them.
	fields = append(fields, op.Observe.Log.Fields...)
	for i := range rec.Rounds {
		fields = append(fields, rec.Rounds[i].Observed.LogFields...)
	}
	fields = append(fields, log.String("status", status), log.Float("duration_ms", instrument.DurMs(dur)))
	// The timeout sits next to the duration on purpose: "given 500ms, took
	// 498ms" is the line an operator reads to decide whether the bound is too
	// tight or the downstream too slow. Omitted when the call had no bound.
	if rd := rec.LastRound(); rd.Timeout > 0 {
		fields = append(fields, log.Float("timeout_ms", instrument.DurMs(rd.Timeout)))
	}
	if errorType != "" {
		fields = append(fields, log.String("resilience.error_type", string(errorType)))
	}
	// The retry side of the call, beside the timeout: "given 2 retries, used 1"
	// is what separates an EXHAUSTED budget from a call that never had one, and
	// it is the runtime fact a hot-fix override makes unknowable at declaration
	// time. The ceiling is staged on the round when it is created; the count is
	// derived from the rounds already recorded, never stored twice.
	fields = append(fields, retryFields(rec)...)
	// The access line is ONE line but the call may have fired on several rounds,
	// so the fired attempts are summarised here — kinds and rules in round order,
	// how many, and the total injected latency. With a single fire the values are
	// exactly the historical ones.
	var (
		faultCount  int
		faultKinds  []string
		faultRules  []string
		faultStatus int
		faultDelay  time.Duration
		faultRun    string
	)
	for _, rd := range rec.Rounds {
		f := rd.Fault
		if f == nil {
			continue
		}
		faultCount++
		faultKinds = append(faultKinds, f.Error)
		faultRules = append(faultRules, f.Name)
		if f.Status > 0 {
			faultStatus = f.Status
		}
		faultDelay += f.Delay
		if faultRun == "" {
			faultRun = f.RunID
		}
	}
	if faultCount > 0 {
		fields = append(fields,
			log.Int("resilience.injected_count", faultCount),
			log.String("resilience.injected", strings.Join(faultKinds, ",")),
			log.String("resilience.injected.rule", strings.Join(faultRules, ",")),
			log.Float("resilience.injected.delay_ms", float64(faultDelay)/float64(time.Millisecond)))
		if faultStatus > 0 {
			fields = append(fields, log.Int("resilience.injected.status", faultStatus))
		}
		if faultRun != "" {
			fields = append(fields, log.String("resilience.injected.run", faultRun))
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

// retryFields renders the retry side of the call: the ceiling its plan allowed
// ([chain.Round.MaxRetries]) and the retries it actually attempted (the rounds
// beyond the first). It returns nothing for a call that had no retry plan and
// never retried, so an ordinary single-attempt line is unchanged — the same
// "zero means the fact does not apply" rule the timeout field follows.
func retryFields(rec *chain.Recorder) []log.Field {
	rd := rec.LastRound()
	if rd.MaxRetries == 0 && len(rec.Rounds) == 1 {
		return nil
	}
	return []log.Field{
		log.Int("retry_max", rd.MaxRetries),
		log.Int("retry_count", len(rec.Rounds)-1),
	}
}

// spanName names the call's span: the operation's own action word, else the
// service label the emitter was built for.
func (e *callEmitter) spanName(op chain.Operation) string {
	if op.Request.Action != "" {
		return op.Request.Action
	}
	return e.service
}

// spanAttrs is what the span carries: the operation's declared
// [chain.Observe.Span.Attrs] plus the target the protocol's vocabulary does not
// name.
func (e *callEmitter) spanAttrs(op chain.Operation) []attribute.KeyValue {
	// The declared attrs are the protocol's vocabulary — db.system,
	// db.operation, url.full — and NONE of them says which service was
	// called, so a span would not read as a call edge at all without the target
	// added beside them. The metric label and the log field carry the same value
	// under this layer's shared name `service`; the span carries it under the
	// conventional trace name, which is what a service graph and a trace UI join
	// on.
	//
	// The slice is fresh: the declared attrs belong to the (shared, reusable)
	// Operation, so appending in place could write into a sibling call's array.
	return append([]attribute.KeyValue{attribute.String("peer.service", e.target(op))}, op.Observe.Span.Attrs...)
}

// target resolves the thing the call went TO: the operation's own target when it
// declared one, else the label this emitter was built with (the label the
// executor was built for). One resolver, so the metric label, the log field and
// the span attribute can never disagree about where the call went.
func (e *callEmitter) target(op chain.Operation) string {
	if op.Service != "" {
		return op.Service
	}
	return e.service
}

// callOf builds the metric seam's value for one call with the given labels.
func (e *callEmitter) callOf(op chain.Operation, attrs []attribute.KeyValue) Call {
	return Call{
		System:    e.system,
		Service:   e.service,
		Target:    e.target(op),
		Operation: op.Observe.Metric.Name,
		Attrs:     attrs,
	}
}
