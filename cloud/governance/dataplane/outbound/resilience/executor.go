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

// executor.go is the bundled "default" engine for OUTBOUND calls and the
// organs only it composes: the client executor (timeout + retry
// around the shared rate-limit and circuit-breaker organs) and the backoff
// that paces its attempts. Every capability the outbound side offers — rate
// limiting, circuit breaking, isolation, retry, timeout — is composed here.
//
// The breaker and the rate budget are not implemented here: they are the
// direction-neutral organs in go-spring.org/cloud/governance/capability/breaker and
// go-spring.org/cloud/governance/capability/ratelimit, shared with inbound admission so both directions
// run the same state machine and counters.

package resilience

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/log"
	"go-spring.org/stdlib/timeutil"
)

// Executor is the ONE outward type of this package: the resilience stage of a
// chain. It owns the rest of the chain (next) and the per-label protection state
// it runs on, so one label is one scope — build it once and hold it. The manager
// keeps the object for the process (a client holds it for life), so a push moves
// the state inside it, never the shell.
//
// next nil means the attempt is the call itself: [chain.Next] invokes fn. A
// non-nil next is what puts a layer INSIDE the retry loop — the loop calls it
// once per attempt — which is how the composition root places fault injection
// below governance without wrapping the whole stack from outside.
//
// # Three independently replaceable organs
//
// What the executor MATERIALIZES is: the per-key breaker table, the per-key
// rate-budget table, and the label-level retry budget. The manager owns the
// push: [Manager.ApplyBreaker] and [Manager.ApplyRateLimit] reach into the
// registered executors and move only the organ whose configuration moved, so a
// breaker change leaves a live rate budget counting and vice versa. Nothing
// about a replaced breaker table survives into the next one, which is what lets
// a config change be a plain construction instead of a per-organ merge of "what
// can be kept".
//
// A call already in flight when a swap happens keeps the organs it loaded: its
// outcome lands on organs nobody will consult again, which costs one lost sample
// and nothing else.
//
// # State is per key
//
// Protective state is partitioned by the identity the resolver produces: a
// matched rule's identity — its index for the breaker, its selector-derived key
// ([rateKey]) for a rate limit. A call governed by one rule gets its own breaker
// and rate budget, so a downstream that misbehaves for one traffic class cannot
// trip another's protection. Metadata selects a rule but never becomes a key — a key must stay
// bounded by the config. The retry budget is the one organ that is NOT per key:
// it is a per-label guard by design.
//
// The Mutex makes this type non-copyable; it is only ever used through a pointer.
type Executor struct {
	service string

	// localLimiters holds one counter per rate key, built by the limiterFactory the
	// key's rule names. It lives on the executor, not on a pushed document, so a
	// configuration change updates the existing counter (the rule travels with
	// every call) instead of replacing its state. A change of limiter KIND
	// replaces the entry, because the limiterFactory (and so the state shape) changed.
	localLimiters sync.Map // key -> *limiterEntry

	// The breaker and retry organs are materialized by the manager's push: a
	// change to one organ's knobs leaves the others' state untouched. An atomic
	// pointer keeps the call path — key already built — lock-free.
	breaker atomic.Pointer[breakerSlot]
	retry   atomic.Pointer[retrySlot]

	// mu serializes materialization — the first build of a key and the manager's
	// slot swap. It is never held on the fast path, and the manager is
	// never called under it.
	mu sync.Mutex

	// mgr is the manager this executor reads its policy from on every call. It is
	// set at construction and never replaced; the executor holds no configuration
	// of its own. It must NOT call back into this executor.
	mgr *Manager

	// next is the rest of the chain this stage delegates to.
	next chain.Executor
}

// breakerSlot is the breaker table the executor currently serves: a breaker per
// key, filled lazily on first use. A push that moves the label's breaker content
// replaces the whole slot, so state built under one configuration never migrates
// into the next and the next configuration's state never leaks in.
type breakerSlot struct {
	keys sync.Map // key -> *breakerCell
}

