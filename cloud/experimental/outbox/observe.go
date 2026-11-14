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

// observe.go is the outbox relay's metric surface: the per-record event, the
// [MetricsObserver] seam, and the default OTel-backed adapter. The lifecycle
// [Observer] (publish/retry/dead callbacks) lives in outbox.go; this file covers
// the signal side.

package outbox

import (
	"context"

	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter in this
// package reports under.
const componentName = "go-spring.org/cloud/experimental/outbox"

// MetricsObserver receives the relay's per-record outcome event. status is
// published | retry | dead; summed over status it is the number of relay
// decisions taken. A nil observer disables the metric.
type MetricsObserver interface {
	Record(ctx context.Context, status string)
}

// NewOTelObserver resolves the layer's instrument and returns the default
// OTel-backed [MetricsObserver]. It is registered as the process's default bean.
func NewOTelObserver() (MetricsObserver, error) {
	delivered, err := otel.Meter(componentName).
		Int64Counter("outbox.record.total",
			metric.WithDescription("Outbox records relayed, by outcome"),
			metric.WithUnit("{record}"))
	if err != nil {
		return nil, errutil.Explain(err, "outbox: register outbox.record.total")
	}
	return &otelObserver{delivered: delivered}, nil
}

// otelObserver adapts the relay's events to the OTel instrument.
type otelObserver struct {
	delivered metric.Int64Counter
}

// Record implements [MetricsObserver].
func (o *otelObserver) Record(ctx context.Context, status string) {
	o.delivered.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
}
