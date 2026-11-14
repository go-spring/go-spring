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

// observe.go is the scheduling layer's observability surface: the events a fire
// reports, the [Observer] seam, the default OTel-backed adapter, and the
// per-fire access log (which stays on the log package).

package scheduling

import (
	"context"
	"errors"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/randutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// componentName is the instrumentation componentName name every meter and tracer in this
// package reports under.
const componentName = "go-spring.org/cloud/scheduling"

// Observability contract.
//
// Every fire — whether it ran or was swallowed — is reported through [Observer],
// and one status value drives the metric, the span and the log so they can never
// disagree:
//
//	status     ok | error | panic | skipped_policy | skipped_lock
//
// A skipped fire is one a concurrency policy dropped (policy) or another replica
// was running (lock); telling them apart matters because under multi-replica
// de-duplication lock skips are routine, while a policy skip means a job is not
// keeping up.

// lagBuckets are the wake-up-lag histogram boundaries (seconds). Lag is the
// delay between a fire's scheduled instant and when its run actually started, so
// the interesting range is sub-millisecond to seconds — finer than
// [attrs.DurationBuckets], which starts at 5ms.
var lagBuckets = []float64{0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}

// accessTag is the static log tag for the per-fire log lines.
var accessTag = log.RegisterAppTag("scheduler", "access")

// Observer receives the scheduling layer's METRIC events. It is the seam for
// the metric plane only: spans and the access log are standard OTel/log calls
// made by this package, not part of this contract. A nil Observer disables the
// metrics; implementations must not block.
type Observer interface {
	// Run records one finished run. status is ok | error | panic.
	Run(ctx context.Context, job, status string, duration, lag time.Duration)

	// Skipped records a fire that did not run, with reason "policy" or "lock".
	Skipped(ctx context.Context, job, reason string)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.runs, err = m.Int64Counter("scheduling.runs",
		metric.WithDescription("Scheduled job fires by status"),
		metric.WithUnit("{fire}"))
	if err != nil {
		return nil, errutil.Explain(err, "scheduling: register scheduling.runs")
	}
	o.duration, err = m.Float64Histogram("scheduling.run.duration",
		metric.WithDescription("Duration of a job run"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "scheduling: register scheduling.run.duration")
	}
	// A rising lag is the scheduler's own health signal: the schedule is not
	// drifting, but runs are starting later and later, which points at a process
	// that is saturated or stalled.
	o.lag, err = m.Float64Histogram("scheduling.lag",
		metric.WithDescription("Delay between a fire's scheduled instant and the start of its run"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(lagBuckets...))
	if err != nil {
		return nil, errutil.Explain(err, "scheduling: register scheduling.lag")
	}
	return o, nil
}

// otelObserver adapts the layer's events to OTel metrics.
type otelObserver struct {
	runs     metric.Int64Counter
	duration metric.Float64Histogram
	lag      metric.Float64Histogram
}

// Run implements [Observer].
func (o *otelObserver) Run(ctx context.Context, job, status string, duration, lag time.Duration) {
	attrsOpt := metric.WithAttributes(
		attribute.String("job", job),
		attribute.String("status", status),
	)
	o.runs.Add(ctx, 1, attrsOpt)
	o.duration.Record(ctx, duration.Seconds(), attrsOpt)
	o.lag.Record(ctx, lag.Seconds(), metric.WithAttributes(attribute.String("job", job)))
}

// Skipped implements [Observer].
func (o *otelObserver) Skipped(ctx context.Context, job, reason string) {
	o.runs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("job", job),
		attribute.String("status", "skipped_"+reason),
	))
}

// traceRun opens the span for one run. Span handling is the STANDARD OTel
// capability, not part of the metric seam.
func traceRun(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer(componentName).Start(ctx, "scheduler.job "+name,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String("scheduler.job.name", name)),
	)
}

// startRun opens the run's span. With no active tracer no span is produced, so
// the run gets a synthetic trace_id on its context and the per-fire logs still
// carry a correlation id.
func startRun(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, span := traceRun(ctx, name)
	if !span.SpanContext().IsValid() {
		ctx = log.WithFields(ctx, log.String("trace_id", randutil.Hex(16)))
	}
	return ctx, span
}

// endRun finishes the run's span with its outcome.
func endRun(span trace.Span, err error) {
	span.SetAttributes(attribute.String("status", statusOfRun(err)))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// statusOfRun classifies a run's outcome.
func statusOfRun(err error) string {
	switch {
	case errors.Is(err, ErrJobPanicked):
		return "panic"
	case err != nil:
		return "error"
	default:
		return "ok"
	}
}

// statusOf classifies one fire. A skipped fire is a whole status of its own,
// with its reason folded into the value rather than made a second dimension.
func statusOf(ev event) string {
	if ev.Skipped {
		return "skipped_" + ev.Reason
	}
	return statusOfRun(ev.Err)
}

// recordLog writes the per-fire access line. The status drives the level: a
// panic or a failed run at Error, an ordinary fire — and a swallowed one, which
// is routine under multi-replica de-duplication — at Debug.
func recordLog(ev event) {
	status := statusOf(ev)
	ctx := context.Background()
	fields := []log.Field{
		log.String("job", ev.Name),
		log.String("status", status),
	}
	switch status {
	case "panic":
		log.Error(ctx, accessTag, ev.Err, "scheduler: job panicked", append(fields,
			log.Float("duration_ms", ms(ev.Duration)))...)
	case "error":
		log.Error(ctx, accessTag, ev.Err, "scheduler: job failed", append(fields,
			log.Float("duration_ms", ms(ev.Duration)))...)
	case "skipped_policy", "skipped_lock":
		log.Debug(ctx, accessTag, "scheduler: job skipped", func() []log.Field {
			return append(fields,
				log.String("reason", ev.Reason))
		})
	default:
		log.Debug(ctx, accessTag, "scheduler: job ran", func() []log.Field {
			return append(fields,
				log.Float("duration_ms", ms(ev.Duration)))
		})
	}
}

// recordSkip reports a fire that did not run, with reason "policy" or "lock".
// err is the failure that prevented the run (a lock acquisition error), nil
// when the fire was simply dropped.
func recordSkip(scheduled time.Time, name, reason string, err error, obs Observer) {
	if obs != nil {
		obs.Skipped(context.Background(), name, reason)
	}
	recordLog(event{Name: name, Scheduled: scheduled, Skipped: true, Reason: reason, Err: err})
}

// ms renders a duration the way the other domain packages' access logs do, so
// the field is comparable across packages.
func ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}
