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

package StarterConfigBus

import (
	"context"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Observability contract.
//
// A broadcast is fire-and-forget: core NATS gives the publisher no acknowledgement,
// so nothing but the subscriber can report whether a refresh actually happened.
// Every received event therefore ends in exactly one status, and that one value
// drives both the metric and the log line, so the two can never disagree:
//
//	outcome    refreshed | ignored_prefix | malformed | refresh_error
//
// Because the status is exclusive, "events received" is the sum over the
// dimension — there is no separate received counter that could double-count.
// The publisher's side is a different direction with a different vocabulary, so
// it gets its own counter rather than muddying this one.
//
// The interesting failure is refresh_error: the signal arrived and was honored,
// but the property refresh itself failed, leaving the instance on stale
// configuration. That is the case a log-only bus cannot be alerted on.
//
// Spans are not here: Conn.Consume (starter-nats) already wraps the callback in
// a consumer span linked to the publisher's span, so a business span would only
// add a redundant child.

// instrumentationName is the OTel instrumentation scope; the module path keeps
// it unique across starters.
const instrumentationName = "go-spring.org/starter-config-bus"

// The exclusive statuses of one received broadcast.
const (
	// outcomeRefreshed — the event applied and the property refresh succeeded.
	outcomeRefreshed = "refreshed"
	// outcomeIgnored — the event was outside this instance's watched prefixes.
	outcomeIgnored = "ignored_prefix"
	// outcomeMalformed — the payload was not a valid RefreshEvent.
	outcomeMalformed = "malformed"
	// outcomeRefreshError — the refresh was attempted and failed.
	outcomeRefreshError = "refresh_error"
)

// Publish statuses, a separate vocabulary for the sending direction.
const (
	outcomePublishOK    = "ok"
	outcomePublishError = "error"
)

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set, shared with the other domain packages.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// instruments bundles the metrics the bus records. They are built at wiring time
// (see Init), not at package init, so an SDK installed later than this package's
// init still receives the records.
type instruments struct {
	events     metric.Int64Counter
	refreshDur metric.Float64Histogram
	publishes  metric.Int64Counter
}

func newInstruments() instruments {
	m := otel.Meter(instrumentationName)
	events, _ := m.Int64Counter("config.bus.events",
		metric.WithDescription("Configuration refresh broadcasts received, by outcome"),
		metric.WithUnit("{event}"))
	refreshDur, _ := m.Float64Histogram("config.bus.refresh.duration",
		metric.WithDescription("Duration of the property refresh triggered by a broadcast"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	publishes, _ := m.Int64Counter("config.bus.publishes",
		metric.WithDescription("Configuration refresh broadcasts published, by outcome"),
		metric.WithUnit("{event}"))
	return instruments{events: events, refreshDur: refreshDur, publishes: publishes}
}

// record is the single sink for one received event: it counts the outcome,
// records the refresh duration when a refresh was actually attempted, and emits
// one log line whose level is the outcome. dur is ignored for outcomes that
// never reached a refresh, so no meaningless zero is recorded against the
// histogram.
func (b *ConfigBus) record(ctx context.Context, outcome string, ev RefreshEvent, dur time.Duration, err error) {
	attrs := metric.WithAttributes(attribute.String("status", outcome))
	b.ins.events.Add(ctx, 1, attrs)

	// The log line carries the same outcome the counter just counted, under the
	// same key, so the two can never be read as disagreeing — and a dashboard
	// selecting config.bus.events{outcome=...} lands on the line explaining it.
	switch outcome {
	case outcomeRefreshed:
		b.ins.refreshDur.Record(ctx, dur.Seconds(), attrs)
		log.Info(ctx, starterTag, append(eventFields(outcome),
			log.String("prefix", ev.Prefix),
			log.String("origin", ev.Origin),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
			log.Msg("config bus: refreshed properties on event"))...)
	case outcomeRefreshError:
		b.ins.refreshDur.Record(ctx, dur.Seconds(), attrs)
		log.Error(ctx, starterTag, append(eventFields(outcome),
			log.Any("error", err),
			log.Msg("config bus: property refresh failed"))...)
	case outcomeIgnored:
		log.Debug(ctx, starterTag, func() []log.Field {
			return append(eventFields(outcome),
				log.String("prefix", ev.Prefix),
				log.String("origin", ev.Origin),
				log.Any("watched", b.prefixes),
				log.Msg("config bus: ignoring refresh event outside watched prefixes"))
		})
	case outcomeMalformed:
		log.Warn(ctx, starterTag, append(eventFields(outcome),
			log.Any("error", err),
			log.Msg("config bus: ignoring malformed refresh event"))...)
	}
}

// eventFields returns the fields every bus log line carries: the outcome under
// the key the counters attribute it with. It lives in one place for the same
// reason the metric's own keys do — a key spelled out at each call site drifts,
// and a drifted key is silent: the line still looks right and joins nothing.
func eventFields(outcome string) []log.Field {
	return []log.Field{log.String("status", outcome)}
}

// recordPublish is the sending-direction sibling of record: one outcome per
// broadcast, driving the counter and a log line at the same level.
func (b *ConfigBus) recordPublish(ctx context.Context, outcome string, dur time.Duration, err error) {
	b.ins.publishes.Add(ctx, 1, metric.WithAttributes(attribute.String("status", outcome)))
	if err != nil {
		log.Error(ctx, starterTag, append(eventFields(outcome),
			log.String("subject", b.Config.Subject),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
			log.Any("error", err),
			log.Msg("config bus: publish refresh event failed"))...)
		return
	}
	log.Debug(ctx, starterTag, func() []log.Field {
		return append(eventFields(outcome),
			log.String("subject", b.Config.Subject),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
			log.Msg("config bus: published refresh event"))
	})
}
