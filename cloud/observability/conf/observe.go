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

// observe.go is the config-refresh funnel's observability surface: the event it
// reports, the [Observer] seam, and the default OTel-backed adapter.

package conf

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter in this
// package reports under.
const componentName = "go-spring.org/cloud/observability/conf"

// Observer receives the refresh funnel's observability events. A nil Observer
// disables observation; implementations must not block.
type Observer interface {
	// Refresh records one triggered property refresh. status is ok | error and
	// elapsed is the refresh's measured duration.
	Refresh(ctx context.Context, status string, elapsed time.Duration)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer].
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.total, err = m.Int64Counter("config.refresh.total",
		metric.WithDescription("Property refreshes triggered, by status"),
		metric.WithUnit("{refresh}"))
	if err != nil {
		return nil, errutil.Explain(err, "conf: register config.refresh.total")
	}
	o.duration, err = m.Float64Histogram("config.refresh.duration",
		metric.WithDescription("Duration of a triggered property refresh"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "conf: register config.refresh.duration")
	}
	o.lastSuccess, err = m.Float64Gauge("config.refresh.last_success_timestamp",
		metric.WithDescription("Unix time of the last successful property refresh"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, errutil.Explain(err, "conf: register config.refresh.last_success_timestamp")
	}
	return o, nil
}

// otelObserver adapts the funnel's events to the OTel instruments, resolved
// when the observer is constructed.
type otelObserver struct {
	total       metric.Int64Counter
	duration    metric.Float64Histogram
	lastSuccess metric.Float64Gauge
}

// Refresh implements [Observer].
func (o *otelObserver) Refresh(ctx context.Context, status string, elapsed time.Duration) {
	o.total.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	o.duration.Record(ctx, elapsed.Seconds())
	if status == statusOK {
		o.lastSuccess.Record(ctx, float64(time.Now().Unix()))
	}
}

// defaultObserver is the process-wide observer [Refresh] uses. It is lazy so it
// binds to whichever OTel provider is current at first use; a registration
// failure is surfaced once and observation degrades to a no-op.
var defaultObserver = sync.OnceValue(defaultObserverBuilder)

// defaultObserverBuilder is the once-body, named so resetObservers can
// reinstall it.
func defaultObserverBuilder() Observer {
	obs, err := NewOTelObserver()
	if err != nil {
		instrument.ReportInstrumentError("config.refresh", err)
		return nil
	}
	return obs
}

// resetObservers makes the next use of defaultObserver resolve a fresh
// observer. It exists for tests that install their own MeterProvider.
func resetObservers() { defaultObserver = sync.OnceValue(defaultObserverBuilder) }
