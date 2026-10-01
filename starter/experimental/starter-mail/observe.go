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

// observe.go is the module-local observer for the Mailer: one duration metric,
// an in-flight gauge and an access log per send. go-mail offers no hook, so a
// send can only be observed here, at the wrapper — which is why the observation
// lives in [Mailer.Send] and not in the caller-side span helpers.
//
// The vocabulary is its own: email is not messaging (there is no broker, no
// destination, no consumer side), so the labels are email.* rather than
// messaging.*. The result axis and duration key are the ecosystem-wide ones
// (status, duration_ms), so a mail failure still joins the same query shape as
// every other client.
package StarterMail

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-mail"

// emailSystem is the value the email.system label carries — the capability's
// name, not a per-instance choice.
const emailSystem = "smtp"

// accessTag is the static log tag for the mail access log.
var accessTag = log.RegisterAppTag("mail", "access")

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever meter provider is current then,
// and immutable afterwards. It holds no per-mailer state — the email.system /
// status labels travel with each record, not here.
type instrumentSet struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

// buildInstruments builds the instruments from whatever meter provider is
// current — resolved on first use, not at package init, so an SDK installed
// later than this package's init still receives the records.
func buildInstruments() *instrumentSet {
	m := otel.GetMeterProvider().Meter(meterName)
	duration, _ := m.Float64Histogram(
		"email.client.operation.duration",
		metric.WithDescription("Duration of mail sends"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...),
	)
	active, _ := m.Int64UpDownCounter(
		"email.client.active_requests",
		metric.WithDescription("Number of mail sends currently in flight"),
		metric.WithUnit("{request}"),
	)
	return &instrumentSet{duration: duration, active: active}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// start bumps the in-flight gauge and returns the value to hand back to [done],
// plus the instant the send began.
func (o *instrumentSet) start(ctx context.Context) (metric.MeasurementOption, time.Time) {
	inflight := metric.WithAttributes(attribute.String("email.system", emailSystem))
	o.active.Add(ctx, 1, inflight)
	return inflight, time.Now()
}

// done records the duration and the access log, and balances the gauge.
//
// status is the ecosystem-wide result axis (ok|error) and duration_ms is the
// ecosystem-wide duration key, so a failing send joins the same queries as
// every other client — while email.system keeps the line identifiable as mail.
func (o *instrumentSet) done(ctx context.Context, inflight metric.MeasurementOption, start time.Time, err error) {
	dur := time.Since(start)
	status := statusOf(err)
	o.active.Add(ctx, -1, inflight)
	o.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("email.system", emailSystem),
		attribute.String("status", status),
	))

	fields := []log.Field{
		log.String("email.system", emailSystem),
		log.String("status", status),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if err != nil {
		log.Warn(ctx, accessTag, append(fields, log.Err(err))...)
		return
	}
	log.Info(ctx, accessTag, fields...)
}

// statusOf names the outcome the way the rest of the ecosystem does.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
