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
	"sync"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/cloud/observability/instrument"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// clientInstrumentSet is the outbound REQUEST plane's always-on signals: the
// per-call duration and status counter. One set per process, resolved lazily on
// first use so it binds to whichever providers are current then, and immutable
// afterwards. It holds no per-client state — the system and service labels
// travel with each wrapper, not here.
//
// Governance organs' own state (breaker, rate budget) is deliberately absent:
// each organ publishes its own state from its own package, by pull. This plane
// answers only what happened to one call.
//
// It is a separate set from the inbound (resilience.server.*) family rather than
// one family with a direction attribute: the outcome sets genuinely differ
// (inbound has no retry, so no retry_exceeded and no per-attempt timeout),
// and keeping them apart leaves existing outbound dashboards untouched.
type clientInstrumentSet struct {
	duration      metric.Float64Histogram
	calls         metric.Int64Counter
	faultInjected metric.Int64Counter
	faultDelayed  metric.Int64Counter
}

// clientInstruments is the one outbound instrument set this package uses for the
// whole process.
var clientInstruments = sync.OnceValue(buildClientInstruments)

func buildClientInstruments() *clientInstrumentSet {
	m := otel.Meter(instrument.Scope)
	in := &clientInstrumentSet{}
	in.duration, _ = m.Float64Histogram("resilience.client.duration",
		metric.WithDescription("Duration of resilience-protected calls"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	// calls counts protected calls on the shared status axis (ok/error) plus,
	// where protection intervened, the resilience.error_type that says what it did
	// (rate_limited, circuit_open, bulkhead_full, retry_exceeded, timeout).
	in.calls, _ = m.Int64Counter("resilience.client.calls",
		metric.WithDescription("Number of resilience-protected calls by status"),
		metric.WithUnit("{call}"))
	// Fault counters are the experiment's per-rule result signal: how many calls
	// each rule actually affected, split by effect. run_id stays on spans/logs —
	// it is high-cardinality and must not become a metric label.
	in.faultInjected, _ = m.Int64Counter("resilience.client.fault.injected",
		metric.WithDescription("Calls whose error was injected by a fault rule"),
		metric.WithUnit("{call}"))
	in.faultDelayed, _ = m.Int64Counter("resilience.client.fault.delayed",
		metric.WithDescription("Calls delayed by a fault rule"),
		metric.WithUnit("{call}"))
	return in
}

// opInstrumentSet holds the instruments one metric prefix needs. The prefix
// comes from the operation a client declared (see [chain.Operation]), so
// this set cannot be built once per process the way [clientInstrumentSet] is — a
// process serves any number of client families, each with its own prefix. It is
// memoized per prefix on first use, which is also what binds it to whichever
// meter provider is current then.
type opInstrumentSet struct {
	duration metric.Float64Histogram
	attempts metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// opInstruments memoizes one [opInstrumentSet] per metric prefix. It holds no
// per-call state — the system/service/operation labels travel with each record.
var opInstruments = struct {
	mu sync.Mutex
	m  map[string]*opInstrumentSet
}{m: map[string]*opInstrumentSet{}}

// operationInstruments returns the instrument set for a metric prefix, building
// it on first use. Two names per prefix carry the two data grains: the call-level
// ".operation.duration" (attempts and backoff included — what this service
// promised its caller) and the attempt-level ".attempt.duration" (what the
// downstream itself took, per try). Keeping them separate is the point: mixing
// retry cost into downstream latency makes "raise the backoff" read as "the
// downstream got slower". The active-requests gauge is in-flight CALLS, so it
// lives here beside the call level, not on the attempt.
func operationInstruments(prefix string) *opInstrumentSet {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	if s, ok := opInstruments.m[prefix]; ok {
		return s
	}
	m := otel.Meter(instrument.ComponentName)
	s := &opInstrumentSet{}
	var err error
	s.duration, err = m.Float64Histogram(prefix+".operation.duration",
		metric.WithDescription("Duration of one logical client operation, retries and backoff included"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	instrument.ReportInstrumentError(prefix, err)
	s.attempts, err = m.Float64Histogram(prefix+".attempt.duration",
		metric.WithDescription("Duration of one downstream attempt inside a client operation"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	instrument.ReportInstrumentError(prefix, err)
	s.active, err = m.Int64UpDownCounter(prefix+".active_requests",
		metric.WithDescription("Number of in-flight client operations"),
		metric.WithUnit("{request}"))
	instrument.ReportInstrumentError(prefix, err)
	opInstruments.m[prefix] = s
	return s
}

// ResetInstruments drops the memoized outbound instrument sets so the next use
// resolves fresh ones. It exists for tests that install their own
// MeterProvider: the sets are process-wide and resolved once, so a test running
// after one that already resolved them would otherwise keep reporting into the
// earlier provider.
//
// It covers both the always-on client set and the per-prefix operation sets,
// since a MeterProvider swap invalidates all of them together.
func ResetInstruments() {
	clientInstruments = sync.OnceValue(buildClientInstruments)
	resetOperationInstruments()
}

// resetOperationInstruments drops the memoized per-prefix sets, for the same
// reason [ResetInstruments] does.
func resetOperationInstruments() {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	opInstruments.m = map[string]*opInstrumentSet{}
}