// breakerCell wraps a breaker so a DISABLED breaker can be stored (a nil
// *Breaker in a sync.Map would be indistinguishable from a missing entry).
type breakerCell struct{ b *circuitbreaker.Breaker }

// retrySlot is the label-level retry budget. The channel's capacity IS the
// declared budget, so no separate copy of the value is stored: a change is
// visible as a different cap.
type retrySlot struct {
	sem chan struct{}
}

// callPlan is one call's governance plan: the values this call reads, the knobs
// its gates apply, and the handles that enforce them. Everything here is ready
// to use — matching, materialization and the retry merge happen together in
// [Executor.planFor].
type callPlan struct {
	timeout    time.Duration
	retry      Retry                   // the effective schedule; the budget lives in budget, not here
	attempts   int                     // retry.MaxRetries + 1
	breaker    *circuitbreaker.Breaker // nil = the breaker stage is off
	rate       ratelimit.Limiter       // nil = no rate limit configured
	ratePolicy ratelimit.Policy        // what the limiter charges against
	budget     chan struct{}           // nil = no retry budget
}

// limiterEntry binds one counter to the kind that built it; a change of kind
// starts that key's state over.
type limiterEntry struct {
	kind    string
	limiter ratelimit.Limiter
}

// Wrap returns service's resilience node with next as the rest of the chain. The
// node owns its protection state: one label is one protection scope, so the
// caller must build it once and hold it.
//
// The executor registers with the manager, which is how a later push reaches it.
// Registration happens at wiring; pushes run on the source loop afterwards.
//
// The executor publishes no state of its own: what an operator needs to know
// about protection is carried by the outcome of the calls it governs — a
// refusal is a call whose error names the gate — so there is no second series
// to keep in step with the call plane.
func (m *Manager) Wrap(service string, next chain.Executor) *Executor {
	e := &Executor{service: service, mgr: m, next: next}
	e.breaker.Store(&breakerSlot{})
	m.executors[e] = struct{}{}
	return e
}

// Close drops this executor from its manager's registration table and closes the
// part of the chain it owns. It runs at destroy, after the source loops have
// stopped, so a later push cannot reach this shell.
func (e *Executor) Close() error {
	delete(e.mgr.executors, e)
	if e.next == nil {
		return nil
	}
	return e.next.Close()
}

// planFor answers, for ONE call, which policy applies, which state handles
// enforce it, and the retry node — and attempt count — the call runs under. The
// manager only publishes: matching depends on the operation, so it happens here,
// together with the first-use materialization of the organs.
//
// Timeout and retry come from the policy document alone; a call cannot declare
// them. The caller keeps only its context deadline, which can tighten the
// policy's budget but never extend it.
//
// The fast path — keys already built — is entirely lock-free: one atomic load per
// capability plus one table lookup. The lock is taken only to materialize, and a
// materialization re-checks under it, so a key is built exactly once however many
// calls race onto it.
func (e *Executor) planFor(op chain.Operation) callPlan {
	trs := e.mgr.timeoutRetrySettings()
	tr, _ := trs.findRule(op)
	br, brOn := e.mgr.breakerSettings().findRule(op)
	rr, rateOn := e.mgr.rateLimitSettings().findRule(op)

	plan := callPlan{
		timeout:  tr.Timeout,
		retry:    tr.Retry,
		attempts: tr.Retry.MaxRetries + 1,
		budget:   e.retryBudget(trs.labelBudget(op.Service)),
	}
	if brOn {
		plan.breaker = e.loadBreaker(ruleKey(op.Service, br.ActionMatch), br)
	}
	if rateOn && rr.Policy.RateLimit > 0 {
		plan.rate = e.rateLimiter(ruleKey(op.Service, rr.ActionMatch), rr)
		plan.ratePolicy = rr.Policy
	}
	return plan
}

// ruleKey is the state key of a rule: derived from its SELECTOR, not its index,
// so reordering a document never moves a breaker or a rate budget, and two
// replicas serving different document versions still agree on the key. The
// service-scope key is the bare label.
func ruleKey(service string, m matcher.ActionMatch) string {
	switch {
	case m.Exact == matcher.ServiceScopeAction:
		return service
	case m.Exact != "":
		return service + "#exact:" + m.Exact
	default:
		return service + "#prefix:" + m.Prefix
	}
}

