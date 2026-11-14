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

// observe.go is the connection pool's observability surface: the acquire/idle
// events, the [Observer] seam, and the default OTel-backed adapter.

package connpool

import (
	"context"

	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation component name this package reports
// under.
const componentName = "go-spring.org/cloud/connpool"

// Observer receives the pool's observability events. A nil Observer disables
// observation; implementations must not block.
type Observer interface {
	// Acquire records one acquire attempt. outcome is hit (idle reused),
	// dialed, exhausted or dial_failed.
	Acquire(ctx context.Context, outcome string)

	// Idle moves the number of connections parked idle: +1 when one is parked,
	// -1 when one is taken or reaped.
	Idle(ctx context.Context, delta int64)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.acquire, err = m.Int64Counter("connpool.acquire.total",
		metric.WithDescription("Connection acquires by outcome: hit (idle reused), dialed, exhausted, failed"),
		metric.WithUnit("{acquire}"))
	if err != nil {
		return nil, errutil.Explain(err, "connpool: register connpool.acquire.total")
	}
	o.idle, err = m.Int64UpDownCounter("connpool.conn.idle",
		metric.WithDescription("Connections currently parked idle in the pool"),
		metric.WithUnit("{connection}"))
	if err != nil {
		return nil, errutil.Explain(err, "connpool: register connpool.conn.idle")
	}
	return o, nil
}

// otelObserver adapts the pool's events to the OTel instruments, resolved when
// the pool is constructed.
type otelObserver struct {
	acquire metric.Int64Counter
	idle    metric.Int64UpDownCounter
}

// Acquire implements [Observer].
func (o *otelObserver) Acquire(ctx context.Context, outcome string) {
	o.acquire.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// Idle implements [Observer].
func (o *otelObserver) Idle(ctx context.Context, delta int64) {
	o.idle.Add(ctx, delta)
}
