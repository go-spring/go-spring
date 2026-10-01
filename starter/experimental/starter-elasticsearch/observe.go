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

// observe.go is this starter's own elasticsearch instrumentation: a per-operation
// client span, the db.client.* duration/in-flight metrics, and an access
// log riding the log package's native levels. It is deliberately local —
// no shared observer framework — so the emitted vocabulary is all this
// package's own.
package StarterElasticsearch

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// elasticsearchSystem is the value the db.system label carries for this backend.
const elasticsearchSystem = "elasticsearch"

// maxArg bounds the operation argument captured on the access log.
const maxArg = 512

// accessTag is the static log tag for the elasticsearch access log.
var accessTag = log.RegisterAppTag("elasticsearch", "access")

// dbObserver emits the elasticsearch client signals for one instance: the
// db.client.operation.duration histogram, the db.client.active_requests
// gauge, and the access log (no span — the elastic transport
// instrumentation already emits it). Its records go through the process-wide
// instrument set (see [instruments]); only the db.system label is per instance.
type dbObserver struct {
	system string
}

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever meter provider is current then,
// and immutable afterwards. It holds no per-instance state — the db.system /
// db.operation / status labels travel with each record, not here.
type instrumentSet struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

// buildInstruments builds the OTel instruments from whatever meter provider is
// current — resolved on first use, not at package init, so an SDK installed
// later than this package's init still receives the records.
func buildInstruments() *instrumentSet {
	m := otel.Meter("go-spring.org/starter-elasticsearch")
	duration, _ := m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of "+elasticsearchSystem+" client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	active, _ := m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight "+elasticsearchSystem+" client operations"),
		metric.WithUnit("{request}"))
	return &instrumentSet{duration: duration, active: active}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// newDBObserver builds the observer for one instance.
func newDBObserver() *dbObserver {
	return &dbObserver{system: elasticsearchSystem}
}

// Start begins one operation: it bumps the in-flight gauge and records the
// start time (no span — see the comment in the body); the returned span's End records the
// duration histogram, balances the gauge, ends the span, and emits the access
// log. op names the operation (span name, db.operation); arg is the optional
// operation argument (statement, URL path), bounded by maxArg.
func (o *dbObserver) Start(ctx context.Context, op, arg string) (context.Context, *dbSpan) {
	inflight := metric.WithAttributes(
		attribute.String("db.system", o.system),
		attribute.String("db.operation", op),
	)
	instruments().active.Add(ctx, 1, inflight)
	// No span here: elasticsearch's own transport instrumentation (see
	// command.go's newOtelInstrumentation) already emits the client span, so
	// this observer only fills the metric + access-log gap.
	return ctx, &dbSpan{o: o, ctx: ctx, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// dbSpan is the handle returned by dbObserver.Start; End must be called
// exactly once.
type dbSpan struct {
	o        *dbObserver
	ctx      context.Context
	op       string
	arg      string
	start    time.Time
	inflight metric.MeasurementOption
}

// End records the operation's outcome: the duration histogram, the in-flight
// gauge balance, the span (with err, if any), and the access log — an error
// at Warn, a success carrying an operation argument at Debug, a plain
// success at Info.
func (s *dbSpan) End(err error) {
	o := s.o
	dur := time.Since(s.start)
	status := "ok"
	if err != nil {
		status = "error"
	}
	instruments().duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("db.system", o.system),
		attribute.String("db.operation", s.op),
		attribute.String("status", status),
	))
	instruments().active.Add(s.ctx, -1, s.inflight)

	common := func() []log.Field {
		fields := []log.Field{
			log.String("db.operation", s.op),
			log.String("status", status),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
		}
		if s.arg != "" {
			fields = append(fields, log.String("db.statement", strutil.Truncate(s.arg, maxArg)))
		}
		return fields
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(common(), log.Err(err))...)
	case s.arg != "":
		log.Debug(s.ctx, accessTag, common)
	default:
		log.Info(s.ctx, accessTag, common()...)
	}
}
