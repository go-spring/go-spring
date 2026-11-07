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

package loadbalance

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation component every meter in this package
// reports under.
const componentName = "go-spring.org/cloud/loadbalance"

// suspendState is one endpoint address's suspension bookkeeping. suspendedAt
// doubles as the state flag: zero means "not suspended", non-zero is when the
// current cool-down window started.
type suspendState struct {
	failures    int
	halfOpen    bool
	suspendedAt time.Time

	// seen is scratch space for [Tracker.Prune]: it marks the addresses present
	// in the current endpoint set so the sweep can drop the rest. It carries no
	// meaning between calls.
	seen bool
}

// suspended reports whether s is out of steady rotation: cooling down, or
// admitted as a half-open trial but not yet recovered. It is the condition the
// suspended gauge counts.
func (s *suspendState) suspended() bool { return !s.suspendedAt.IsZero() }

// TrackerConfig is the configuration for a [Tracker].
type TrackerConfig struct {
	// Threshold is the number of consecutive failures that suspends an endpoint.
	// 0 (or negative) disables suspension entirely.
	Threshold int

	// SuspendFor is how long an endpoint stays suspended before a half-open trial
	// request is allowed through. Defaults to 5s when unset (and suspension is
	// enabled), matching the resilience breaker default.
	SuspendFor time.Duration
}

// Tracker is the load-balancing layer's outlier-suspension mechanism. It
// watches the success/failure outcome of balanced requests per endpoint and
// temporarily suspends an instance that fails repeatedly. Once the cool-down
// elapses it admits the endpoint's traffic again (a half-open round): the first
// success restores it, any failure re-suspends it for a fresh cool-down.
//
// It is the LB-layer counterpart to the circuit breaker in
// [go-spring.org/cloud/governance/dataplane/outbound/resilience]: the same
// consecutive-failure + half-open semantics, but keyed by endpoint address and
// *queryable* so a [Pool] can drop bad instances from the candidate set
// proactively rather than only rejecting a call after it is routed. The same
// Complete-err signal that feeds a resilience chain.Executor feeds a Tracker, so
// the two stay consistent without one depending on the other.
//
// A Tracker with Threshold <= 0 is disabled: [Tracker.Allows] returns every
// endpoint and [Tracker.Record] is a no-op. It is safe to attach a disabled
// Tracker — it has no effect until Threshold is set.
//
// The config is fixed for the tracker's whole life: a governance change does
// not retune a tracker in place, it REPLACES the whole generation the tracker
// belongs to (see [Chain]). The per-endpoint state (failure counts, cool-down
// windows) is that generation's own runtime state, born and dropped with it.
type Tracker struct {
	// cfg is the normalized [TrackerConfig] this tracker was built with,
	// immutable afterwards. SuspendFor's default is applied by [NewTracker].
	cfg TrackerConfig

	// ins is the shared instrument set, resolved once on first use (see
	// [instruments]) so an OTel SDK installed later still receives the records.
	// The tracer is deliberately NOT held here.
	ins *instrumentSet

	mu     sync.Mutex
	states map[string]*suspendState

	// closed is set by [Tracker.Close] when the generation owning this tracker
	// is replaced. In-flight requests of that generation may still settle
	// afterwards; their outcomes are dropped rather than resurrecting state (and
	// a gauge count that will never be returned).
	closed bool
}

