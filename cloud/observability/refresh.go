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

package observability

import (
	"context"
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var configTag = log.RegisterAppTag("config", "")

// Status attribute values. The statuses are exclusive, so
// config.refresh.total summed over status is the number of refreshes
// triggered — there is no separate counter that could double-count.
const (
	statusOK    = "ok"
	statusError = "error"
)

// instrumentSet bundles the metrics a refresh records: one per process, resolved
// lazily on first use so the set binds to whichever OTel global provider is
// current then (starter-otel wires it during RefreshPrepare, after all package
// inits), and immutable afterwards. It holds no per-refresh state.
type instrumentSet struct {
	total       metric.Int64Counter
	duration    metric.Float64Histogram
	lastSuccess metric.Float64Gauge
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter("go-spring.org/cloud/observability")
	in := &instrumentSet{}
	in.total, _ = m.Int64Counter("config.refresh.total",
		metric.WithDescription("Property refreshes triggered, by status"),
		metric.WithUnit("{refresh}"))
	in.duration, _ = m.Float64Histogram("config.refresh.duration",
		metric.WithDescription("Duration of a triggered property refresh"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(DurationBuckets()...))
	in.lastSuccess, _ = m.Float64Gauge("config.refresh.last_success_timestamp",
		metric.WithDescription("Unix time of the last successful property refresh"),
		metric.WithUnit("s"))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// RefreshConf is the shared funnel for property-refresh triggers. Every config
// backend (nacos, etcd, consul, vault, k8s, file, ...) fires the same
// application-wide refresh when its watch reports a change; without a shared
// funnel, "was the fleet actually refreshed, and did it fail" is answerable
// only by grepping per-module logs.
//
// It wraps the refresh function instead of referencing it: callers pass their
// own (typically gs.RefreshProperties), so this package stays spring-free.
//
//	observability.RefreshConf(ctx, gs.RefreshProperties)
//
// Logs are recorded here too, centrally: a refresh is a rare, fleet-wide
// event, so the funnel is also the natural place for its log line. Callers
// log only their backend events (key deleted, watch retrying, poll failed) —
// the refresh outcome itself is this function's to report.
//
// RefreshConf executes fn once and records the status: one exclusive value on
// config.refresh.total, the elapsed time on config.refresh.duration, and the
// wall-clock time on config.refresh.last_success_timestamp when fn succeeded
// (refresh stalls show up as that gauge going flat; alert on
// time() - last_success_timestamp exceeding a threshold). fn's error is
// returned unchanged — this is instrumentation, not error policy.
//
// ctx is the refresh's context: it flows to the metric records (available
// for exemplar sampling) and into fn. Refresh events originate from backend
// watch callbacks (k8s informer handlers, SDK listeners) that often carry no
// context, so callers without one pass context.Background() at the boundary.
func RefreshConf(ctx context.Context, fn func(context.Context) error) error {
	in := instruments()

	start := time.Now()
	err := fn(ctx)
	elapsed := time.Since(start)

	status := statusOK
	if err != nil {
		status = statusError
	}
	in.total.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	in.duration.Record(ctx, elapsed.Seconds())
	if err == nil {
		in.lastSuccess.Record(ctx, float64(time.Now().Unix()))
	}

	// One status value drives both the metrics and the log, so they can never
	// disagree. The log level carries the outcome: a failed refresh at Warn
	// (the previous snapshot is retained, so it is degraded, not broken), a
	// successful one at Info (refreshes are rare and always worth a line).
	fields := []log.Field{
		log.String("status", status),
		log.Float("duration_ms", float64(elapsed.Nanoseconds())/1e6),
	}
	if err != nil {
		fields = append(fields, log.Err(err), log.Msg("property refresh failed"))
		log.Warn(ctx, configTag, fields...)
	} else {
		fields = append(fields, log.Msg("property refresh completed"))
		log.Info(ctx, configTag, fields...)
	}
	return err
}
