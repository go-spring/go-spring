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

// observe.go is the load-balancing layer's OBSERVABILITY surface: the events the
// layer reports, the [Observer] seam an implementation adapts them through, and
// the default OTel-backed adapter. It is deliberately its own file, so the
// layer's emission vocabulary stays in one place rather than being scattered
// through the tracker's state machine.

package loadbalance

import (
	"context"

	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation component every meter in this package
// reports under.
const componentName = "go-spring.org/cloud/governance/dataplane/outbound/loadbalance"

// Observer receives the load-balancing layer's observability events. It is the
// seam: the tracker reports what happened in its own vocabulary, and an
// implementation decides how to record it — the default is OTel (see
// [NewOTelObserver]), and an application may replace it with its own bean.
//
// Observation is off when the tracker is given a no-op implementation, which
// this package supplies internally as [emptyObserver]. Implementations must not
// block: the events are emitted synchronously from the tracker's state
// transitions.
type Observer interface {
	// Suspended reports one endpoint suspension for service; reason is the
	// outcome class that tripped the threshold.
	Suspended(ctx context.Context, service, reason string)

	// SuspendedDelta moves the number of endpoints currently suspended for
	// service: +1 when one is suspended, -1 when it recovers, is removed as stale, or its
	// generation is closed.
	SuspendedDelta(ctx context.Context, service string, delta int64)
}

// emptyObserver is the [Observer] the manager installs when a tracker should
// emit nothing: every report is a no-op, so the report path needs no nil
// branch. It is unexported because "off" is the manager's decision, not an API
// a caller composes with.
type emptyObserver struct{}

func (emptyObserver) Suspended(context.Context, string, string)     {}
func (emptyObserver) SuspendedDelta(context.Context, string, int64) {}

// otelObserver adapts the layer's events to the OTel instruments. ONE observer
// is built per process and the Manager injects it into every tracker; a tracker
// constructed standalone resolves its own default observer, and OTel's
// name-based lookup makes both point at the same two instruments.
//
// The observer holds no per-tracker state — the endpoint address is never a
// metric attribute, so no per-endpoint value travels here. A tracker given
// [emptyObserver] is the single spelling of "observation off".
//
// The suspension of an endpoint is an event (a dedicated counter named after
// the event, attributed by service), and the number of currently suspended
// endpoints is a state — a gauge per service. The endpoint address and the
// reason for the suspension are unbounded/diagnostic detail: they live in the
// log line, never in a metric attribute.
type otelObserver struct {
	suspensions metric.Int64Counter
	suspended   metric.Int64UpDownCounter
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean, so an
// application that contributes its own Observer takes precedence.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	suspensions, err := m.Int64Counter("loadbalance.endpoint.suspension.total",
		metric.WithDescription("Endpoints suspended for consecutive failures; repeated suspensions of the same endpoint count again"),
		metric.WithUnit("{event}"))
	if err != nil {
		return nil, errutil.Explain(err, "loadbalance: register loadbalance.endpoint.suspension.total")
	}
	suspended, err := m.Int64UpDownCounter("loadbalance.endpoint.suspended",
		metric.WithDescription("Endpoints currently suspended from rotation"),
		metric.WithUnit("{endpoint}"))
	if err != nil {
		return nil, errutil.Explain(err, "loadbalance: register loadbalance.endpoint.suspended")
	}
	return &otelObserver{suspensions, suspended}, nil
}

// Suspended implements [Observer].
func (o *otelObserver) Suspended(ctx context.Context, service, _ string) {
	o.suspensions.Add(ctx, 1, metric.WithAttributes(attribute.String("service", service)))
}

// SuspendedDelta implements [Observer]. delta is +1 when an endpoint is
// suspended, -1 when it recovers, is removed as stale, or its generation is closed.
func (o *otelObserver) SuspendedDelta(ctx context.Context, service string, delta int64) {
	o.suspended.Add(ctx, delta, metric.WithAttributes(attribute.String("service", service)))
}
