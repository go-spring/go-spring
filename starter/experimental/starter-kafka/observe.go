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

// observe.go is the per-message observability of this starter: the access log
// plus the messaging family's operation instruments.
//
// Spans come from kotel (wired in driver.go), so this layer opens none — one
// emitted here would duplicate kotel's, and kotel's carry the family's span
// attributes already (semconv messaging.system / messaging.operation /
// messaging.destination.name).
//
// The operation metrics are NOT kotel's. kotel emits client/broker health
// under implementation-named series (messaging.kafka.connects.count,
// messaging.kafka.write_bytes, ... per node); none of them says how long one
// produce or consume took, or how many are in flight. Those two are what the
// messaging family shares, so this layer provides them.
package StarterKafka

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

// kafkaSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const kafkaSystem = "kafka"

// accessTag is the static log tag for the kafka access log — the messaging
// family's shared tag, so one filter greps every broker's records.
var accessTag = log.RegisterAppTag("messaging", "access")

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever providers are current then.
type instrumentSet struct {
	opTotal    metric.Int64Counter
	opDuration metric.Float64Histogram
	activeReqs metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before any bean is built, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

// buildInstruments builds the messaging.operation.* instruments — the same
// names, attributes and status words cloud/messaging's Observe decorator uses —
// so kafka's client-hook instrumentation (which cannot wrap in Observe without
// double-counting every record) still lands on the family's dashboards.
func buildInstruments() *instrumentSet {
	m := otel.Meter("go-spring.org/starter-kafka")
	in := &instrumentSet{}
	in.opTotal, _ = m.Int64Counter("messaging.operation.total",
		metric.WithDescription("Messages published and consumed, by operation and status"),
		metric.WithUnit("{message}"))
	in.opDuration, _ = m.Float64Histogram("messaging.operation.duration",
		metric.WithDescription("Duration of a publish or a consume"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	in.activeReqs, _ = m.Int64UpDownCounter("messaging.operation.active",
		metric.WithDescription("Number of in-flight messaging operations"),
		metric.WithUnit("{operation}"))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// inflightOf names the in-flight gauge's dimensions. The +1 taken when an
// operation starts and the -1 taken when it ends must carry identical
// attributes, or the gauge never balances — so both go through here.
func inflightOf(op string) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("messaging.system", kafkaSystem),
		attribute.String("messaging.operation", op),
	)
}

// statusOf names the outcome the way the family's metric label and log field
// expect — the same two words the other members use.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// accessRecord is one in-flight access-log record, opened by startAccess and
// closed by exactly one End, which emits the log line carrying the measured
// duration.
type accessRecord struct {
	ctx      context.Context
	op       string
	arg      string
	start    time.Time
	inflight metric.MeasurementOption
}

// startAccess opens an access-log record for op (e.g. "publish", "consume");
// arg is the destination topic, captured in the log when non-empty.
func startAccess(ctx context.Context, op, arg string) *accessRecord {
	inflight := inflightOf(op)
	instruments().activeReqs.Add(ctx, 1, inflight)
	return &accessRecord{ctx: ctx, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// End records the operation's duration, balances the in-flight gauge, and
// emits the access record. The log level carries the outcome: an error at
// Warn, a success with a destination at Debug (the per-message record is
// frequent and uninteresting until it fails), a success without a destination
// at Info.
func (s *accessRecord) End(err error) {
	status := statusOf(err)
	dur := time.Since(s.start)
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", kafkaSystem),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	)
	ins := instruments()
	ins.opTotal.Add(s.ctx, 1, attrs)
	ins.opDuration.Record(s.ctx, dur.Seconds(), attrs)
	ins.activeReqs.Add(s.ctx, -1, s.inflight)

	// Log keys are the metric labels' names, so a dashboard selecting failed
	// operations lands on the lines that explain them.
	fields := func() []log.Field {
		f := []log.Field{
			log.String("messaging.operation", s.op),
			log.String("status", status),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
		}
		if s.arg != "" {
			f = append(f, log.String("messaging.destination.name", strutil.Truncate(s.arg, 512)))
		}
		return f
	}
	if err != nil {
		log.Warn(s.ctx, accessTag, append(fields(), log.Err(err))...)
		return
	}
	if s.arg != "" {
		log.Debug(s.ctx, accessTag, fields)
		return
	}
	log.Info(s.ctx, accessTag, fields()...)
}
