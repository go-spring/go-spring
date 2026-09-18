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

// Package confrefresh is the metrics funnel for property-refresh triggers.
// Every config backend (nacos, etcd, consul, vault, k8s, file, ...) fires the
// same application-wide refresh when its watch reports a change; without a
// shared funnel, "was the fleet actually refreshed, and did it fail" is
// answerable only by grepping per-module logs.
//
// The package deliberately wraps instead of referencing the refresh function:
// callers pass their own (typically gs.RefreshProperties), so this package
// stays spring-free and the cloud layer's dependency direction is preserved.
//
//	Run(gs.RefreshProperties)
//
// Logs are NOT recorded here on purpose: callers already log failures with
// module-specific context (source paths, dataIds). Metrics are the part that
// needs exactly one funnel; logs belong to the module that knows the subject.
package confrefresh

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Status attribute values. The statuses are exclusive, so
// config.refresh.total summed over status is the number of refreshes
// triggered — there is no separate counter that could double-count.
// "status" is the name this axis carries everywhere in go-spring (the db /
// messaging / http families, discovery and lock); it was "outcome" here.
const (
	statusOK    = "ok"
	statusError = "error"
)

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// instruments bundles the metrics. Built lazily on the first Run, not at
// package init, so an SDK installed after this package's init (starter-otel
// installs its provider during wiring) still receives the records.
type instruments struct {
	total       metric.Int64Counter
	duration    metric.Float64Histogram
	lastSuccess metric.Float64Gauge
}

var (
	insOnce sync.Once
	ins     instruments
)

func newInstruments() instruments {
	m := otel.Meter("go-spring.org/cloud/confrefresh")
	total, _ := m.Int64Counter("config.refresh.total",
		metric.WithDescription("Property refreshes triggered, by status"),
		metric.WithUnit("{refresh}"))
	duration, _ := m.Float64Histogram("config.refresh.duration",
		metric.WithDescription("Duration of a triggered property refresh"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	lastSuccess, _ := m.Float64Gauge("config.refresh.last_success",
		metric.WithDescription("Unix time of the last successful property refresh; stalls are visible as a flat line"),
		metric.WithUnit("s"))
	return instruments{total: total, duration: duration, lastSuccess: lastSuccess}
}

// Run executes fn once and records the status: one exclusive value on
// config.refresh.total, the elapsed time on config.refresh.duration, and the
// wall-clock time on config.refresh.last_success when fn succeeded. fn's error
// is returned unchanged — this is instrumentation, not error policy.
func Run(fn func() error) error {
	insOnce.Do(func() { ins = newInstruments() })
	start := time.Now()
	err := fn()
	elapsed := time.Since(start)

	ctx := context.Background()
	status := statusOK
	if err != nil {
		status = statusError
	}
	ins.total.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	ins.duration.Record(ctx, elapsed.Seconds())
	if err == nil {
		ins.lastSuccess.Record(ctx, float64(time.Now().Unix()))
	}
	return err
}