// instrumentSet is this package's metric set: one per process, resolved lazily
// on first use so it binds to whichever meter provider is current then, and
// immutable afterwards. It holds no per-tracker state — the endpoint address is
// never a metric attribute, so no per-endpoint value travels here either.
//
// The suspension of an endpoint is an event (a dedicated counter named after the
// event, per the metrics rules), and the number of currently suspended endpoints
// is a state — a gauge. The endpoint address is unbounded cardinality and never a
// metric attribute; it lives in the log line.
type instrumentSet struct {
	suspensions metric.Int64Counter
	suspended   metric.Int64UpDownCounter
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

// buildInstruments resolves the process-wide instruments from the meter provider
// in force when it first runs.
func buildInstruments() *instrumentSet {
	m := otel.Meter(componentName)
	set := &instrumentSet{}
	set.suspensions, _ = m.Int64Counter("loadbalance.endpoint.suspension.total",
		metric.WithDescription("Endpoints suspended for consecutive failures, including re-suspensions after failed half-open trials"),
		metric.WithUnit("{event}"))
	set.suspended, _ = m.Int64UpDownCounter("loadbalance.endpoint.suspended",
		metric.WithDescription("Endpoints currently suspended from rotation"),
		metric.WithUnit("{endpoint}"))
	return set
}

// NewTracker builds a [Tracker] from cfg. A zero SuspendFor is normalized to
// the 5s default here.
func NewTracker(cfg TrackerConfig) *Tracker {
	if cfg.SuspendFor <= 0 {
		cfg.SuspendFor = 5 * time.Second
	}
	return &Tracker{
		cfg:    cfg,
		states: map[string]*suspendState{},
		ins:    instruments(),
	}
}

// Allows returns the subset of eps that may currently receive traffic:
// endpoints that are suspended and still cooling down are dropped, while a
// suspended endpoint whose cool-down has elapsed is admitted (half-open
// trial). When the tracker is disabled it returns eps unchanged.
//
// If every endpoint is suspended it falls back to eps — black-holing all
// traffic is worse than probing a degraded instance.
func (t *Tracker) Allows(eps []discovery.Endpoint) []discovery.Endpoint {
	cfg := t.cfg
	if cfg.Threshold <= 0 || len(eps) == 0 {
		return eps
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	allowed := make([]discovery.Endpoint, 0, len(eps))
	for _, ep := range eps {
		if t.admit(ep.Addr, cfg.SuspendFor) {
			allowed = append(allowed, ep)
		}
	}
	if len(allowed) == 0 {
		return eps
	}
	return allowed
}

// admit reports whether addr may receive a request, advancing a suspended
// endpoint into the half-open trial state once its cool-down has elapsed. Caller
// holds t.mu.
func (t *Tracker) admit(addr string, suspendFor time.Duration) bool {
	s := t.states[addr]
	if s == nil || s.suspendedAt.IsZero() {
		return true // never failed, or recovered
	}
	if time.Since(s.suspendedAt) < suspendFor {
		return false // suspended, cooling down
	}
	if !s.halfOpen {
		s.halfOpen = true // cool-down elapsed: admit a trial request
		log.Info(context.Background(), log.TagAppDef,
			"loadbalance: cool-down elapsed, admitting a half-open trial",
			log.String("address", addr))
	}
	return true
}

// Prune drops the suspension state of addresses that are no longer in live, the
// endpoint set the source currently advertises. A tracker outlives individual
// endpoints: discovery churn (rescheduled pods, scale-down) otherwise leaves
// state behind for addresses that will never be picked again, which both leaks
// the map and keeps the "currently suspended" gauge counting ghosts.
//
// An empty live set is ignored: a source that momentarily advertises nothing is
// not evidence that every endpoint is gone, and wiping all state on a transient
// refresh failure would forget real suspensions. It costs one pass over the
// tracked addresses per call, the same order [Tracker.Allows] already pays.
func (t *Tracker) Prune(live []discovery.Endpoint) {
	if t.cfg.Threshold <= 0 || len(live) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || len(t.states) == 0 {
		return
	}

	for _, ep := range live {
		if s := t.states[ep.Addr]; s != nil {
			s.seen = true
		}
	}
	for addr, s := range t.states {
		if s.seen {
			s.seen = false
			continue
		}
		if s.suspended() {
			t.ins.suspended.Add(context.Background(), -1)
		}
		delete(t.states, addr)
	}
}

// suspendedCount reports how many tracked addresses are out of rotation. It is
// the value the shared suspended gauge should hold on this tracker's behalf.
func (t *Tracker) suspendedCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, s := range t.states {
		if s.suspended() {
			count++
		}
	}
	return count
}

// Close returns this tracker's share of the process-wide "currently suspended"
// gauge and drops its state. A tracker is one generation's runtime state (see
// [Chain]): when a governance change replaces the generation, nothing will ever
// decrement the +1s this tracker contributed, so leaving them behind makes the
// gauge drift upward across pushes.
//
// Closing is final and idempotent. In-flight requests of the retired generation
// may still settle afterwards — they carry their own [PickResult] — and their
// outcomes are dropped: the generation is gone, so what it would have learned
// belongs to no one, and letting it recreate state would leak the gauge it just
// returned.
func (t *Tracker) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	count := 0
	for _, s := range t.states {
		if s.suspended() {
			count++
		}
	}
	clear(t.states)
	t.mu.Unlock()
	if count > 0 {
		t.ins.suspended.Add(context.Background(), int64(-count))
	}
}

