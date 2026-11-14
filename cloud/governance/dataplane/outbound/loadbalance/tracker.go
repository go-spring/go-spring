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
	"go-spring.org/stdlib/errutil"
)

// endpointTag is this layer's dedicated stream for endpoint up/down tracking —
// suspension, re-admission and the clean-slate return to rotation. It is
// separate from the application tag so these rotation diagnostics can be
// routed and filtered on their own; none of them is an application event.
var endpointTag = log.RegisterAppTag("loadbalance", "endpoint")

// TrackerConfig configures a [Tracker]. Every threshold is an ABSOLUTE count
// and every knob is zero-value-safe: an unset threshold adopts its default, a
// NEGATIVE threshold disables that arm, and a zero MinHealthyPercent disables the
// panic floor. Suspension is therefore ON by default — a zero config protects
// rather than waits to be asked.
type TrackerConfig struct {
	// DialThreshold is the number of consecutive [connectFailed]
	// outcomes that suspends an endpoint. 0 adopts the default (2); negative
	// disables the dial arm. A dial failure is the strongest evidence this
	// package holds, so its threshold sits below the request one.
	DialThreshold int

	// RequestThreshold is the number of consecutive [failed]
	// outcomes that suspends an endpoint. 0 adopts the default (5); negative
	// disables the request arm. The two arms count independently; either one
	// tripping suspends the endpoint.
	RequestThreshold int

	// SuspendDuration is the BASE ejection time: how long a suspended endpoint
	// stays out before it is returned with a clean failure slate. The real
	// ejection time is multiplied by the number of times the address has been
	// ejected and capped by [TrackerConfig.MaxSuspendDuration], the shape Envoy
	// uses. 0 adopts the default (30s).
	SuspendDuration time.Duration

	// MaxSuspendDuration caps the multiplied ejection time. 0 adopts the default
	// (300s). The effective cap is the larger of this and SuspendDuration,
	// matching Envoy's max_ejection_time rule, so a base longer than the cap is
	// never shortened on a later ejection.
	MaxSuspendDuration time.Duration

	// Interval is Envoy's ejection-analysis sweep interval. This tracker has no
	// background sweep — it is evaluated on the request path — so it uses the
	// interval for the one thing a sweep decides on its own: an endpoint that
	// has stayed in rotation for each full interval after re-admission drops its
	// ejection backoff by one, Envoy's rule. It is NOT a window in which failures
	// must fall; the consecutive-failure counters are cleared only by a success.
	// 0 adopts the default (10s).
	Interval time.Duration

	// MaxEjectionPercent caps how much of the eligible pool may be out of
	// rotation at once. It is the percentage of the pool, rounded down: a pool
	// too small for one full percent gets no ejection. This is Envoy's
	// max_ejection_percent comparison (100*ejected/pool <= percent) with its
	// at-least-one carve-out off, the default (`always_eject_one_host`); this
	// tracker has no such switch. Raise the percentage for a small pool. 0
	// adopts the default (10); 100 means no cap.
	MaxEjectionPercent int

	// MinHealthyPercent is the panic threshold: the share of the eligible pool
	// that must remain servable. The pool is what [Tracker.Available] was
	// handed: non-disabled, undrained, after the selection's metadata match.
	// When the servable share falls below it — health loss, or ejections
	// outliving a discovery shrink — Available panics: it ignores health and
	// suspension and serves the whole pool. This mirrors Envoy's
	// healthy_panic_threshold (the proxy then load-balances across all hosts,
	// as Istio documents for minHealthPercent); it does NOT gate ejections —
	// [TrackerConfig.MaxEjectionPercent] is the ejection cap.
	//
	// "Servable" is the non-suspended healthy subset, or the whole available
	// subset when the backend reports no health (every Healthy is false). 0
	// disables the floor; the valid range is 0-100.
	//
	// The denominator is the pool the last Available observed, not every address
	// this tracker has ever seen: a suspension outside the current selection
	// must not consume the floor of the selection in force.
	MinHealthyPercent int
}

