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
	"unicode/utf8"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies the OTel tracer/meter for this kit. Each Observer is
// additionally labeled per client instance by the system argument, so spans and
// metrics carry the concrete db.system / messaging.system rather than this name.
const tracerName = "go-spring.org/cloud/observe"

// durationBuckets are the OTel histogram boundaries for operation duration (in
// seconds). They span the range that matters for client ops — sub-millisecond
// cache hits through multi-second queries — mirroring the server-side buckets
// used by starter-gin so client and server latencies share a scale.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// SemConv describes one OTel semantic-convention namespace: the metric-name
// prefix and the attribute keys an Observer tags its spans, metrics, and access
// logs with. The four fields travel together — a metric prefix is meaningless
// without its matching attribute names — so the general constructor New takes
// the whole bundle rather than loose strings. Start from DBSemConv or
// MessagingSemConv when defining a custom one.
type SemConv struct {
	Domain    string // metric-name prefix: "db.client" -> db.client.operation.duration
	SystemKey string // attribute naming the client system: "db.system"
	OpKey     string // attribute naming the operation: "db.operation"
	ArgKey    string // attribute carrying the operation argument: "db.statement"
}

// DBSemConv is the semantic convention for database/cache clients: metrics
// under db.client.*, attributes from the OTel db namespace.
var DBSemConv = SemConv{
	Domain:    "db.client",
	SystemKey: "db.system",
	OpKey:     "db.operation",
	ArgKey:    "db.statement",
}

// MessagingSemConv is the semantic convention for messaging clients: metrics
// under messaging.client.*, attributes from the OTel messaging namespace.
var MessagingSemConv = SemConv{
	Domain:    "messaging.client",
	SystemKey: "messaging.system",
	OpKey:     "messaging.operation",
	ArgKey:    "messaging.destination.name",
}

// ResilienceSemConv is the semantic convention for resilience-protected calls
// (the observe/resilience executor bridge): metrics under resilience.*, span
// kind internal. The "operation" of a resilience call is the protected resource
// name, so OpKey names it resilience.resource.
var ResilienceSemConv = SemConv{
	Domain:    "resilience",
	SystemKey: "resilience.system",
	OpKey:     "resilience.resource",
	ArgKey:    "resilience.arg",
}

// statusKey labels the coarse ok/error dimension on metrics and the access log.
// It is the kit's own attribute (not OTel semconv): the status code is the
// metric dimension, the error detail lives on the span/log.
const statusKey = "status"

// Observer emits trace span + duration/in-flight metric + access log for one
// client instance. Build one per configured instance with NewDB (database /
// cache), NewProducer/NewConsumer (messaging), or New (a custom semantic
// convention), then call Start at the start of each operation and End on the
// returned Span when it completes.
//
// It rides the OTel globals: when starter-otel is not imported the global
// TracerProvider and MeterProvider are no-ops, so trace+metric add negligible
// overhead and the span's SpanContext stays invalid (no trace_id on the access
// log). The access log itself always emits through the project log package,
// gated by ObserveConfig.Level.
type Observer struct {
	system string  // db.system / messaging.system attribute value, e.g. "redis"
	sc     SemConv // semantic-convention namespace this Observer emits into
	kind   trace.SpanKind

	trace  bool // emit a client span per op (default true)
	metric bool // emit duration/in-flight metrics per op (default true)

	tracer   trace.Tracer
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
	logTag   *log.Tag
	cfg      ObserveConfig
	skipOps  map[string]struct{}
}

// Option customizes an Observer.
type Option func(*Observer)

// WithoutTraceAndMetric disables the kit's span and metric emission, keeping
// only the access log. Use when the client library already provides trace+metric
// (go-redis via redisotel, kafka via kotel) and you want the kit only to fill the
// access-log gap, avoiding duplicate spans/metrics. The log still rides the
// caller's context, so it picks up the library span's trace_id for correlation.
func WithoutTraceAndMetric() Option {
	return func(o *Observer) { o.trace = false; o.metric = false }
}