// Record folds a request outcome back into the tracker. ctx is the context of
// the request whose outcome this is, carried into the suspension log lines so
// they keep its trace and values; it is read, never canceled, here. success=true
// clears the endpoint's failure state (or closes a half-open trial);
// success=false counts toward suspension (or re-suspends a failed half-open
// trial). It is a no-op when the tracker is disabled.
//
// An outcome from a request admitted BEFORE the endpoint was suspended is
// ignored: while it cools down the endpoint receives no new traffic, so the
// result is stale, and only the half-open trial may decide recovery.
func (t *Tracker) Record(ctx context.Context, addr string, success bool) {
	cfg := t.cfg
	if cfg.Threshold <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return // the generation is gone; its observations go with it
	}

	s := t.states[addr]
	if s == nil {
		if success {
			return // never failed: nothing to clear, and no state to keep
		}
		s = &suspendState{}
		t.states[addr] = s
	}
	// While the endpoint cools down, Allows admits none of its traffic, so an
	// outcome arriving now belongs to a request picked BEFORE the suspension. It
	// describes a state that no longer exists: it must not close the circuit
	// (only the half-open trial can) nor extend the cool-down. This is the same
	// rule the circuit breaker enforces with its permit generations.
	if !s.suspendedAt.IsZero() && !s.halfOpen {
		return
	}
	if success {
		wasDown := !s.suspendedAt.IsZero() || s.halfOpen
		s.failures = 0
		s.suspendedAt = time.Time{}
		s.halfOpen = false
		if wasDown {
			t.ins.suspended.Add(context.Background(), -1)
			log.Info(ctx, log.TagAppDef, "loadbalance: endpoint recovered, back in rotation",
				log.String("address", addr))
		}
		return
	}
	if s.halfOpen {
		// Trial request failed: restart the cool-down window. The endpoint was
		// counted as suspended all along (half-open is still out of steady
		// rotation), so the gauge does not move — only the event counter does.
		s.halfOpen = false
		s.suspendedAt = time.Now()
		t.ins.suspensions.Add(context.Background(), 1)
		log.Warn(ctx, log.TagAppDef, "loadbalance: half-open trial failed, endpoint re-suspended",
			log.String("address", addr))
		return
	}
	s.failures++
	if s.failures >= cfg.Threshold && s.suspendedAt.IsZero() {
		s.suspendedAt = time.Now()
		t.ins.suspensions.Add(context.Background(), 1)
		t.ins.suspended.Add(context.Background(), 1)
		log.Warn(ctx, log.TagAppDef, "loadbalance: endpoint suspended after consecutive failures",
			log.String("address", addr),
			log.Int("failures", s.failures),
			log.Float("suspend_for_ms", float64(cfg.SuspendFor.Nanoseconds())/1e6))
	}
}
