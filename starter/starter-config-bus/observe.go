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
// Every received event therefore ends in exactly one outcome, and that one value
// drives both the metric and the log line, so the two can never disagree:
//
//	outcome    refreshed | ignored_prefix | malformed | refresh_error
//
// Because the outcome is exclusive, "events received" is the sum over the
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

// The exclusive outcomes of one received broadcast.
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

// Publish outcomes, a separate vocabulary for the sending direction.
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
	attrs := metric.WithAttributes(attribute.String("outcome", outcome))
	b.ins.events.Add(ctx, 1, attrs)

	switch outcome {
	case outcomeRefreshed:
		b.ins.refreshDur.Record(ctx, dur.Seconds(), attrs)
		log.Infof(ctx, starterTag,
			"config bus: refreshed properties on event (prefix=%q origin=%q duration_ms=%.3f)",
			ev.Prefix, ev.Origin, float64(dur.Nanoseconds())/1e6)
	case outcomeRefreshError:
		b.ins.refreshDur.Record(ctx, dur.Seconds(), attrs)
		log.Errorf(ctx, starterTag,
			"config bus: property refresh failed: %v", err)
	case outcomeIgnored:
		log.Debugf(ctx, starterTag,
			"config bus: ignoring refresh event outside watched prefixes (prefix=%q origin=%q watched=%v)",
			ev.Prefix, ev.Origin, b.prefixes)
	case outcomeMalformed:
		log.Warnf(ctx, starterTag, "config bus: ignoring malformed refresh event: %v", err)
	}
}

// recordPublish is the sending-direction sibling of record: one outcome per
// broadcast, driving the counter and a log line at the same level.
func (b *ConfigBus) recordPublish(ctx context.Context, outcome string, dur time.Duration, err error) {
	b.ins.publishes.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
	if err != nil {
		log.Errorf(ctx, starterTag,
			"config bus: publish refresh event failed (subject=%q duration_ms=%.3f): %v",
			b.Config.Subject, float64(dur.Nanoseconds())/1e6, err)
		return
	}
	log.Debugf(ctx, starterTag,
		"config bus: published refresh event (subject=%q duration_ms=%.3f)",
		b.Config.Subject, float64(dur.Nanoseconds())/1e6)
}
