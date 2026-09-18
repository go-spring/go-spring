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

package StarterScheduler

import (
	"context"
	"errors"
	"time"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Observability contract.
//
// Every fire — whether it ran or was swallowed — is reported to the scheduling
// observer, and this is where that report becomes a metric and a log line. One
// status value drives both, so the metric can never disagree with the log:
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
// Spans are not here: they wrap the run itself (see instrument in starter.go),
// because a swallowed fire has no run to trace.

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set, shared with the other domain packages.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// lagBuckets are the wake-up-lag histogram boundaries (seconds). Lag is the
// delay between a fire's scheduled instant and when its run actually started, so
// the interesting range is sub-millisecond to seconds — finer than
// durationBuckets, which starts at 5ms.
var lagBuckets = []float64{0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}

// accessTag is the static log tag for the per-fire log lines. The lifecycle
// lines (starting / started / drain) stay on the default tag: only the
// one-line-per-fire records are the access log, and only they need a tag of
// their own to be selectable apart from application logs.
var accessTag = log.RegisterAppTag("scheduler", "access")

// instruments bundles the metrics the observer records. They are built at
// wiring time (see Server.Run), not at package init, so an SDK installed later
// than this package's init still receives the records.
type instruments struct {
	runs     metric.Int64Counter
	duration metric.Float64Histogram
	lag      metric.Float64Histogram
}

func newInstruments() instruments {
	m := otel.Meter(instrumentationName)
	runs, _ := m.Int64Counter("scheduling.runs",
		metric.WithDescription("Scheduled job fires by status"),
		metric.WithUnit("{fire}"))
	duration, _ := m.Float64Histogram("scheduling.run.duration",
		metric.WithDescription("Duration of a job run"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	// A rising lag is the scheduler's own health signal: the schedule is not
	// drifting (the next fire stays anchored on the planned instant), but runs are
	// starting later and later, which points at a process that is saturated or
	// stalled.
	lag, _ := m.Float64Histogram("scheduling.lag",
		metric.WithDescription("Delay between a fire's scheduled instant and the start of its run"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(lagBuckets...))
	return instruments{runs: runs, duration: duration, lag: lag}
}

// statusOfRun classifies a finished run. A panic is reported as an error
// wrapping [scheduling.ErrJobPanicked], which is what makes it separable from a
// run that merely returned an error.
func statusOfRun(err error) string {
	switch {
	case errors.Is(err, scheduling.ErrJobPanicked):
		return "panic"
	case err != nil:
		return "error"
	default:
		return "ok"
	}
}

// statusOf classifies one fire. A skipped fire is a whole status of its own,
// and its reason is folded into the value rather than made a second dimension —
// "skipped_" plus [scheduling.Event.Reason], so an unknown reason still reports
// honestly instead of being folded into a known one.
func statusOf(ev scheduling.Event) string {
	if ev.Skipped {
		return "skipped_" + ev.Reason
	}
	return statusOfRun(ev.Err)
}

// record emits the metrics and the log line for one fire. The log level carries
// the same status: a panic or a failed run at Error, an ordinary fire — and a
// swallowed one, which is routine under multi-replica de-duplication — at Debug.
func (s *Server) record(ev scheduling.Event) {
	status := statusOf(ev)
	ctx := context.Background()

	s.instruments.runs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("job", ev.Name),
		attribute.String("status", status),
	))

	// A skipped fire has no run: no duration and no start, so neither histogram
	// gets a meaningless zero recorded against it.
	if !ev.Skipped {
		s.instruments.duration.Record(ctx, ev.Duration.Seconds(), metric.WithAttributes(
			attribute.String("job", ev.Name),
			attribute.String("status", status),
		))
		if !ev.Start.IsZero() {
			s.instruments.lag.Record(ctx, ev.Start.Sub(ev.Scheduled).Seconds(),
				metric.WithAttributes(attribute.String("job", ev.Name)))
		}
	}

	// The log line carries the same job and status the metrics above just
	// recorded, under the same keys, so the line joins runs{job,status} and
	// run.duration{job,status} instead of only describing them in prose.
	switch status {
	case "panic":
		log.Error(ctx, accessTag, append(runFields(ev, status),
			log.Float("duration_ms", ms(ev.Duration)),
			log.Any("error", ev.Err),
			log.Msg("scheduler: job panicked"))...)
	case "error":
		log.Error(ctx, accessTag, append(runFields(ev, status),
			log.Float("duration_ms", ms(ev.Duration)),
			log.Any("error", ev.Err),
			log.Msg("scheduler: job failed"))...)
	case "skipped_policy", "skipped_lock":
		log.Debug(ctx, accessTag, func() []log.Field {
			return append(runFields(ev, status),
				log.String("reason", ev.Reason),
				log.Msg("scheduler: job skipped"))
		})
	default:
		log.Debug(ctx, accessTag, func() []log.Field {
			return append(runFields(ev, status),
				log.Float("duration_ms", ms(ev.Duration)),
				log.Msg("scheduler: job ran"))
		})
	}
}

// runFields returns the fields one fire's log line carries: the keys are the
// metric attribute names, so the line and the counters for the same fire can
// never be read as describing different things.
func runFields(ev scheduling.Event, status string) []log.Field {
	return []log.Field{
		log.String("job", ev.Name),
		log.String("status", status),
	}
}

// ms renders a duration the way the other domain packages' access logs do, so
// the field is comparable across starters.
func ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}
