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
	"go-spring.org/stdlib/strutil"
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// kafkaSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const kafkaSystem = "kafka"

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set, shared with the other family members.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// accessTag is the static log tag for the kafka access log.
var accessTag = log.RegisterAppTag("kafka", "access")

var (
	// instruments are resolved on first use rather than at package init, so a
	// SDK installed later (starter-otel) still receives the records.
	instOnce sync.Once

	opDuration metric.Float64Histogram
	activeReqs metric.Int64UpDownCounter
)

func instruments() {
	instOnce.Do(func() {
		m := otel.Meter("go-spring.org/starter-kafka")
		opDuration, _ = m.Float64Histogram("messaging.client.operation.duration",
			metric.WithDescription("Duration of kafka client operations"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(durationBuckets...))
		activeReqs, _ = m.Int64UpDownCounter("messaging.client.active_requests",
			metric.WithDescription("Number of in-flight kafka client operations"),
			metric.WithUnit("{request}"))
	})
}

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
	instruments()
	inflight := inflightOf(op)
	activeReqs.Add(ctx, 1, inflight)
	return &accessRecord{ctx: ctx, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// End records the operation's duration, balances the in-flight gauge, and
// emits the access record. The log level carries the outcome: an error at
// Warn, a success with a destination at Debug (the per-message record is
// frequent and uninteresting until it fails), a success without a destination
// at Info.
func (s *accessRecord) End(err error) {
	status := statusOf(err)
	opDuration.Record(s.ctx, time.Since(s.start).Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", kafkaSystem),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	))
	activeReqs.Add(s.ctx, -1, s.inflight)

	// Log keys are the metric labels' names, so a dashboard selecting failed
	// operations lands on the lines that explain them.
	fields := func() []log.Field {
		f := []log.Field{
			log.String("messaging.operation", s.op),
			log.String("status", status),
			log.Float("duration_ms", float64(time.Since(s.start).Nanoseconds())/1e6),
		}
		if s.arg != "" {
			f = append(f, log.String("messaging.destination.name", strutil.Truncate(s.arg, 512)))
		}
		return f
	}
	if err != nil {
		log.Warn(s.ctx, accessTag, append(fields(), log.Any("error", err))...)
		return
	}
	if s.arg != "" {
		log.Debug(s.ctx, accessTag, fields)
		return
	}
	log.Info(s.ctx, accessTag, fields()...)
}
