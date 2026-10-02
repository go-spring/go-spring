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

package scheduling

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

// scope is the instrumentation scope name every meter and tracer in this
// package reports under.
const scope = "go-spring.org/cloud/scheduling"

// Observability contract.
//
// Every fire — whether it ran or was swallowed — is reported here, becoming a
// metric and a log line. One status value drives both, so the metric can never
// disagree with the log:
//
//	status     ok | error | panic | skipped_policy | skipped_lock
//
// The status answers "how did this fire end?" in a single dimension, the same
// shape the lock package uses for its status. A skipped fire is one that a
// concurrency policy dropped (policy) or one another replica was running (lock);
// telling them apart matters because under multi-replica de-duplication lock
// skips are routine, while a policy skip means a job is not keeping up.
//
// The job name is a metric dimension. Unlike a lock key — chosen by the caller,
// with unbounded cardinality — a job name is fixed in code, so its cardinality
// is the number of registered jobs.
//
// Runs also get a span on the global otel pipeline; a swallowed fire has no run
// to trace, so it appears in metrics and logs only.

// lagBuckets are the wake-up-lag histogram boundaries (seconds). Lag is the
// delay between a fire's scheduled instant and when its run actually started, so
// the interesting range is sub-millisecond to seconds — finer than
// [observability.DurationBuckets], which starts at 5ms.
var lagBuckets = []float64{0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}

// accessTag is the static log tag for the per-fire log lines.
var accessTag = log.RegisterAppTag("scheduler", "access")

// instrumentSet bundles the metrics a fire records: one per process, resolved
// lazily on first use so the set binds to whichever OTel global provider is
// current then (it is installed during wiring, after package inits), and
// immutable afterwards. It holds no per-fire state.
type instrumentSet struct {
	runs     metric.Int64Counter
	duration metric.Float64Histogram
	lag      metric.Float64Histogram
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(scope)
	runs, _ := m.Int64Counter("scheduling.runs",
		metric.WithDescription("Scheduled job fires by status"),
		metric.WithUnit("{fire}"))
	duration, _ := m.Float64Histogram("scheduling.run.duration",
		metric.WithDescription("Duration of a job run"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	// A rising lag is the scheduler's own health signal: the schedule is not
	// drifting (the next fire stays anchored on the planned instant), but runs are
	// starting later and later, which points at a process that is saturated or
	// stalled.
	lag, _ := m.Float64Histogram("scheduling.lag",
		metric.WithDescription("Delay between a fire's scheduled instant and the start of its run"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(lagBuckets...))
	return &instrumentSet{runs: runs, duration: duration, lag: lag}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

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
// and its reason is folded into the value rather than made a second dimension —
// "skipped_" plus the event's Reason, so an unknown reason still reports
// honestly instead of being folded into a known one.
func statusOf(ev event) string {
	if ev.Skipped {
		return "skipped_" + ev.Reason
	}
	return statusOfRun(ev.Err)
}

// record emits the metrics and the log line for one fire. The log level carries
// the same status: a panic or a failed run at Error, an ordinary fire — and a
// swallowed one, which is routine under multi-replica de-duplication — at Debug.
func record(ev event) {
	status := statusOf(ev)
	ctx := context.Background()
	ins := instruments()

	ins.runs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("job", ev.Name),
		attribute.String("status", status),
	))

	// A skipped fire has no run: no duration and no start, so neither histogram
	// gets a meaningless zero recorded against it.
	if !ev.Skipped {
		ins.duration.Record(ctx, ev.Duration.Seconds(), metric.WithAttributes(
			attribute.String("job", ev.Name),
			attribute.String("status", status),
		))
		if !ev.Start.IsZero() {
			ins.lag.Record(ctx, ev.Start.Sub(ev.Scheduled).Seconds(),
				metric.WithAttributes(attribute.String("job", ev.Name)))
		}
	}

	// The log line carries the same job and status the metrics above just
	// recorded, under the same keys, so the line joins runs{job,status} and
	// run.duration{job,status} instead of only describing them in prose.
	fields := []log.Field{
		log.String("job", ev.Name),
		log.String("status", status),
	}
	switch status {
	case "panic":
		log.Error(ctx, accessTag, append(fields,
			log.Float("duration_ms", ms(ev.Duration)),
			log.Err(ev.Err),
			log.Msg("scheduler: job panicked"))...)
	case "error":
		log.Error(ctx, accessTag, append(fields,
			log.Float("duration_ms", ms(ev.Duration)),
			log.Err(ev.Err),
			log.Msg("scheduler: job failed"))...)
	case "skipped_policy", "skipped_lock":
		log.Debug(ctx, accessTag, func() []log.Field {
			return append(fields,
				log.String("reason", ev.Reason),
				log.Msg("scheduler: job skipped"))
		})
	default:
		log.Debug(ctx, accessTag, func() []log.Field {
			return append(fields,
				log.Float("duration_ms", ms(ev.Duration)),
				log.Msg("scheduler: job ran"))
		})
	}
}

// recordSkip reports a fire that did not run, with reason "policy" or "lock".
func recordSkip(scheduled time.Time, name, reason string) {
	record(event{Name: name, Scheduled: scheduled, Skipped: true, Reason: reason})
}

// traceRun opens the span for one run of t's job and returns the run's
// context. The tracer is the global one — the no-op implementation unless an
// SDK-based provider has been installed — so without otel the span is free.
func traceRun(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer(scope).Start(ctx, "scheduler.job "+name,
		trace.WithAttributes(attribute.String("scheduler.job.name", name)),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
}

// endRun finishes the run's span with the outcome the metrics and log report.
func endRun(span trace.Span, err error) {
	span.SetAttributes(attribute.String("status", statusOfRun(err)))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// ms renders a duration the way the other domain packages' access logs do, so
// the field is comparable across packages.
func ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}
