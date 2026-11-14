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

// observe.go is the batch layer's observability surface: the event it reports,
// the [Observer] seam, and the default OTel-backed adapter.

package batch

import (
	"context"
	"time"

	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation component name this package reports
// under.
const componentName = "go-spring.org/cloud/experimental/batch"

// Observer receives the batch layer's observability events. A nil Observer
// disables observation; implementations must not block.
type Observer interface {
	// JobRun records one job run that reached a terminal status.
	JobRun(ctx context.Context, job, status string, elapsed time.Duration)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer].
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.total, err = m.Int64Counter("batch.job.total",
		metric.WithDescription("Batch job runs reaching a terminal state, by status"),
		metric.WithUnit("{run}"))
	if err != nil {
		return nil, errutil.Explain(err, "batch: register batch.job.total")
	}
	o.duration, err = m.Float64Histogram("batch.job.duration",
		metric.WithDescription("Duration of one batch job run"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, errutil.Explain(err, "batch: register batch.job.duration")
	}
	return o, nil
}

// otelObserver adapts the layer's events to the OTel instruments. The
// instruments are immutable once resolved at construction.
type otelObserver struct {
	total    metric.Int64Counter
	duration metric.Float64Histogram
}

// JobRun implements [Observer].
func (o *otelObserver) JobRun(ctx context.Context, job, status string, elapsed time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("job", job),
		attribute.String("status", status),
	)
	o.total.Add(ctx, 1, attrs)
	o.duration.Record(ctx, elapsed.Seconds(), attrs)
}