// Policy-level defaults follow Envoy's outlier detection for the request arm
// and the ejection shape — five consecutive request failures, a 30s base
// ejection time capped at 300s, a 10s analysis interval and at most 10% of the
// pool ejected. The dial arm is this package's split of the local-origin
// evidence and uses a lower threshold (2): a refused connection is
// near-deterministic evidence, where Envoy folds dial failures into its single
// consecutive-failure counter (default 5).
const (
	defaultDialThreshold      = 2
	defaultRequestThreshold   = 5
	defaultSuspendDuration    = 30 * time.Second
	defaultMaxSuspendDuration = 300 * time.Second
	defaultInterval           = 10 * time.Second
	defaultMaxEjectionPercent = 10
)

// suspendState is one endpoint address's suspension bookkeeping. resumeAt
// doubles as the state flag: zero means "in steady rotation", non-zero is when
// the current ejection ends. ejections counts how often this address has been
// ejected while continuously present in discovery; it multiplies the ejection
// time, decays while the address stays in rotation and is reset when the
// address leaves the endpoint set.
type suspendState struct {
	dialFails int
	reqFails  int
	resumeAt  time.Time
	ejections int

	// unejectedAt starts the current healthy stretch — set when the endpoint is
	// re-admitted — and is the clock the ejection backoff decays against: one
	// step per full Interval in rotation. Zero for an endpoint never ejected.
	unejectedAt time.Time
}

// suspended reports whether s is out of rotation. It is the condition the
// suspended gauge counts.
func (s *suspendState) suspended() bool { return !s.resumeAt.IsZero() }

// suspendOutcome is one Record's suspension decision, carried out of the
// critical section so observation and logging run after the lock is released.
// The zero value means "no suspension, nothing to report".
type suspendOutcome struct {
	suspended      bool
	refused        bool
	live           int
	suspendedCount int
	budget         int
}

// Tracker is the load-balancing layer's endpoint registry for ONE service: it
// watches the typed outcome of each round per endpoint ([errorKind],
// settled by the chain's addressing stage) and temporarily suspends an endpoint
// that fails repeatedly on either arm — consecutive dial failures or
// consecutive request failures. Once the
// ejection time elapses the endpoint is returned to rotation with a clean
// failure streak; the ejection COUNT survives, so a repeatedly failing address
// stays out longer each time (base × count, capped). While the address stays in
// rotation the count decays by one per [TrackerConfig.Interval], so a recovered
// endpoint is eventually forgiven; it resets outright when the address leaves
// the endpoint set.
//
// Re-admission is TIMER-BASED, not decided by a trial round. This follows
// client-side outlier detection (Envoy / gRPC xDS) rather than a circuit
// breaker's half-open probe: a round that was already in flight when the
// endpoint was suspended can never decide recovery, no matter how late it
// completes, and no extra admission token has to travel with the round.
//
// One tracker belongs to one chain, and a chain belongs to one service, so the
// service label it carries is constant. The suspension metrics are per service
// (the label is bounded); the endpoint address is unbounded and stays in the
// log line.
//
// It is the endpoint-axis counterpart to the circuit breaker in
// [go-spring.org/cloud/governance/dataplane/outbound/resilience]: the breaker is
// keyed by operation and answers "should this call go out at all"; the tracker
// is keyed by endpoint address and answers "is this peer worth routing to".
// The two share nothing but the verdict convention: the same round outcome
// feeds both, each counting what its own thresholds name.
//
// The state machine per endpoint is:
//
//	steady ──threshold reached──▶ suspended ──SuspendDuration elapsed──▶ steady
//	   ▲                                                                   │
//	   └─────────────────────── threshold reached ─────────────────────────┘
//
// Outcomes that carry no evidence ([contextCanceled], [resourceExhausted])
// never count and never reset a streak. Outcomes arriving while an endpoint is
// suspended belong to rounds admitted BEFORE it went out of rotation (or to
// the all-suspended fallback, which exists to keep traffic flowing, not to
// learn); they are ignored — recovery is the timer's decision, never a
// result's.
//
// The config is fixed for the tracker's whole life: a governance change does
// not retune a tracker in place, it REPLACES the whole generation the tracker
// belongs to (see [Chain]). The per-endpoint state (failure counts, cool-down
// deadlines) is that generation's own runtime state, born and dropped with it.
type Tracker struct {
	// service is the client service this tracker guards. One tracker belongs to
	// one service (one chain), and the service is the bounded label its
	// suspension metrics carry.
	service string

	// cfg is the normalized [TrackerConfig] this tracker was built with,
	// immutable afterwards. Defaults are applied by [NewTracker].
	cfg TrackerConfig

	// obs receives this tracker's suspension events. It is never nil: the caller
	// spells "off" as [emptyObserver], so the report path never branches on it.
	obs Observer

	mu     sync.Mutex
	states map[string]*suspendState

	// live is the eligible pool the last [Tracker.Available] observed — the
	// non-disabled, undrained set of the selection in force. The
	// MinHealthyPercent gate counts within it only, so a narrowed (canary)
	// selection is judged by its own endpoints rather than every address this
	// tracker has ever seen. Rebuilt on every Available (capacity retained), so
	// it is as fresh as the last pick.
	live map[string]struct{}

	// closed is set by [Tracker.Close] when the generation owning this tracker
	// is replaced. In-flight rounds of that generation may still settle
	// afterwards; their outcomes are dropped rather than resurrecting state (and
	// a gauge count that will never be returned).
	closed bool
}

