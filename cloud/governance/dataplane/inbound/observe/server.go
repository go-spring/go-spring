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
	"errors"
	"sync"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/cloud/observability/instrument"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ServerObserver emits the standard inbound signals for ONE route: the request
// span, the request-level duration, the in-flight gauge, the always-on
// resilience.server.* counter and one access log — plus the route family's own
// request metrics.
//
// It is the inbound counterpart of [WrapClientExecutor], and it replaces BOTH the
// old executor-wrapping path and the observe-only self-emission path with one
// object: inbound governance is driven by a peer object (see go-spring.org/cloud/governance/dataplane/inbound/admission)
// the middleware composes with this one, so the observer no longer wraps an
// executor. It offers the same two forms that object does — [ServerObserver.Run]
// (callback) and [ServerObserver.Begin]/[ServerObserver.End] (split) — so a
// protocol can drive it at whatever hook points it has.
type ServerObserver struct {
	*serverEmitter
}

// NewServerObserver builds the inbound observer for one route system/service.
// system is the server's name ("gin", "grpc"); service is the route label. m may
// be nil, which disables the metrics.
func NewServerObserver(system, service string, m Metrics) *ServerObserver {
	return &ServerObserver{serverEmitter: &serverEmitter{
		system:  system,
		service: service,
		metrics: m,
	}}
}

// Run is the PREFERRED form: it opens the span, hands fn the observed context and
// the [Observation] (so a handler can record the late half of its operation — the
// response status code — while it answers), then emits the signals with fn's
// outcome.
func (o *ServerObserver) Run(ctx context.Context, op ServerOperation, fn func(context.Context, *Observation) error) error {
	ctx, ob := o.Begin(ctx, op)
	err := fn(ctx, ob)
	o.End(ob, err)
	return err
}

// Begin is the SPLIT form: it opens the span and the in-flight gauge and returns
// the per-request [Observation] to close with [ServerObserver.End]. It exists for
// a protocol whose request cannot be wrapped in one closure, mirroring
// admission.Guard's Admit.
func (o *ServerObserver) Begin(ctx context.Context, op ServerOperation) (context.Context, *Observation) {
	ctx, span := otel.Tracer(instrument.ComponentName).Start(ctx, o.serverSpanName(op),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithLinks(op.Observe.Span.Links...),
		trace.WithAttributes(o.serverSpanAttrs(op)...))

	if o.metrics != nil {
		o.metrics.Active(ctx, o.requestOf(op), 1)
	}
	return ctx, &Observation{
		observer: o,
		op:       op,
		start:    time.Now(),
		span:     span,
		resp:     &Response{},
	}
}

// End emits an observation's signals with err as the outcome and closes its span.
// It is a no-op for a nil observation.
func (o *ServerObserver) End(ob *Observation, err error) {
	if ob == nil {
		return
	}
	ob.end(err)
}

// Observation is one inbound request's observation state, held between
// [ServerObserver.Begin] and [ServerObserver.End] — on the object, never on the
// context. Its [Response] is where the handler records the late half of its
// operation (the response status code), which no declaration can know up front.
type Observation struct {
	observer *ServerObserver
	op       ServerOperation
	start    time.Time
	span     trace.Span
	resp     *Response

	// The admission/fault stage's result, recorded through [MarkInjected] so
	// the span can say a failure was set on fire on purpose and by which rule.
	// Kept on the object (never the context), like the rest of the observation's
	// state.
	injected *kfault.Result
}

// Response returns the holder the handler fills as it answers. It is never nil
// for an observation from [ServerObserver.Begin], so a handler may call it
// unconditionally.
func (ob *Observation) Response() *Response { return ob.resp }

// MarkInjected records the fault decision the admission stage made about this
// request, so the span can tag it — an injected failure must never be read as a
// real one, and "which experiment did this?" must be answerable from the trace.
// The outbound side has the same signal on the call record; this is the inbound
// half, where no record exists.
//
// A nil observation (governance without observation) and a nil rule (no
// experiment fired) are no-ops, so a caller records unconditionally.
func (ob *Observation) MarkInjected(f kfault.Result) {
	if ob == nil || !f.Fired() {
		return
	}
	ob.injected = &f
}

