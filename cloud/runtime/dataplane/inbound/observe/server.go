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

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	"go-spring.org/cloud/runtime/dataplane/kernel/breaker"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// serverInstrumentSet is the inbound layer's always-on signals: the per-request
// duration and status counter, and the circuit-breaker transition counter. One
// set per process, resolved lazily on first use so it binds to whichever
// providers are current then, and immutable afterwards. It holds no per-route
// state — the system and service labels travel with each observer, not here.
//
// It is a separate set from the outbound (resilience.client.*) family rather
// than one family with a direction attribute: the outcome sets genuinely differ
// (inbound has no retry, so no retry_budget_exceeded and no per-attempt timeout),
// and keeping them apart leaves existing outbound dashboards untouched.
type serverInstrumentSet struct {
	duration       metric.Float64Histogram
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
}

// serverInstruments is the one inbound instrument set this package uses for the
// whole process.
var serverInstruments = sync.OnceValue(buildServerInstruments)

func buildServerInstruments() *serverInstrumentSet {
	m := otel.Meter(instrument.Scope)
	in := &serverInstrumentSet{}
	in.duration, _ = m.Float64Histogram("resilience.server.duration",
		metric.WithDescription("Duration of inbound requests under resilience governance"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	in.calls, _ = m.Int64Counter("resilience.server.calls",
		metric.WithDescription("Number of inbound requests by inbound status"),
		metric.WithUnit("{request}"))
	in.breakerChanges, _ = m.Int64Counter("resilience.server.breaker.state_change",
		metric.WithDescription("Inbound circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	return in
}

// ServerObserver emits the standard inbound signals for ONE route: the request
// span, the request-level duration, the in-flight gauge, the always-on
// resilience.server.* counter and one access log — plus, for a route that
// declares an operation, that family's own request metrics.
//
// It is the inbound counterpart of [WrapClientExecutor], and it replaces BOTH the
// old executor-wrapping path and the observe-only self-emission path with one
// object: inbound governance is driven by a peer object (see go-spring.org/cloud/runtime/dataplane/inbound/admission)
// the middleware composes with this one, so the observer no longer wraps an
// executor. It offers the same two forms that object does — [ServerObserver.Run]
// (callback) and [ServerObserver.Begin]/[ServerObserver.End] (split) — so a
// protocol can drive it at whatever hook points it has.
type ServerObserver struct {
	*serverEmitter
}

// NewServerObserver builds the inbound observer for one route system/service.
// system is the server's name ("gin", "grpc"); service is the route label.
func NewServerObserver(system, service string) *ServerObserver {
	return &ServerObserver{serverEmitter: &serverEmitter{
		system:  system,
		service: service,
		ins:     serverInstruments(),
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
	// A described route carries a non-empty metric prefix; a route that declares
	// no description (the zero Operation) takes the fallback path.
	hasOp := op.Described()

	ctx, span := otel.Tracer(instrument.ComponentName).Start(ctx, o.serverSpanName(op, hasOp),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithLinks(op.Observe.Span.Links...),
		trace.WithAttributes(o.serverSpanAttrs(op, hasOp)...))

	var ins *opServerInstrumentSet
	if hasOp {
		ins = serverOperationInstruments(op.Observe.Metric.Name)
		ins.active.Add(ctx, 1, metric.WithAttributes(o.serverLabels(op.Observe.Metric.Attrs, "", nil)...))
	}
	return ctx, &Observation{
		observer: o,
		op:       op,
		hasOp:    hasOp,
		start:    time.Now(),
		span:     span,
		ins:      ins,
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

// OnBreakerStateChange satisfies [breaker.EventListener] for the route's
// inbound breaker. A trip (→open) is a route-level degradation worth flagging at
// Warn; recovery and half-open trial are Info — the same levelling the outbound
// listener uses.
func (o *ServerObserver) OnBreakerStateChange(service string, from, to breaker.State) {
	o.ins.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("system", o.system),
		attribute.String("service", o.service),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	fields := []log.Field{
		log.String("system", o.system),
		log.String("service", o.service),
		log.String("from", from.String()),
		log.String("to", to.String()),
	}
	if to == breaker.Open {
		log.Warn(context.Background(), instrument.Tag, "resilience: circuit breaker opened", fields...)
	} else {
		log.Info(context.Background(), instrument.Tag, "resilience: circuit breaker state changed", fields...)
	}
}

// Observation is one inbound request's observation state, held between
// [ServerObserver.Begin] and [ServerObserver.End] — on the object, never on the
// context. Its [Response] is where the handler records the late half of its
// operation (the response status code), which no declaration can know up front.
type Observation struct {
	observer *ServerObserver
	op       ServerOperation
	hasOp    bool
	start    time.Time
	span     trace.Span
	ins      *opServerInstrumentSet
	resp     *Response
}

// Response returns the holder the handler fills as it answers. It is never nil
// for an observation from [ServerObserver.Begin], so a handler may call it
// unconditionally.
func (ob *Observation) Response() *Response { return ob.resp }

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
	if err != nil {
		ob.span.SetStatus(codes.Error, err.Error())
	}
	ob.span.End()

	dur := time.Since(ob.start)
	durMs := float64(dur.Nanoseconds()) / 1e6

	// resilience.server.* is the inbound layer's own signal and keeps its shape
	// whether or not the route declared an operation. The emission context is the
	// span's own, not a request context the caller owns.
	emitCtx := context.Background()
	labels := []attribute.KeyValue{
		attribute.String("system", o.system),
		attribute.String("service", o.service),
		attribute.String("status", status),
	}
	if errorType := serverErrorType(err); errorType != "" {
		labels = append(labels, attribute.String("resilience.error_type", errorType))
	}
	o.ins.duration.Record(emitCtx, dur.Seconds(), metric.WithAttributes(labels...))
	o.ins.calls.Add(emitCtx, 1, metric.WithAttributes(labels...))

	if ob.ins != nil {
		// The declared family metrics: the route's own bounded vocabulary, the
		// response half, and the shared status axis. The family names are the HTTP
		// family's — `<prefix>.request.duration` / `<prefix>.active_requests` —
		// because a server's signal is a request, and those are the names every
		// dashboard already reads.
		opLabels := o.serverLabels(op.Observe.Metric.Attrs, status, respAttrs)
		ob.ins.duration.Record(emitCtx, dur.Seconds(), metric.WithAttributes(opLabels...))
		ob.ins.active.Add(emitCtx, -1, metric.WithAttributes(o.serverLabels(op.Observe.Metric.Attrs, "", nil)...))
	}

	tag := instrument.Tag
	fields := []log.Field{
		log.String("system", o.system),
		log.String("service", o.service),
	}
	if ob.hasOp {
		if op.Observe.Log.Tag != nil {
			tag = op.Observe.Log.Tag
		}
		fields = append(fields, op.Observe.Log.Fields...)
		fields = append(fields, instrument.AttrsToLogFields(respAttrs)...)
	}
	fields = append(fields, log.String("status", status), log.Float("duration_ms", durMs))

	if err != nil {
		log.Warn(emitCtx, tag, "server call failed", append(fields, log.Err(err))...)
		return
	}
	quiet := !ob.hasOp || op.Observe.Log.QuietSuccess
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

	// ins is the shared inbound instrument set; the tracer is looked up per use,
	// so a captured otel.Tracer never goes stale.
	ins *serverInstrumentSet
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
// server-side counterpart of chain.ClassifyErrorType; the two vocabularies are
// deliberately separate, so the observer does not import the client's sentinels.
func serverErrorType(err error) string {
	switch {
	case errors.Is(err, admission.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, admission.ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, admission.ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return ""
	}
}

// serverSpanName names an inbound span: the route's own word when it declared
// one, the route service otherwise — which is what this layer's spans have
// always been named in the undeclared case.
func (e *serverEmitter) serverSpanName(op ServerOperation, hasOp bool) string {
	if hasOp && op.Action != "" {
		return op.Action
	}
	return e.service
}

// serverSpanAttrs is what the route's span carries: its declared
// [chain.Observe.Span.Attrs] when it declared an operation, else this layer's own
// inbound labels.
func (e *serverEmitter) serverSpanAttrs(op ServerOperation, hasOp bool) []attribute.KeyValue {
	if hasOp {
		return op.Observe.Span.Attrs
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

// responseKey is the private key under which a context carries the response
// holder. An unexported empty-struct type keeps it collision-free.
type responseKey struct{}

// Response is where a server handler records what only it knows once it has
// answered — in practice the response status code. A nil *Response is valid and
// discards every write, so a handler may call [ResponseFrom] and use the result
// without checking: a call that runs outside a server emitter is simply not
// recorded, rather than panicking on the path that is least tested.
type Response struct {
	mu    sync.Mutex
	attrs []attribute.KeyValue
}

// WithResponse returns ctx together with the holder a handler fills as it
// answers. The emitter installs it; nothing else should need to.
func WithResponse(ctx context.Context) (context.Context, *Response) {
	r := &Response{}
	return context.WithValue(ctx, responseKey{}, r), r
}

// ResponseFrom returns the holder ctx carries, or nil when it carries none —
// which is the ordinary case for a call that is not an inbound request.
func ResponseFrom(ctx context.Context) *Response {
	r, _ := ctx.Value(responseKey{}).(*Response)
	return r
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