// NewTracker builds a tracker around obs: the process-wide observer the Manager
// injects, or [emptyObserver] when observation is off. obs is required — the
// caller owns the "off" spelling, so the report path is a plain call. It applies
// the config defaults and rejects an invalid one; service is the label its
// events carry. The Manager is the normal caller; a custom chain composed
// outside the governance runtime builds its tracker here too, passing its own
// observer.
func NewTracker(service string, cfg TrackerConfig, obs Observer) (*Tracker, error) {
	if cfg.DialThreshold == 0 {
		cfg.DialThreshold = defaultDialThreshold
	}
	if cfg.RequestThreshold == 0 {
		cfg.RequestThreshold = defaultRequestThreshold
	}
	if cfg.SuspendDuration <= 0 {
		cfg.SuspendDuration = defaultSuspendDuration
	}
	if cfg.MaxSuspendDuration <= 0 {
		cfg.MaxSuspendDuration = defaultMaxSuspendDuration
	}
	// Envoy's effective cap is the configured max (or 300s) or the base,
	// whichever is larger, so a base longer than the cap is not shortened.
	if cfg.MaxSuspendDuration < cfg.SuspendDuration {
		cfg.MaxSuspendDuration = cfg.SuspendDuration
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.MaxEjectionPercent == 0 {
		cfg.MaxEjectionPercent = defaultMaxEjectionPercent
	}
	if cfg.MinHealthyPercent < 0 || cfg.MinHealthyPercent > 100 {
		return nil, errutil.Explain(nil, "loadbalance: min-healthy-percent must be within 0-100")
	}
	if cfg.MaxEjectionPercent < 0 || cfg.MaxEjectionPercent > 100 {
		return nil, errutil.Explain(nil, "loadbalance: max-ejection-percent must be within 0-100")
	}
	return &Tracker{service: service, cfg: cfg, obs: obs, states: map[string]*suspendState{}}, nil
}

// thresholdFor returns the configured threshold for a outcome's arm, or 0 when
// that arm is disabled. Only failure outcomes reach here.
func (t *Tracker) thresholdFor(v errorKind) int {
	if v == connectFailed {
		return t.cfg.DialThreshold
	}
	return t.cfg.RequestThreshold
}

// Available decides the SERVABLE subset of pool — the endpoints that may
// receive a round right now. Health preference, suspension and the
// MinHealthyPercent floor share one denominator (the pool), so they are decided
// here:
//
//   - suspension: cooling-down endpoints are dropped, and one whose cool-down
//     elapsed is returned to rotation with a clean failure slate;
//   - health preference: when the pool reports health (at least one Healthy
//     endpoint), only the available (non-suspended) healthy endpoints are
//     servable; when the backend reports no health at all — every Healthy is
//     false, the discovery contract for backends that do not track it — health
//     adds no information and the available set is servable;
//   - panic floor: when the servable share of the pool falls below
//     MinHealthyPercent — a real health loss, or ejections outliving a
//     discovery shrink — health and suspension are both ignored and the whole
//     pool is served, the behaviour Envoy/Istio run below their healthy
//     threshold. 0 disables the floor.
//
// When every endpoint is suspended it also falls back to the pool: black-holing
// traffic is worse than probing a degraded instance. It records the pool as the
// set the suspension gate counts within.
func (t *Tracker) Available(ctx context.Context, pool []discovery.Endpoint) []discovery.Endpoint {
	if len(pool) == 0 {
		return pool
	}
	t.mu.Lock()
	if t.live == nil {
		t.live = make(map[string]struct{}, len(pool))
	} else {
		clear(t.live)
	}
	for _, ep := range pool {
		t.live[ep.Addr] = struct{}{}
	}

	var available []discovery.Endpoint
	var reAdmitted []string
	if len(t.states) == 0 {
		available = pool // nothing tracked: every endpoint is available
	} else {
		available = make([]discovery.Endpoint, 0, len(pool))
		for _, ep := range pool {
			ok, now := t.availableLocked(ep.Addr)
			if ok {
				available = append(available, ep)
			}
			if now {
				reAdmitted = append(reAdmitted, ep.Addr)
			}
		}
	}
	t.mu.Unlock()

	// Report after unlocking: the transition is rare, but a mass cool-down
	// expiry would otherwise write synchronously under the mutex every pick
	// waits on.
	for _, addr := range reAdmitted {
		t.obs.SuspendedDelta(ctx, t.service, -1)
		log.Debug(ctx, endpointTag,
			"cool-down elapsed, endpoint back in rotation (failure slate cleared)",
			func() []log.Field {
				return []log.Field{log.String("service", t.service), log.String("address", addr)}
			})
	}

	return t.servable(pool, available)
}

// servable applies health preference and the panic floor over pool, given its
// available (non-suspended) subset. It reads only its arguments and the
// immutable config, so it runs outside the lock.
//
// Whether the backend reports health is decided over the POOL, not the
// available subset: if every healthy peer happens to be suspended, the pool
// still reports health, so the health preference applies and the result is the
// panic path — not the "no health information" fallback that would silently
// serve only the unhealthy peers.
func (t *Tracker) servable(pool, available []discovery.Endpoint) []discovery.Endpoint {
	if len(available) == 0 {
		return pool // every endpoint suspended: probing beats black-holing
	}
	healthy := make([]discovery.Endpoint, 0, len(available))
	for _, ep := range available {
		if ep.Healthy {
			healthy = append(healthy, ep)
		}
	}
	servable := healthy
	if !anyHealthy(pool) {
		servable = available // the backend reports no health
	}
	if pct := t.cfg.MinHealthyPercent; pct > 0 && len(servable)*100 < len(pool)*pct {
		return pool // panic: ignore health and suspension
	}
	if len(servable) == 0 {
		// Health is unsatisfiable and the floor is off: probe the degraded
		// peers rather than black-hole.
		return available
	}
	return servable
}

// anyHealthy reports whether the set carries health information at all.
func anyHealthy(eps []discovery.Endpoint) bool {
	for _, ep := range eps {
		if ep.Healthy {
			return true
		}
	}
	return false
}

// availableLocked reports whether addr may receive a round, and whether this
// call RE-ADMITTED it. Re-admission is timer-based: once the ejection time
// elapses, the consecutive-failure streak is cleared and the endpoint returns
// to rotation. The ejection COUNT is kept — it multiplies the next ejection
// time and decays while the endpoint stays healthy (see [Tracker.decayLocked])
// — and is reset when the address leaves the endpoint set (see
// [Tracker.RemoveStale]). The caller reports the gauge return after releasing
// the lock.
//
// Caller holds t.mu.
func (t *Tracker) availableLocked(addr string) (available, reAdmitted bool) {
	s := t.states[addr]
	if s == nil {
		return true, false // never failed
	}
	now := time.Now()
	if !s.suspended() {
		t.decayLocked(s, now)
		return true, false // already in rotation
	}
	if now.Before(s.resumeAt) {
		return false, false // ejected, still within the ejection time
	}
	s.dialFails, s.reqFails = 0, 0
	s.resumeAt = time.Time{}
	s.unejectedAt = now
	return true, true
}

// decayLocked drops the ejection backoff one step per full Interval the
// endpoint has stayed in rotation since re-admission. Envoy decrements the
// backoff once per sweep for a host unejected for at least one interval; with
// no sweep loop, the elapsed time gives the same count. A host that keeps
// failing is ejected again before a whole interval passes, so its backoff still
// escalates; a host that recovers is forgotten over time.
//
// Caller holds t.mu.
func (t *Tracker) decayLocked(s *suspendState, now time.Time) {
	if s.ejections == 0 || s.unejectedAt.IsZero() {
		return
	}
	steps := int64(now.Sub(s.unejectedAt) / t.cfg.Interval)
	if steps <= 0 {
		return
	}
	if steps >= int64(s.ejections) {
		s.ejections = 0
		s.unejectedAt = time.Time{}
		return
	}
	s.ejections -= int(steps)
	s.unejectedAt = s.unejectedAt.Add(time.Duration(steps) * t.cfg.Interval)
}

// RemoveStale drops the state of addresses that are no longer in live, the
// endpoint set the source currently advertises. A tracker outlives individual
// endpoints: discovery churn (rescheduled pods, scale-down) otherwise leaves
// state behind for addresses that will never be picked again, which both leaks
// the map and keeps the "currently suspended" gauge counting ghosts. Dropping
// the state also resets the address's ejection count — Envoy's rule: a host
// removed from the cluster and re-added starts over.
//
// An empty live set is ignored: a source that momentarily advertises nothing is
// not evidence that every endpoint is gone, and wiping all state on a transient
// refresh failure would forget real suspensions. It costs one pass over the
// tracked addresses per call, the same order [Tracker.Available] already pays.
// ctx is the pick's, carried into the gauge events so they keep its trace.
func (t *Tracker) RemoveStale(ctx context.Context, live []discovery.Endpoint) {
	if len(live) == 0 {
		return
	}
	t.mu.Lock()
	if t.closed || len(t.states) == 0 {
		t.mu.Unlock()
		return
	}

	present := make(map[string]struct{}, len(live))
	for _, ep := range live {
		present[ep.Addr] = struct{}{}
	}
	dropped := 0
	for addr, s := range t.states {
		if _, ok := present[addr]; ok {
			continue
		}
		if s.suspended() {
			dropped++
		}
		delete(t.states, addr)
	}
	t.mu.Unlock()

	// Only the state pruning belongs under the lock; the gauge returns are
	// reported here, one share per address that actually left suspended.
	for i := 0; i < dropped; i++ {
		t.obs.SuspendedDelta(ctx, t.service, -1)
	}
}

// Record folds one round's outcome into the registry: it settles err — the
// round's own error, the same value the caller observed — via [classifyError],
// so callers hand over the fact, not a classification of it. ctx is the context
// of the round, carried into the log lines so they keep its trace and values;
// it is read, never canceled, here.
//
// A success (err == nil) clears the endpoint's failure streak; the ejection
// count is kept while the address stays in discovery, because it multiplies the
// next ejection time. A failure with evidence counts on its own arm — CONSECUTIVE
// failures, Envoy's semantics: nothing but a success breaks the streak, so the
// count is not bounded by a time window — and suspends the endpoint when its
// threshold is reached, unless [TrackerConfig.MaxEjectionPercent] refuses it. An
// outcome without evidence (caller abort, local limit) is ignored. An outcome
// arriving while the endpoint is ejected is ignored: it belongs to a round
// admitted before the ejection (or to the panic fallback), and recovery is the
// timer's decision, never a result's.
func (t *Tracker) Record(ctx context.Context, ep discovery.Endpoint, err error) {
	v := classifyError(err)
	addr := ep.Addr
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return // the generation is gone; its observations go with it
	}

	s := t.states[addr]
	if s == nil {
		// Only a FAILED round is evidence about the endpoint: connectFailed is
		// the strongest kind, and timeoutFailed / unknownFailed are counted on
		// the request arm.
		if v != connectFailed && v != timeoutFailed && v != unknownFailed {
			t.mu.Unlock()
			return // nothing to clear, and no state to keep
		}
		s = &suspendState{}
		t.states[addr] = s
	}

	if s.suspended() {
		t.mu.Unlock()
		return // stale: see the doc comment
	}

	var out suspendOutcome
	switch {
	case v == success:
		// The streak is broken. Keep the entry only when the address carries
		// ejection history: the count multiplies its next ejection time, Envoy's
		// rule while the host stays in the cluster.
		s.dialFails, s.reqFails = 0, 0
		if s.ejections == 0 {
			delete(t.states, addr)
		}
	case v != connectFailed && v != timeoutFailed && v != unknownFailed:
		// No evidence: the streak stays.
	default:
		if v == connectFailed {
			s.dialFails++
		} else {
			s.reqFails++
		}
		out = t.trySuspendLocked(addr, s, v)
	}
	t.mu.Unlock()

	// The tracker's mutex is on the pick hot path: only state changes happen
	// inside it, so observation and logging run here, after it is released.
	if out.suspended {
		t.obs.Suspended(ctx, t.service, v.String())
		t.obs.SuspendedDelta(ctx, t.service, 1)
		log.Warn(ctx, endpointTag, "endpoint suspended after consecutive failures",
			log.String("service", t.service), log.String("address", addr), log.String("reason", v.String()))
	}
	if out.refused {
		log.Warn(ctx, endpointTag, "suspension refused, max-ejection-percent reached",
			log.String("service", t.service), log.String("address", addr),
			log.Int("live", out.live), log.Int("suspended", out.suspendedCount),
			log.Int("budget", out.budget),
			log.Int("max_ejection_percent", t.cfg.MaxEjectionPercent))
	}
}