// loadBreaker returns this call's breaker, materializing its key on first use.
// The fast path is one atomic load plus one table lookup; the lock is taken only
// to materialize, and a materialization re-checks under it, so a key is built
// exactly once however many calls race onto it.
func (e *Executor) loadBreaker(key string, r BreakerRule) *circuitbreaker.Breaker {
	st := e.breaker.Load()
	if v, ok := st.keys.Load(key); ok {
		return v.(*breakerCell).b
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if v, ok := st.keys.Load(key); ok {
		return v.(*breakerCell).b // built while we waited
	}
	cell := &breakerCell{}
	if r.BreakerPolicy.Enabled() {
		b, err := circuitbreaker.New(r.BreakerPolicy.Spec())
		if err != nil {
			// The policy is validated before publication; reaching this means the
			// validator and the materializer drifted.
			panic("resilience: validated policy produced an invalid breaker: " + err.Error())
		}
		cell.b = b
	}
	st.keys.Store(key, cell)
	return cell.b
}

// resetBreakerIfChanged replaces the breaker table with an empty one when this
// label's breaker content moved between prev and next. The comparison is by
// CONTENT, not by the fact that a push ran, so a push that changes nothing keeps
// the live state. A call that already loaded the old slot keeps it until it
// returns.
func (e *Executor) resetBreakerIfChanged(prev, next *BreakerSettings) {
	if breakerGen(e.service, prev) == breakerGen(e.service, next) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.breaker.Store(&breakerSlot{})
}

// rateLimiter returns the counter this call charges, built by the limiterFactory from
// the kind its rule names. The counter lives for the executor's lifetime, not
// for a pushed document, so a rule change updates it in place (numerics travel
// per call); a change of KIND replaces it, because the limiterFactory changed.
// Counters whose rule disappeared are collected by the manager's push.
func (e *Executor) rateLimiter(key string, r RateLimitRule) ratelimit.Limiter {
	kind := r.Policy.Limiter
	if v, ok := e.localLimiters.Load(key); ok {
		old := v.(*limiterEntry)
		if old.kind == kind {
			return old.limiter
		}
	}
	// The scope is the RULE identity, not the label: two rules of one service
	// keep independent budgets on any backend (a shared store included). The
	// service-scope rule's key IS the label, so the common case stays stable
	// across document edits.
	lim, err := e.mgr.limiterFactory.NewLimiter(kind, key)
	if err != nil {
		// Apply built this kind through the same call; a failure now is the
		// backend's own.
		panic(err)
	}
	entry := &limiterEntry{kind: kind, limiter: lim}
	actual, _ := e.localLimiters.LoadOrStore(key, entry)
	if old, ok := actual.(*limiterEntry); ok && old.kind != kind {
		e.localLimiters.Store(key, entry)
		return lim
	}
	return actual.(*limiterEntry).limiter
}

// deleteUnusedLimiters drops counters whose keys the label no longer resolves to
// under s — one key per rule naming it. It is idempotent and never touches an
// active key, so a push that changes nothing leaves every live budget counting.
// A call that already holds a dropped counter keeps using it until it returns;
// the next call rebuilds a fresh one if the key is active again.
func (e *Executor) deleteUnusedLimiters(s RateLimitSettings) {
	active := make(map[string]struct{})
	for i := range s.Services {
		g := &s.Services[i]
		if g.Service != e.service {
			continue
		}
		for j := range g.Rules {
			active[ruleKey(e.service, g.Rules[j].ActionMatch)] = struct{}{}
		}
		break // first group naming the label wins, as in findRule
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.localLimiters.Range(func(k, _ any) bool {
		if _, ok := active[k.(string)]; !ok {
			e.localLimiters.Delete(k)
		}
		return true
	})
}

// retryBudget returns the label-level retry budget, rebuilt only when the
// declared capacity changes. The capacity is read from the raw limit each call;
// the channel is the only state kept, and its cap is the budget itself.
func (e *Executor) retryBudget(capacity int) chan struct{} {
	if st := e.retry.Load(); st != nil && cap(st.sem) == capacity {
		return st.sem
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if st := e.retry.Load(); st != nil && cap(st.sem) == capacity {
		return st.sem
	}
	var sem chan struct{}
	if capacity > 0 {
		sem = make(chan struct{}, capacity)
	}
	st := &retrySlot{sem: sem}
	e.retry.Store(st)
	return sem
}

// refusedRound records a gate's verdict as the call's round. A refusal IS a
// decision the chain made — "we never sent it, and here is why" — so it is
// recorded like any other round, with no peer (the downstream was never touched,
// and with it no writer — the gate refused before the round's fn ran), the gate's
// own word in ErrorType and a zero Duration (no downstream time was spent). Its
// Start is the instant the gate decided, which is what keeps the gap-derivation
// of the retry cost honest even across a refusal.
//
// Recording it here, where the decision is made, is what makes refused volume
// visible in the attempt-level series instead of silently missing; it is also
// why the loop TERMINATES on a refusal rather than retrying it — a retry of an
// admission-refused call (rate limited today) is exactly the load amplification
// the gate exists to prevent.
func refusedRound(rec *chain.Recorder, err error) {
	rec.AddRound(chain.Round{
		Start:     time.Now(),
		Status:    chain.ClassifyStatus(err),
		ErrorType: chain.ClassifyErrorType(err),
		Error:     err,
	})
}

// Execute is the whole pipeline, in blocks:
//
//	deadline  → retry plan → rate gate → breaker gate → attempts → settlement
//
// It is the only outward seam. Everything it touches is either per-composition
// (next) or per-label state held by the executor itself.
func (e *Executor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	plan := e.planFor(op)

	// What governed this call, for the record.
	rec.Timeout = plan.timeout

	// One deadline bounds every round and the backoff between them; it only
	// tightens the caller's own context. A zero budget leaves the context
	// untouched.
	callCtx := ctx
	cancel := func() {}
	if plan.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, plan.timeout)
	}
	defer cancel()

	// Rate limit: every attempt is a real downstream request. The first charge
	// runs before the breaker gate so a limited call never spends a half-open
	// trial permit.
	if err := chargeRate(callCtx, plan.rate, plan.ratePolicy); err != nil {
		refusedRound(rec, err)
		return err
	}

	// Breaker: admits the LOGICAL call once and settles exactly one sample on
	// every return path, so a retried call cannot amplify one failure into N. A
	// nil breaker admits everything.
	var (
		err        error
		attemptDur time.Duration
		ran        bool
		permit     circuitbreaker.Permit
	)
	admitted := true
	if plan.breaker != nil {
		permit, admitted = plan.breaker.Allow()
	}
	if !admitted {
		refusedRound(rec, chain.ErrCircuitOpen)
		return chain.ErrCircuitOpen
	}
	if plan.breaker != nil {
		defer func() { settleBreaker(plan.breaker, permit, err, attemptDur, ran, ctx) }()
	}

	// Attempts.
	budget := retryBudget{sem: plan.budget}
	for i := range plan.attempts {
		if i > 0 {
			// The budget gates retries only: an exhausted cap stops the call
			// instead of piling one more retry on a downstream everyone is
			// already retrying.
			if rerr := budget.take(); rerr != nil {
				refusedRound(rec, rerr)
				return rerr
			}
			if rerr := chargeRate(callCtx, plan.rate, plan.ratePolicy); rerr != nil {
				budget.release()
				refusedRound(rec, rerr)
				return rerr
			}
		}

		start := time.Now()
		err = chain.Next(e.next, callCtx, op, rec, fn)
		attemptDur = time.Since(start)
		ran = true
		// The attempt's own cost and outcome, recorded where only this loop can
		// see them. Status is classified HERE: the words are this package's, and
		// an emitter that had to classify would depend on this package — the
		// reverse import direction.
		rec.AddRound(chain.Round{
			Start:     start,
			Duration:  attemptDur,
			Status:    chain.ClassifyStatus(err),
			ErrorType: chain.ClassifyErrorType(err),
			Error:     err,
		})
		if err == nil || callCtx.Err() != nil || !ShouldRetry(err) || i == plan.attempts-1 {
			break
		}
		if !timeutil.Sleep(callCtx, Backoff(plan.retry, i)) {
			break // backoff interrupted by ctx cancellation
		}
		// The slot covers the attempt, not the backoff sleep. The sleep itself is
		// not recorded: the next round's Start minus this round's end IS the
		// retry cost, derived instead of stored twice.
		budget.release()
	}
	budget.release()
	return err
}

// chargeRate charges one attempt. A store failure is not a budget decision: the
// call proceeds rather than turning an unreachable counter backend into an
// outage.
func chargeRate(ctx context.Context, limiter ratelimit.Limiter, policy ratelimit.Policy) error {
	if limiter == nil {
		return nil
	}
	admitted, err := limiter.Allow(ctx, policy, 1)
	if err != nil {
		log.Warn(ctx, log.TagAppDef, "resilience: rate-limit counters unavailable, allowing call",
			log.Err(err))
		return nil
	}
	if !admitted {
		return chain.ErrRateLimited
	}
	return nil
}

// settleBreaker records the one sample a logical call owes. A panic before fn
// returns leaves ran false and settles a failure; a caller abort and an error
// the caller classifies as not evidence (an injected business fault, a 4xx-style
// adapter error) carry no verdict rather than opening the circuit against a
// healthy downstream.
func settleBreaker(br *circuitbreaker.Breaker, permit circuitbreaker.Permit, err error, dur time.Duration, ran bool, ctx context.Context) {
	switch {
	case !ran:
		br.Record(permit, false, dur)
	case err == nil:
		br.Record(permit, true, dur)
	case ctx.Err() != nil && errors.Is(err, ctx.Err()), !CountsAsFailure(err):
		// A caller abort is not evidence against the downstream. Only the PARENT
		// context counts: a timeout derived from the policy budget means the unit
		// did not serve within the budget and remains a failure, while a caller
		// that walked away says nothing about the unit's health.
		br.RecordIgnored(permit)
	default:
		br.Record(permit, false, dur)
	}
}

// retryBudget is the in-flight retry slot one attempt holds.
type retryBudget struct {
	sem  chan struct{}
	held bool
}

func (b *retryBudget) take() error {
	if b.sem == nil {
		return nil
	}
	select {
	case b.sem <- struct{}{}:
		b.held = true
		return nil
	default:
		return chain.ErrRetryExceeded
	}
}

func (b *retryBudget) release() {
	if !b.held {
		return
	}
	<-b.sem
	b.held = false
}

// Backoff returns the sleep before the retry following the (0-indexed) attempt-th
// failure, applying exponential growth and jitter per p. Both engines call it so
// the math cannot drift.
//
// The schedule grows [Retry.InitialInterval] by
// [Retry.Multiplier] each step up to
// [Retry.MaxInterval], then decorrelates concurrent callers by ±
// [Retry.RandomizationFactor]. A zero InitialInterval yields 0 (no
// backoff), so a node that sets only MaxRetries retries back to back.
func Backoff(p Retry, attempt int) time.Duration {
	if p.InitialInterval <= 0 {
		return 0
	}
	mult := p.Multiplier
	if mult <= 0 {
		mult = 1
	}
	d := float64(p.InitialInterval)
	for range attempt {
		d *= mult
		if p.MaxInterval > 0 && d > float64(p.MaxInterval) {
			d = float64(p.MaxInterval)
			break
		}
	}
	if p.RandomizationFactor > 0 {
		// ± factor*d: center the jitter on d so the average backoff stays d.
		delta := d * p.RandomizationFactor
		d = d + (rand.Float64()*2-1)*delta
	}
	return time.Duration(d)
}