// end emits the observation's signals with err as the outcome and closes the span.
func (ob *Observation) end(err error) {
	o := ob.observer
	op := ob.op

	status := "ok"
	if err != nil {
		status = "error"
	}
	respAttrs := ob.resp.Attributes()

	// The response half travels the same two channels the declaration's Detail
	// does — span and log — while the bounded part of it (a status code) also
	// labels the family metric below.
	ob.span.SetAttributes(attribute.String("status", status))
	if len(respAttrs) > 0 {
		ob.span.SetAttributes(respAttrs...)
	}
	// Same attribute names as the outbound edge, so a trace viewer reads a fire
	// the same way on both sides of a call. At most one rule governed the
	// request, so the call-level and rule-level facts are the same scalars.
	if ob.injected != nil {
		attrs := []attribute.KeyValue{
			attribute.String("resilience.injected", "response"),
			attribute.Int("resilience.injected.status", ob.injected.Status),
			attribute.String("resilience.injected.rule", ob.injected.Name),
			attribute.Float64("resilience.injected.delay_ms", float64(ob.injected.Delay)/float64(time.Millisecond)),
		}
		if ob.injected.RunID != "" {
			attrs = append(attrs, attribute.String("resilience.injected.run", ob.injected.RunID))
		}
		ob.span.SetAttributes(attrs...)
	}
	if err != nil {
		ob.span.SetStatus(codes.Error, err.Error())
	}
	ob.span.End()

	dur := time.Since(ob.start)
	durMs := float64(dur.Nanoseconds()) / 1e6

	// resilience.server.* is the inbound layer's own signal and keeps its shape
	// independent of the route family's metrics. The emission context is the
	// span's own, not a request context the caller owns.
	emitCtx := context.Background()
	if o.metrics != nil {
		req := o.requestOf(op)
		req.Attrs = op.Observe.Metric.Attrs
		req.ResponseAttrs = respAttrs
		req.Status = status
		req.ErrorType = serverErrorType(err)
		req.Duration = dur
		o.metrics.Request(emitCtx, req)
		if !op.Observe.Metric.Disabled {
			o.metrics.Active(emitCtx, req, -1)
		}
		if ob.injected != nil {
			o.metrics.Fault(emitCtx, req, Fault{
				Rule:   ob.injected.Name,
				Status: ob.injected.Status,
				Inject: ob.injected.Inject,
				Delay:  ob.injected.Delay,
			})
		}
	}

	tag := instrument.Tag
	fields := []log.Field{
		log.String("system", o.system),
		log.String("service", o.service),
	}
	if op.Observe.Log.Tag != nil {
		tag = op.Observe.Log.Tag
	}
	fields = append(fields, op.Observe.Log.Fields...)
	fields = append(fields, instrument.AttrsToLogFields(respAttrs)...)
	fields = append(fields, log.String("status", status), log.Float("duration_ms", durMs))

	if err != nil {
		log.Warn(emitCtx, tag, "server call failed", append(fields, log.Err(err))...)
		return
	}
	quiet := op.Observe.Log.QuietSuccess
	if quiet {
		log.Debug(emitCtx, tag, "server call succeeded", func() []log.Field { return fields })
		return
	}
	log.Info(emitCtx, tag, "server call finished", fields...)
}

// serverEmitter holds the route identity and instrument set shared by the
// observer's Begin/End halves.
type serverEmitter struct {
	system  string
	service string

	// metrics is the metric seam; nil disables the metrics. The tracer is looked
	// up per use, so a captured otel.Tracer never goes stale.
	metrics Metrics
}

// requestOf maps one server operation onto the metric seam's value.
func (e *serverEmitter) requestOf(op ServerOperation) Request {
	r := Request{
		System:  e.system,
		Service: e.service,
		Attrs:   op.Observe.Metric.Attrs,
	}
	if !op.Observe.Metric.Disabled {
		r.Operation = op.Observe.Metric.Name
	}
	return r
}