// trySuspendLocked suspends s when its arm's threshold is reached, subject to
// [TrackerConfig.MaxEjectionPercent], which counts within the eligible pool the
// last [Tracker.Available] observed (never every tracked address, so a narrowed
// selection is judged by its own endpoints).
//
// The ejection time is the base [TrackerConfig.SuspendDuration] multiplied by
// the current ejection backoff, capped by [TrackerConfig.MaxSuspendDuration];
// Envoy stops advancing the backoff once base × backoff reaches the cap, and
// the backoff decays while the endpoint is healthy. It reports its decision for
// the caller to observe and log after releasing the lock.
// Record has already ruled out an already-suspended state.
// Caller holds t.mu.
func (t *Tracker) trySuspendLocked(addr string, s *suspendState, v errorKind) suspendOutcome {
	threshold := t.thresholdFor(v)
	if threshold <= 0 {
		return suspendOutcome{} // arm disabled
	}
	fails := s.dialFails
	if v != connectFailed {
		fails = s.reqFails
	}
	if fails < threshold {
		return suspendOutcome{}
	}

	// Envoy resets the tripped arm's counter the moment the threshold fires,
	// whether or not the ejection gate below lets the ejection through; a
	// refused endpoint therefore has to build a fresh streak instead of
	// re-testing the gate on every subsequent failure.
	if v == connectFailed {
		s.dialFails = 0
	} else {
		s.reqFails = 0
	}

	total := len(t.live)
	if _, in := t.live[addr]; in {
		suspended := 0
		for a := range t.live {
			if st := t.states[a]; st != nil && st.suspended() {
				suspended++
			}
		}
		if budget := ejectionBudget(total, t.cfg.MaxEjectionPercent); suspended >= budget {
			return suspendOutcome{
				refused: true,
				live:    total, suspendedCount: suspended, budget: budget,
			}
		}
	}
	// An address outside the pool consumes no pool capacity: it may be ejected
	// without the gate.

	// Envoy's backoff advances only while it is still below the max, then the
	// duration is min(base × backoff, max). Decay may have run since the last
	// ejection, so the backoff is read again here.
	now := time.Now()
	t.decayLocked(s, now)
	base, maxD := t.cfg.SuspendDuration, t.cfg.MaxSuspendDuration
	if time.Duration(s.ejections)*base < maxD+base {
		s.ejections++
	}
	d := min(time.Duration(s.ejections)*base, maxD)
	s.resumeAt = now.Add(d)
	return suspendOutcome{suspended: true}
}

// ejectionBudget returns how many endpoints may be out of rotation at once: the
// configured percentage of the pool, rounded down. This is Envoy's comparison
// (100*ejected/pool <= max_ejection_percent) without its at-least-one
// carve-out, which is off by default (`always_eject_one_host`); a pool smaller
// than 100/pct therefore has a budget of zero, and raising the percentage is
// how a small pool opts in.
func ejectionBudget(total, pct int) int {
	return min(total*pct/100, total)
}

// suspendedCountLocked reports how many tracked addresses are out of rotation.
// Caller holds t.mu.
func (t *Tracker) suspendedCountLocked() int {
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
// Closing is final and idempotent. In-flight rounds of the retired generation
// may still settle afterwards — they carry their own `chain.Peer` — and their
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
		t.obs.SuspendedDelta(context.Background(), t.service, int64(-count))
	}
}
