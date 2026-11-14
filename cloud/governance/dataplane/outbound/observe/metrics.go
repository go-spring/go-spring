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

package observe

import (
	"context"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics receives the outbound call's METRIC events. It is the seam for the
// metric plane only: the call span and the access log are the STANDARD
// capabilities (OTel / the log package) and stay in the emitters. A nil Metrics
// disables the metrics.
type Metrics interface {
	// Inflight moves the in-flight call gauge by delta: +1 when a call starts,
	// -1 when it finishes.
	Inflight(ctx context.Context, c Call, delta int64)

	// Call records one finished call: the status-classified call counter and
	// the call-level duration histogram.
	Call(ctx context.Context, c Call, status string, errorType chain.ErrorType, d time.Duration)

	// Attempt records one downstream attempt (one round of the retry loop).
	Attempt(ctx context.Context, c Call, status string, errorType chain.ErrorType, d time.Duration)

	// Fault records one fired round of a fault rule.
	Fault(ctx context.Context, c Call, f Fault)
}

// Call is one outbound call as the metric seam reports it.
type Call struct {
	// System is the client's backend name ("redis", "kafka", ...).
	System string
	// Service is the per-instance label the emitter was built for.
	Service string
	// Target is what the call went TO: the operation's own target when it
	// declared one, else Service.
	Target string
	// Operation is the operation's metric prefix; "" means the client declared
	// no per-operation metrics.
	Operation string
	// Attrs are the bounded labels the record carries.
	Attrs []attribute.KeyValue
}

// Fault is one fired round of a fault rule, as the metric seam reports it.
type Fault struct {
	// Kind is the effect's vocabulary word; Class is its failure class; Status
	// is the protocol status the rule declared (0 when none).
	Kind, Class string
	Status      int
	// Rule names the rule that fired, and Attempt is the round it fired in.
	Rule    string
	Attempt int
	// Inject is whether the round's error was injected (vs a delay only); Delay
	// is the latency the rule added.
	Inject bool
	Delay  time.Duration
}

// NewOTelMetrics returns the default OTel-backed [Metrics]. It is registered as
// the process's default bean.
func NewOTelMetrics() Metrics { return otelMetrics{} }

// otelMetrics is the default [Metrics].
type otelMetrics struct{}

// Inflight implements [Metrics].
func (otelMetrics) Inflight(ctx context.Context, c Call, delta int64) {
	operationInstruments(c.Operation).active.Add(ctx, delta,
		metric.WithAttributes(callLabels(c.Target, c.Attrs, "", "")...))
}

// Call implements [Metrics].
func (otelMetrics) Call(ctx context.Context, c Call, status string, errorType chain.ErrorType, d time.Duration) {
	calls := []attribute.KeyValue{
		attribute.String("system", c.System),
		attribute.String("service", c.Service),
		attribute.String("status", status),
	}
	if errorType != "" {
		calls = append(calls, attribute.String("resilience.error_type", string(errorType)))
	}
	clientInstruments().calls.Add(ctx, 1, metric.WithAttributes(calls...))
	operationInstruments(c.Operation).duration.Record(ctx, d.Seconds(),
		metric.WithAttributes(callLabels(c.Target, c.Attrs, status, errorType)...))
}

// Attempt implements [Metrics].
func (otelMetrics) Attempt(ctx context.Context, c Call, status string, errorType chain.ErrorType, d time.Duration) {
	operationInstruments(c.Operation).attempts.Record(ctx, d.Seconds(),
		metric.WithAttributes(callLabels(c.Target, c.Attrs, status, errorType)...))
}

// Fault implements [Metrics].
func (otelMetrics) Fault(ctx context.Context, c Call, f Fault) {
	base := []attribute.KeyValue{
		attribute.String("system", c.System),
		attribute.String("service", c.Target),
		attribute.String("rule", f.Rule),
		attribute.Int("attempt", f.Attempt),
	}
	if f.Inject {
		labels := append(append([]attribute.KeyValue{}, base...),
			attribute.String("kind", f.Kind),
			attribute.String("class", f.Class))
		if f.Status > 0 {
			labels = append(labels, attribute.Int("status", f.Status))
		}
		clientInstruments().faultInjected.Add(ctx, 1, metric.WithAttributes(labels...))
	}
	if f.Delay > 0 {
		clientInstruments().faultDelayed.Add(ctx, 1, metric.WithAttributes(base...))
	}
}

// callLabels builds the metric labels for a call: the operation's bounded attrs,
// the target label, and — where the metric is outcome-classified — the status.
//
// The service label is what keeps two instances of one backend (two memcached
// pools, two redis instances) from collapsing into a single series: the
// operation's own attrs cannot express it. Detail is never included, so a key or
// a statement can never become a label.
//
// The slice is fresh: attrs comes off the context and is shared, so appending in
// place could write into a sibling call's array.
func callLabels(service string, attrs []attribute.KeyValue, status string, errorType chain.ErrorType) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs)+3)
	out = append(out, attrs...)
	out = append(out, attribute.String("service", service))
	if errorType != "" {
		out = append(out, attribute.String("resilience.error_type", string(errorType)))
	}
	if status == "" {
		return out
	}
	return append(out, attribute.String("status", status))
}