// opServerInstrumentSet is the INBOUND counterpart: the two signals a declared
// server route reports under, named the way the HTTP and RPC families already
// name them. There is no attempt histogram — inbound serving has no retry stage
// to measure, by construction (inbound has one attempt; see admission.Policy).
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
	m := otel.Meter(instrument.ComponentName)
	s := &opServerInstrumentSet{}
	var err error
	s.duration, err = m.Float64Histogram(prefix+".request.duration",
		metric.WithDescription("Duration of one inbound request"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	instrument.ReportInstrumentError(prefix, err)
	s.active, err = m.Int64UpDownCounter(prefix+".active_requests",
		metric.WithDescription("Number of inbound requests currently being served"),
		metric.WithUnit("{request}"))
	instrument.ReportInstrumentError(prefix, err)
	opServerInstruments.m[prefix] = s
	return s
}

// serverErrorType names the class of a refusal in admission's vocabulary. It is the
// server-side counterpart of chain.RoundErrorType; the two vocabularies are
// deliberately separate, so the observer does not import the client's sentinels.
func serverErrorType(err error) string {
	switch {
	case errors.Is(err, admission.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, admission.ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return ""
	}
}

// serverSpanName names an inbound span: the route's own action word, else the
// route service the observer was built for.
func (e *serverEmitter) serverSpanName(op ServerOperation) string {
	if op.Action != "" {
		return op.Action
	}
	return e.service
}

// serverSpanAttrs is what the route's span carries: its declared
// [ServerObserve.Span] attributes.
func (e *serverEmitter) serverSpanAttrs(op ServerOperation) []attribute.KeyValue {
	return op.Observe.Span.Attrs
}

// The response holder carries the half of a SERVER operation's identity that
// only exists once the handler has answered.
//
// A client declares its whole operation up front — it knows the destination and
// the verb before it calls. A server cannot: the response status code is the
// outcome of the work, so it is unknown when the declaration is read at the
// request's entry. [Response] is the holder that closes that gap: the emitter
// installs one, the handler fills it on the way out, the emitter reads it back
// and attaches what it finds to the span and the access log.
//
// It is the inbound counterpart of the client-side [Recorder], and deliberately
// its mirror in shape: a mutable carrier the emitter owns and reads, never a
// signal the handler emits.
//
// What a handler records here is the LATE HALF OF THE OPERATION'S BOUNDED
// ATTRIBUTES — the same kind of value [chain.Observe.Span.Attrs] carries, just
// not known in time to be declared. A response status code is the example:
// bounded, and worth a metric label. So these reach all three signals, exactly
// as the declared attributes do.
//
// It is ONE list mirrored to the three signals, which is the one place this side
// is narrower than the outbound one: a value a handler wanted on the span alone —
// or on the log alone — has no way to say so here, while [chain.Recorder.Observed]
// lets an outbound call address each group separately, the way the declared block
// does. That is a deliberate difference in reach, not in kind: both are "the
// block the call filled", and the inbound one is the shape its seam needs (the
// handler is handed nothing but the context).

// Response is where a server handler records what only it knows once it has
// answered — in practice the response status code. The emitter hands it to the
// handler through [Observation.Response]; a nil *Response discards every write,
// so a handler may use it without checking.
type Response struct {
	mu    sync.Mutex
	attrs []attribute.KeyValue
}

// Add records attributes the handler learned while answering. It is nil-safe.
//
// Later values for the same key win, so a handler that writes the same key twice
// (a redirect chain, say) leaves the last answer standing — the same rule
// [attrs.WithSpanAttributes] applies.
func (r *Response) Add(attrs ...attribute.KeyValue) {
	if r == nil || len(attrs) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attrs = append(r.attrs, attrs...)
}

// Attributes returns a copy of what the handler recorded, in the order it
// recorded it.
func (r *Response) Attributes() []attribute.KeyValue {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, len(r.attrs))
	copy(out, r.attrs)
	return out
}
