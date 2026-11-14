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

// observe.go is the session layer's observability surface: the event it
// reports, the [Observer] seam, and the default OTel-backed adapter.

package session

import (
	"context"

	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter in this
// package reports under.
const componentName = "go-spring.org/cloud/experimental/session"

// Observer receives the session layer's observability events. A nil Observer
// disables observation; implementations must not block.
type Observer interface {
	// Operation records one session-store operation. operation is
	// load|save|delete and status is ok|missed|error.
	Operation(ctx context.Context, operation, status string)
}

// NewOTelObserver resolves the layer's instrument and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean.
func NewOTelObserver() (Observer, error) {
	total, err := otel.Meter(componentName).
		Int64Counter("session.operation.total",
			metric.WithDescription("Session store operations, by operation and status"),
			metric.WithUnit("{operation}"))
	if err != nil {
		return nil, errutil.Explain(err, "session: register session.operation.total")
	}
	return &otelObserver{total: total}, nil
}

// otelObserver adapts the layer's events to the OTel instrument.
type otelObserver struct {
	total metric.Int64Counter
}

// Operation implements [Observer].
func (o *otelObserver) Operation(ctx context.Context, operation, status string) {
	o.total.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
	))
}