// WithoutTrace disables only the kit's span emission, keeping metric + access
// log. Use when the client library already emits spans (elasticsearch's
// transport instrumentation) but you still want the kit's metric and log.
func WithoutTrace() Option {
	return func(o *Observer) { o.trace = false }
}

// NewDB builds an Observer for a database/cache client (span kind = client,
// metrics under db.client.operation.duration). system labels the spans/metrics
// (e.g. "redis", "mysql", "mongo"). cfg controls the access log.
func NewDB(system string, cfg ObserveConfig, opts ...Option) *Observer {
	return New(system, DBSemConv, trace.SpanKindClient, cfg, opts...)
}

// NewProducer builds an Observer for a messaging publisher (span kind = producer,
// metrics under messaging.client.operation.duration). system is e.g. "nats",
// "kafka". See NewDB for cfg.
func NewProducer(system string, cfg ObserveConfig, opts ...Option) *Observer {
	return New(system, MessagingSemConv, trace.SpanKindProducer, cfg, opts...)
}

// NewConsumer builds an Observer for a messaging consumer (span kind = consumer).
// See NewProducer.
func NewConsumer(system string, cfg ObserveConfig, opts ...Option) *Observer {
	return New(system, MessagingSemConv, trace.SpanKindConsumer, cfg, opts...)
}

// New builds an Observer for a semantic-convention namespace the named
// constructors don't cover — the escape hatch for a custom client type with its
// own conventions (metric prefix + attribute names, see SemConv). kind selects
// the span kind. Prefer NewDB/NewProducer/NewConsumer when they fit; they
// guarantee a coherent SemConv/span-kind pairing.
func New(system string, sc SemConv, kind trace.SpanKind, cfg ObserveConfig, opts ...Option) *Observer {
	o := &Observer{
		system: system,
		sc:     sc,
		kind:   kind,
		trace:  true,
		metric: true,
		cfg:    cfg,
		skipOps: func() map[string]struct{} {
			m := make(map[string]struct{}, len(cfg.SkipOps))
			for _, op := range cfg.SkipOps {
				m[op] = struct{}{}
			}
			return m
		}(),
	}
	for _, opt := range opts {
		opt(o)
	}
	// Instruments are created unconditionally: they are no-ops when the metric
	// flag is off (Start/End skip them), and creating them is cheap/idempotent.
	o.tracer = otel.Tracer(tracerName)
	meter := otel.Meter(tracerName)
	o.duration, _ = meter.Float64Histogram(
		sc.Domain+".operation.duration",
		metric.WithDescription("Duration of "+system+" client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	o.active, _ = meter.Int64UpDownCounter(
		sc.Domain+".active_requests",
		metric.WithDescription("Number of in-flight "+system+" client operations"),
		metric.WithUnit("{request}"),
	)
	o.logTag = log.RegisterAppTag(system, "access")
	return o
}

// Start begins a client operation. It opens a span, records the start time, and
// bumps the in-flight gauge; the returned Span's End records the duration
// histogram, balances the gauge, ends the span, and emits the access log.
//
// op is the operation name (e.g. "GET", "query", "publish"); it becomes the span
// name and the db.operation / messaging.operation attribute. arg is the optional
// operation argument (command key, sql statement, cache key, topic) captured in
// detailed log mode and as a span attribute, bounded by ObserveConfig.MaxArgBytes.
//
// If op is listed in ObserveConfig.SkipOps, Start returns a skipped Span whose End is
// a no-op — so a noisy operation (a chatty PING, a health probe) emits no span,
// metric, or log.
//
// Start must be followed by exactly one Span.End (use defer in the seam).
func (o *Observer) Start(ctx context.Context, op, arg string, attrs ...attribute.KeyValue) (context.Context, *Span) {
	if _, skip := o.skipOps[op]; skip {
		return ctx, &Span{o: o, ctx: ctx, op: op, skipped: true}
	}
	attrs = append([]attribute.KeyValue{
		attribute.String(o.sc.SystemKey, o.system),
		attribute.String(o.sc.OpKey, op),
	}, attrs...)
	if arg != "" {
		attrs = append(attrs, attribute.String(o.sc.ArgKey, boundArg(arg, o.cfg.maxArg())))
	}
	var span trace.Span
	if o.trace {
		ctx, span = o.tracer.Start(ctx, op, trace.WithSpanKind(o.kind), trace.WithAttributes(attrs...))
	}
	var inflight metric.MeasurementOption
	if o.metric {
		inflight = metric.WithAttributes(
			attribute.String(o.sc.SystemKey, o.system),
			attribute.String(o.sc.OpKey, op),
		)
		o.active.Add(ctx, 1, inflight)
	}
	return ctx, &Span{o: o, ctx: ctx, span: span, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// Span is the handle returned by Observer.Start. End records the operation's
// outcome; it must be called exactly once.
type Span struct {
	o        *Observer
	ctx      context.Context
	span     trace.Span
	op       string
	arg      string
	start    time.Time
	inflight metric.MeasurementOption
	skipped  bool
}

// SetArg sets or replaces the operation argument captured for the access log
// and (as a span attribute) between Start and End. Use when the argument is not
// known at Start time — e.g. a gorm callback knows the SQL only in the After
// callback, after the Before callback already opened the span for timing. Call
// before End; a no-op on a skipped Span.
func (s *Span) SetArg(arg string) {
	if s.skipped {
		return
	}
	s.arg = arg
	if s.span != nil && arg != "" {
		s.span.SetAttributes(attribute.String(s.o.sc.ArgKey, boundArg(arg, s.o.cfg.maxArg())))
	}
}

// End records the operation: the duration histogram, balances the in-flight
// gauge, ends the span (recording err if non-nil), and emits the access log per
// ObserveConfig.Level. Pass the operation's error, or nil on success.
func (s *Span) End(err error, attrs ...attribute.KeyValue) {
	if s.skipped {
		return
	}
	o := s.o
	dur := time.Since(s.start)

	if o.metric {
		durAttrs := []attribute.KeyValue{
			attribute.String(o.sc.SystemKey, o.system),
			attribute.String(o.sc.OpKey, s.op),
			attribute.String(statusKey, statusOf(err)),
		}
		durAttrs = append(durAttrs, attrs...)
		o.duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(durAttrs...))
		o.active.Add(s.ctx, -1, s.inflight)
	}
	if s.span != nil {
		if err != nil {
			s.span.SetStatus(codes.Error, err.Error())
			s.span.RecordError(err)
		}
		s.span.End()
	}

	if o.cfg.Enabled() {
		o.emitLog(s.ctx, s.op, s.arg, dur, err)
	}
}

// emitLog writes the per-operation access record. Success records at Info; an
// error at Warn (client op errors are operation-level failures worth flagging,
// not process-fatal). Detailed mode appends the bounded operation argument.
func (o *Observer) emitLog(ctx context.Context, op, arg string, dur time.Duration, err error) {
	fields := []log.Field{
		log.String(o.sc.OpKey, op),
		log.String(statusKey, statusOf(err)),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if o.cfg.Detailed() && arg != "" {
		fields = append(fields, log.String(o.sc.ArgKey, boundArg(arg, o.cfg.maxArg())))
	}
	if err != nil {
		fields = append(fields, log.Any("error", err))
		log.Warn(ctx, o.logTag, fields...)
		return
	}
	log.Info(ctx, o.logTag, fields...)
}

// statusOf maps an error to a coarse status string for metric dimensions and the
// log: nil => "ok", any error => "error". Keeping it binary (rather than
// classifying error kinds) matches the db/messaging OTel convention where the
// status code is the dimension and the error detail lives on the span/log.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// boundArg truncates arg to at most n bytes on a rune boundary, appending an
// ellipsis marker when it was shortened, so a large statement or payload can't
// dominate the log line or span attribute.
func boundArg(arg string, n int) string {
	if len(arg) <= n {
		return arg
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(arg[cut]) {
		cut--
	}
	return arg[:cut] + "...(truncated)"
}
