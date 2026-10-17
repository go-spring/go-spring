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

// client_executor.go is the bundled "default" engine for OUTBOUND calls and the
// organs only it composes: the client executor (bulkhead + timeout + retry
// around the shared rate-limit and circuit-breaker organs), the retry
// classification and backoff that pace its attempts, the circuit-breaker organ,
// and the [Fallback] degradation helper. Every capability the outbound side
// offers — rate limiting, circuit breaking, isolation, retry, timeout — is
// implemented here, in one place.
//
// The breaker and rate-limit organs belong to this outbound engine
// (server_executor.go); the breaker lives in this file beside its
// primary builder, and Go's package scope makes the file location immaterial to
// the server. The rate limiter keeps its own file (ratelimit.go) because its
// vocabulary is direction-neutral.

package resilience

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/log"
	"go-spring.org/stdlib/timeutil"
)

// warnedNonIdempotent records the services already told that a configured retry
// was dropped because their operations are non-idempotent, so the warning is one
// per service rather than one per call.
var warnedNonIdempotent sync.Map

// defaultExecutor protects ONE service — the one it was built for — so that a
// misbehaving downstream does not trip protection for any other.
//
// # A stable shell over one replaceable generation
//
// The executor is the object the client holds for life, so it is never rebuilt.
// What it MATERIALIZES is: the per-key breaker, bulkhead and rate budget, plus the
// retry budget, are rebuilt wholesale whenever the config generation behind its
// label moves. [defaultExecutor.snapshot] compares the generation its current
// [execState] was built from against the one its source resolves and replaces the
// lot when they differ — nothing about a generation survives into the next. That
// is the deliberate contract (see policy_source.go), and it is what lets a config
// change be a plain construction instead of a per-organ merge of "what can be
// kept".
//
// A call already in flight when the swap happens keeps the execState it loaded:
// its outcome lands on organs nobody will consult again, which costs one lost
// sample and nothing else.
//
// # State is per key
//
// Protective state is partitioned by the identity key the source resolves (see
// [CallPolicy.Key]): a matched rule's identity, or the service name under the
// default policy. A call governed by one rule gets its own breaker and bulkhead,
// so a downstream that misbehaves for one traffic class cannot trip another's
// protection. Metadata selects a rule but never becomes a key — a key must stay
// bounded by the config. The retry budget is the one organ that is NOT per key:
// it is a per-label guard by design.
type defaultExecutor struct {
	service string

	// fixed is the construction-time policy. It governs every call of an executor
	// built outside a manager — one whose driver does not implement
	// [PolicySourceSetter] — and is otherwise only the pre-first-call value.
	fixed ClientPolicy

	// limiter is nil in the ordinary case, and then the rate-limit stage counts
	// in rate; non-nil means every call charges that store instead, under this
	// executor's service as the scope.
	limiter ClusterLimiter

	// mu guards cur and the maps inside it: a generation is (re)built and a key's
	// state is first created under it. It is held only while something is being
	// built, and the source is never called under it.
	mu  sync.Mutex
	cur *execState

	// source is the manager, when there is one. It is installed before the
	// executor is shared ([PolicySourceSetter]) and read on every call; nil means
	// the construction-time policy governs for ever. It must NOT call back into
	// this executor.
	source PolicySource

	listener BreakerEventListener
}

// execState is one generation of the executor's materialized protection: every
// organ whose knobs come from the config, built together and replaced together.
// The maps fill lazily — a key's state appears when a call first resolves to it —
// but the generation is fixed: state built here never migrates to another
// generation, and another generation's state never leaks in.
type execState struct {
	gen      uint64
	states   map[string]serviceState
	rates    map[string]rateState
	retrySem chan struct{}
}

// newClientExecutor builds the executor for service. Nothing is materialized
// here — not even the retry budget, which now belongs to the first generation:
// the organs are built on first use by [defaultExecutor.snapshot], because they
// must see the breaker listener, and [WrapClientExecutor] attaches that after this
// returns.
func newClientExecutor(service string, p ClientPolicy, limiter ClusterLimiter) *defaultExecutor {
	return &defaultExecutor{service: service, fixed: p, limiter: limiter}
}

// callPolicy asks the source what this call runs under. Without a source both the
// policy and the key are the executor's own (construction-time policy, service
// key) — the behavior every caller had before rules could match on metadata, and
// the only behavior available to a driver outside the manager's reach.
func (e *defaultExecutor) callPolicy(op chain.Operation) CallPolicy {
	if e.source == nil {
		key := op.Service
		if key == "" {
			key = e.service
		}
		return CallPolicy{Policy: e.fixed, Key: key, Label: e.fixed}
	}
	// The executor knows the service it was built for; a call that names none must
	// still resolve against that service's rules, so fill it in before resolving —
	// otherwise the source sees an empty service and every undeclared call would
	// fall to Default.
	//
	// This is a fallback for old/direct callers, NOT a substitute for stating the
	// service: op is a local COPY (the declaration travels by value), so the fill
	// is invisible to every layer above this executor — the fault layer among them
	// — and a client that leaves it blank will not match their service-scoped
	// rules. Clients are expected to name the service on the operation they build.
	if op.Service == "" {
		op.Service = e.service
	}
	// Resolve OUTSIDE the lock: the source reads the manager's published snapshot
	// lock-free, and calling it while holding e.mu would nest a call-path lock
	// inside one the manager's build path takes.
	c := e.source.Resolve(op)
	if c.Key == "" {
		c.Key = op.Service
	}
	return c
}

// snapshot returns the state, rate budget, policy and retry budget a call runs
// on, materializing them on first use of the key. Deferring the build to the
// first call is load-bearing: [WrapClientExecutor] attaches the breaker listener
// while the executor is still private (see [BreakerEventListenerSetter]), so the
// breaker must be created against a listener that is already in place.
//
// The four are returned together, under one lock, so a call never sees a
// half-built set — and so the retry budget is read under the same lock that
// publishes it, rather than off a field a rebuild could be writing.
func (e *defaultExecutor) snapshot(op chain.Operation) (serviceState, rateState, ClientPolicy, chan struct{}) {
	c := e.callPolicy(op)

	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.cur
	if st == nil || st.gen != c.Gen {
		// A new generation: everything the previous one materialized is dropped,
		// its organs included. See the type doc for why nothing is carried over.
		st = &execState{
			gen:    c.Gen,
			states: map[string]serviceState{},
			rates:  map[string]rateState{},
		}
		// RetryBudget is process-level state read from the LABEL-level policy — a
		// per-call node's budget is deliberately ignored, since an unshared budget
		// would not be a process-wide cap.
		if c.Label.Retry.RetryBudget > 0 {
			st.retrySem = make(chan struct{}, c.Label.Retry.RetryBudget)
		}
		e.cur = st
	}
	s, ok := st.states[c.Key]
	if !ok {
		s = e.buildState(c.Key, c.Policy)
		st.states[c.Key] = s
		st.rates[c.Key] = newRateState(c.Policy.RateSpec())
	}
	return s, st.rates[c.Key], c.Policy, st.retrySem
}

// SetPolicySource installs src, the manager this executor asks for the policy of
// each call (see [PolicySourceSetter]). It is the seam that lets governance rules
// match on a call's metadata AND lets the executor follow the config generation:
// with a source installed, a call whose metadata satisfies a rule runs under that
// rule's policy and state, while a call no rule matches resolves to the default
// under the service key — the same protection it had before. It must be set
// before the executor is shared, and the source must not call back into the
// executor.
func (e *defaultExecutor) SetPolicySource(src PolicySource) {
	e.source = src
}

// buildState builds the stages whose state belongs to this executor alone,
// against its policy and listener. Callers must hold mu. The knobs reach each
// stage in the stage's own vocabulary (see [breakerSpec], [RateSpec]), so the
// organs are shared with the inbound engine without either policy being
// privileged.
func (e *defaultExecutor) buildState(key string, p ClientPolicy) serviceState {
	state := serviceState{}
	spec := p.breakerSpec()
	if spec.active() {
		open := p.OpenDuration
		if open <= 0 {
			open = 5 * time.Second
		}
		state.breaker = newCircuitBreaker(spec, open, key, e.listener)
	}
	return state
}

// SetBreakerEventListener attaches l so this executor's breaker — the one it
// builds for its service — emits state transitions. It satisfies
// [BreakerEventListenerSetter].
//
// It must be called before the executor is shared — that is, before its first
// [defaultExecutor.Execute], which is when the breaker is built. [WrapClientExecutor]
// attaches the listener while the executor is still being constructed inside
// [ClientManager.buildLocked], so the executor never escapes unobserved.
func (e *defaultExecutor) SetBreakerEventListener(l BreakerEventListener) {
	e.listener = l
}

// serviceState is the state of the stages that belong to ONE service or route and
// one executor alone — the breaker and the bulkhead. Both engines hold one (see
// [defaultExecutor], [serverExecutor]); nothing about it knows a direction.
type serviceState struct {
	breaker *circuitBreaker
	sem     chan struct{}

	// queueFor is how long acquire may WAIT for a semaphore slot before
	// rejecting with [chain.ErrBulkheadFull] (see [ClientPolicy.BulkheadQueueDuration]);
	// 0 keeps the non-blocking reject.
	queueFor time.Duration
}

// acquireBulkhead takes one bulkhead slot, waiting up to state.queueFor (and no
// longer than ctx allows) when the bulkhead is momentarily full. It reports
// false — and consumes nothing — when no slot was obtained.
func (s serviceState) acquireBulkhead(ctx context.Context) bool {
	if s.sem == nil {
		return true
	}
	if s.queueFor <= 0 {
		select {
		case s.sem <- struct{}{}:
			return true
		default:
			return false
		}
	}
	qctx, cancel := context.WithTimeout(ctx, s.queueFor)
	defer cancel()
	select {
	case s.sem <- struct{}{}:
		return true
	case <-qctx.Done():
		return false
	}
}

func (s serviceState) releaseBulkhead() { <-s.sem }

// releaseRetryBudget returns a held retry-budget slot to the channel it was
// taken from (no-op for nil/-1). Releasing into the CAPTURED channel — not
// whichever budget the executor is on now — is what makes a generation swap
// safe mid-flight: a call that took a slot from the old generation's budget
// returns it there, and the swap stays race-free exactly like the bulkhead's.
func releaseRetryBudget(sem chan struct{}, held int) {
	if sem == nil || held < 0 {
		return
	}
	<-sem
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
// why the loop TERMINATES on a refusal rather than retrying it — a retry of a
// rate-limited or bulkhead-full call is exactly the load amplification the gate
// exists to prevent. A nil recorder is a no-op.
func refusedRound(rec *chain.Recorder, err error) {
	rec.AddRound(chain.Round{
		Start:     time.Now(),
		Status:    chain.ClassifyStatus(err),
		ErrorType: chain.ClassifyErrorType(err),
		Err:       err,
	})
}

func (e *defaultExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	s, rate, policy, retrySem := e.snapshot(op)

	// Record the policy this call ran under — the rule that was matched and the
	// effective knobs it resolved to, after a hot-fix override, not the compiled
	// defaults — so "what governed this call" is answerable from the call's own
	// record. A nil recorder means the caller wants no record.
	if rec != nil {
		rec.Timeout = policy.Timeout
	}

	// Timeout bounds the whole call — every round and the backoff between them —
	// as ONE deadline applied here and inherited by everything below. It only
	// tightens the caller's own context; there is no separate per-round bound, so
	// the rounds share this budget rather than each getting their own.
	callCtx := ctx
	if policy.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, policy.Timeout)
		defer cancel()
	}

	// eff is the retry node this call runs under: the policy's node when the
	// governance center pushed a schedule (it wins whole — a pushed value is how
	// an operator hot-fixes a live incident), otherwise the call's own node. When
	// neither declares one there is no retry and the call runs exactly once.
	eff := EffectiveRetry(policy.Retry, op.Retry)
	attempts := eff.MaxRetries + 1
	// A non-idempotent call is run once whatever the schedule says: its retry
	// would repeat the side effect — a second email, a second published message —
	// and no downstream stage can undo that. The client declares it on the call's
	// retry node (see [chain.RetryPolicy.NonIdempotent]); dropping the retry here
	// is the framework acting on that declaration, and it says so once per target,
	// because a silently ignored rule is worse than one that was never written.
	if attempts > 1 {
		if eff.NonIdempotent {
			attempts = 1
			if _, seen := warnedNonIdempotent.LoadOrStore(e.service, struct{}{}); !seen {
				log.Warn(ctx, log.TagAppDef,
					"resilience: retries suppressed — operations are non-idempotent, "+
						"so a retry would repeat the side effect rather than the attempt",
					log.String("service", e.service))
			}
		}
	}
	var err error
	var attemptDur time.Duration // duration of the last run attempt, for slow-call counting
	ran := false                 // whether at least one attempt actually invoked fn this call
	budgetHeld := -1             // attempt index currently holding a retry-budget slot
	var budgetSem chan struct{}  // the channel that slot was taken from
	for i := range attempts {
		// The retry budget gates attempts after the first: when the executor-
		// wide in-flight retry cap is exhausted, this call stops with
		// [chain.ErrRetryBudgetExceeded] instead of piling one more retry onto a
		// downstream that is already being retried by everyone.
		if sem := retrySem; sem != nil && i > 0 {
			select {
			case sem <- struct{}{}:
				budgetSem, budgetHeld = sem, i
			default:
				refusedRound(rec, chain.ErrRetryBudgetExceeded)
				return chain.ErrRetryBudgetExceeded
			}
		}
		// The rate-limit stage charges every attempt — each attempt is a real
		// downstream request the budget exists to bound. Where the count goes
		// depends on what this executor was built with: a store, when the driver
		// was handed one (shared with every other executor spending it, and with
		// every replica when that store is over a shared backend), or otherwise a
		// bucket of its own, charging this executor's service. A store failure is
		// not a budget decision: the call proceeds rather than turning an
		// unreachable counter backend into an outage.
		if lerr := allowRate(callCtx, rate, policy.RateSpec(), e.service, e.limiter); lerr != nil {
			if !errors.Is(lerr, chain.ErrRateLimited) {
				log.Warn(callCtx, log.TagAppDef, "resilience: rate-limit counters unavailable, allowing call",
					log.Err(lerr))
			} else {
				refusedRound(rec, chain.ErrRateLimited)
				releaseRetryBudget(budgetSem, budgetHeld)
				return chain.ErrRateLimited
			}
		}
		if s.breaker != nil && !s.breaker.allow() {
			// Breaker rejected the attempt. If an earlier attempt already ran
			// this call — e.g. it won the half-open trial permit, failed, and a
			// retry now finds the gate spent — fold that outcome in once so a
			// won trial is never left unresolved (which would stick the breaker
			// half-open with a consumed permit). When nothing ran yet there is
			// no sample to record.
			if ran {
				s.breaker.record(err == nil, attemptDur)
			}
			refusedRound(rec, chain.ErrCircuitOpen)
			releaseRetryBudget(budgetSem, budgetHeld)
			return chain.ErrCircuitOpen
		}

		start := time.Now()
		err = fn(callCtx)
		attemptDur = time.Since(start)
		ran = true
		// The attempt's own cost and outcome, recorded where only this loop can
		// see them. The status is classified HERE, not at the emitter: the words
		// are this package's (the sentinels above), and an emitter that had to
		// classify would have to depend on this package — the reverse of the
		// import direction. The raw error rides along for the failure log.
		rec.AddRound(chain.Round{
			Start:     start,
			Duration:  attemptDur,
			Status:    chain.ClassifyStatus(err),
			ErrorType: chain.ClassifyErrorType(err),
			Err:       err,
		})
		if err == nil {
			break
		}
		// A cancelled/expired deadline (caller cancel or [ClientPolicy.Timeout])
		// ends the loop before the predicate is consulted: there is no budget left
		// for another attempt regardless of why the last one failed.
		if callCtx.Err() != nil {
			break
		}
		if !ShouldRetry(err) {
			break
		}
		if i == attempts-1 {
			break // last attempt — no backoff sleep after it
		}
		backoff := Backoff(eff, i)
		if !timeutil.Sleep(callCtx, backoff) {
			break // backoff interrupted by ctx cancellation
		}
		// The sleep is not recorded as a field of its own: the next round's
		// [chain.Round.Start] is an absolute instant, so this gap is exactly
		// Start[i+1] - (Start[i] + Duration[i]) — the retry policy's own cost,
		// derived by the reader instead of stored twice.
		releaseRetryBudget(budgetSem, budgetHeld)
		budgetHeld = -1 // the slot covers the attempt, not the backoff sleep
	}
	releaseRetryBudget(budgetSem, budgetHeld)
	// The breaker measures the outcome of one protected call, not each retry
	// attempt. Recording once per logical Execute — rather than once per
	// attempt inside the loop — stops a retrying client from amplifying a
	// single downstream failure into N breaker samples, which would trip the
	// circuit far faster than the configured ErrorThreshold / error-rate
	// implies (the "resilience on => breaker trips instantly" symptom). The
	// rate limiter above intentionally still charges per attempt, because each
	// attempt is a real downstream request the limiter exists to bound.
	if s.breaker != nil {
		s.breaker.record(err == nil, attemptDur)
	}
	return err
}

func (e *defaultExecutor) Close() error { return nil }

// EffectiveRetry resolves the retry node one call runs under, from the two
// sources: the policy node (a governance center's push, or a directly
// constructed policy) and the call's own node ([chain.Operation.Retry]). It is
// the ONE place the two are merged, so the engines cannot drift.
//
// Precedence is whole-node: when the policy node declares a retry schedule it
// wins entirely, because a pushed value is how an operator hot-fixes a live
// incident and must not be second-guessed by the code it is fixing. When it
// declares nothing the call's node applies, so a center that pushes no retry
// leaves the caller in charge.
//
// NonIdempotent is the exception to whole-node precedence: it is OR-ed across
// BOTH nodes. It is a property of the CALL, not of the schedule, and a governance
// override that replaced the whole node would otherwise silently drop a caller's
// non-idempotent declaration — re-enabling retry on a call whose repetition
// doubles a side effect. The safety flag survives regardless of who wins.
func EffectiveRetry(policy, call chain.RetryPolicy) chain.RetryPolicy {
	eff := call
	if policy.HasSchedule() {
		eff = policy
	}
	eff.NonIdempotent = policy.NonIdempotent || call.NonIdempotent
	return eff
}

// ShouldRetry reports whether err ought to trigger another attempt, for a retry
// node p. The classification is shared by every engine so retry decisions
// cannot drift: an error implementing [Retryable] opts itself in or out
// explicitly (the caller knows the concrete error type — a non-idempotent write
// opts out); every other non-nil error retries. It reads no field of p and
// exists as a replacement for the historical policy method, now that retry is
// declared per call rather than on the policy.
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var r Retryable
	if errors.As(err, &r) {
		return r.Retryable()
	}
	return true
}

// Backoff returns the sleep before the retry following the (0-indexed) attempt-th
// failure, applying exponential growth and jitter per p. Both engines call it so
// the math cannot drift.
//
// The schedule grows [chain.RetryPolicy.InitialInterval] by
// [chain.RetryPolicy.Multiplier] each step up to
// [chain.RetryPolicy.MaxInterval], then decorrelates concurrent callers by ±
// [chain.RetryPolicy.RandomizationFactor]. A zero InitialInterval yields 0 (no
// backoff), so a node that sets only MaxRetries retries back to back.
func Backoff(p chain.RetryPolicy, attempt int) time.Duration {
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

// BreakerStrategy selects how the circuit breaker counts failures. Each [Driver]
// maps it onto its own primitives; the default driver implements both directly,
// sentinel-golang translates them onto its ErrorCount / ErrorRatio rules.
type BreakerStrategy string

const (
	// BreakerConsecutive trips after [ClientPolicy.ErrorThreshold] failures in a row
	// (any success resets the count). It is the default when BreakerStrategy is
	// empty, matching the historical behavior.
	BreakerConsecutive BreakerStrategy = "consecutive"
	// BreakerErrorRate trips when the failure ratio over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.ErrorRateThreshold], once at least [ClientPolicy.MinRequests]
	// requests have been observed in the window. Use it for high-throughput
	// services where a burst of failures or a steady partial-failure rate is
	// more meaningful than a consecutive run.
	BreakerErrorRate BreakerStrategy = "error-rate"
	// BreakerSlowCall trips when the SLOW-call ratio over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.SlowCallRateThreshold], a call being slow at or above
	// [ClientPolicy.SlowCallDurationThreshold] whether it failed or succeeded — a
	// downstream that never errors but answers slowly is unhealthy too. Unlike the
	// slow-call pair under [BreakerErrorRate] (where slow successes fold into the
	// error rate), this strategy counts ONLY slowness. It is the neutral
	// counterpart of sentinel's SlowRequestRatio and OpenSergo's SlowRequestRatio
	// circuit-breaker strategy.
	BreakerSlowCall BreakerStrategy = "slow-call"
	// BreakerErrorCount trips when the failure COUNT over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.ErrorThreshold], once at least [ClientPolicy.MinRequests]
	// requests have been observed in the window. Unlike [BreakerConsecutive] (which
	// counts an unbounded run and resets on any success), it is a windowed count —
	// the neutral counterpart of sentinel's ErrorCount and OpenSergo's ErrorCount
	// circuit-breaker strategy, so a policy written against either maps exactly.
	BreakerErrorCount BreakerStrategy = "error-count"
)

// BreakerState, BreakerEventListener and BreakerEventListenerSetter — the
// breaker's state and its observation contract — are defined in breaker.go,
// beside the engine that owns them. The observe wrappers import this package to
// read them; this package never imports the observer.

// circuitBreaker supports four strategies over one open/half-open state machine:
//
//   - consecutive: trips after threshold failures in a row (a success resets
//     the count), the historical behavior.
//   - error-rate: trips when bad/total over a rolling window reaches a ratio,
//     once a minimum sample has accumulated. A sample is "bad" when it failed
//     OR (with the slow-call pair set) it succeeded but took at least
//     SlowCallDurationThreshold — a downstream that never errors but answers
//     slowly trips the same window.
//   - slow-call: trips when the slow-call ratio over the same window reaches
//     [breakerSpec.slowRate], slowness counting failures as well (a failed
//     attempt is at least as unhealthy as a slow one), mirroring sentinel's
//     SlowRequestRatio.
//   - error-count: trips when the failure count over the window reaches
//     [breakerSpec.threshold], the windowed counterpart of consecutive.
//
// Half-open admits a bounded number of trials (HalfOpenRequests, default 1):
// the gate is a permit channel, not a bool flag, so concurrent callers arriving
// right after cool-down cannot exceed the admitted count (the prior flag-based
// implementation had exactly that race). All N succeeding closes the circuit;
// the first failure re-opens it.
type circuitBreaker struct {
	strategy BreakerStrategy
	openFor  time.Duration

	// consecutive strategy
	threshold int

	// error-rate strategy
	rateThreshold float64 // bad/total ratio that trips
	minRequests   int     // minimum sample before tripping
	window        time.Duration

	// slow-call counting (error-rate strategy): a success at or above
	// slowThreshold counts as bad when slowRateThreshold is set.
	slowThreshold time.Duration
	slowRate      float64

	// half-open inbound count (0/1 = the historical single trial)
	halfOpenN int

	mu       sync.Mutex
	failures int // consecutive counter
	openedAt time.Time
	halfOpen chan struct{} // non-nil while a single trial permit is offered

	// rolling-window counters for the error-rate strategy. fails counts
	// failures, slows counts slow-but-successful attempts; either ratio (or
	// both) can trip.
	win       time.Duration
	curStart  time.Time
	total     int
	fails     int
	slows     int
	prevTotal int
	prevFails int
	prevSlows int

	// half-open trial tracking: how many trials are admitted and how many have
	// succeeded so far; the circuit closes when ok reaches the admitted count.
	halfOpenOK int

	// event emission
	service  string               // label passed to the listener
	listener BreakerEventListener // nil = no listener; fixed at construction
}

// breakerSpec is the circuit breaker's own vocabulary: the thresholds one breaker
// is built for. Both directions assemble it from their policy (see
// [ClientPolicy.breakerSpec] / [ServerPolicy.breakerSpec]), so the one organ with
// real internal state stays shared without either policy being privileged and
// without a direction ever reaching the other's breaker.
type breakerSpec struct {
	strategy      BreakerStrategy
	threshold     int           // consecutive: failures in a row that trip
	rateThreshold float64       // error-rate: bad/total ratio that trips
	minRequests   int           // error-rate: minimum sample before tripping
	window        time.Duration // error-rate: the rolling interval counted over
	slowThreshold time.Duration // a success at or above this counts as bad
	slowRate      float64
	halfOpenN     int // trials admitted half-open; 0 and 1 both mean one
}

// active reports whether any breaker strategy is configured. Both directions
// consult it, so a breaker is built under exactly the same condition on either
// side. error-count shares ErrorThreshold with consecutive, and slow-call
// activates through the slow-call pair, so the conditions are unchanged.
func (s breakerSpec) active() bool {
	return s.threshold > 0 || s.rateThreshold > 0 || (s.slowThreshold > 0 && s.slowRate > 0)
}

// resolveBreakerStrategy picks the strategy a breaker should apply. An explicit
// strategy (error-rate, slow-call or error-count) wins; a slow-call pair
// without an explicit strategy also selects error-rate (slow successes are
// counted by the rate window — the historical behavior); otherwise the
// consecutive default. Shared by both directions so neither can disagree with
// the other on what an unset strategy means.
func resolveBreakerStrategy(explicit BreakerStrategy, slowThreshold time.Duration, slowRate float64) BreakerStrategy {
	switch explicit {
	case BreakerErrorRate, BreakerSlowCall, BreakerErrorCount:
		return explicit
	}
	if slowThreshold > 0 && slowRate > 0 {
		return BreakerErrorRate
	}
	return BreakerConsecutive
}

// newCircuitBreaker builds a breaker for one service or route from spec. open is
// how long the circuit stays open; listener is the executor's listener captured
// at construction, and it never changes for the life of the breaker, so no
// synchronization is needed to read it.
func newCircuitBreaker(spec breakerSpec, open time.Duration, service string, listener BreakerEventListener) *circuitBreaker {
	c := &circuitBreaker{
		strategy:      spec.strategy,
		openFor:       open,
		threshold:     spec.threshold,
		rateThreshold: spec.rateThreshold,
		minRequests:   spec.minRequests,
		service:       service,
		listener:      listener,
	}
	if spec.slowThreshold > 0 && spec.slowRate > 0 {
		c.slowThreshold = spec.slowThreshold
		c.slowRate = spec.slowRate
	}
	if spec.halfOpenN > 1 {
		c.halfOpenN = spec.halfOpenN
	}
	if c.usesWindow() {
		if c.minRequests <= 0 {
			c.minRequests = 1
		}
		if spec.window > 0 {
			c.win = spec.window
		} else {
			c.win = time.Second
		}
		c.curStart = time.Now()
	}
	return c
}

// usesWindow reports whether the strategy counts over the rolling window (as
// opposed to consecutive's unbounded run).
func (c *circuitBreaker) usesWindow() bool {
	return c.strategy != BreakerConsecutive
}

// refresh re-aims the breaker at spec/open in place: thresholds, window length,
// slow-call pair and half-open trial count are updated on the live organ while
// its counters and its open/half-open state survive — a policy push does not
// reset a circuit that is already tripped. The one discontinuity is a STRATEGY
// switch (consecutive ↔ window-based): the two counting models cannot translate
// each other's counters, so they reset.
func (c *circuitBreaker) refresh(spec breakerSpec, open time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	strategySwitch := c.strategy != spec.strategy
	c.strategy = spec.strategy
	c.openFor = open
	c.threshold = spec.threshold
	c.rateThreshold = spec.rateThreshold
	c.minRequests = spec.minRequests
	c.slowThreshold, c.slowRate = 0, 0
	if spec.slowThreshold > 0 && spec.slowRate > 0 {
		c.slowThreshold = spec.slowThreshold
		c.slowRate = spec.slowRate
	}
	c.halfOpenN = spec.halfOpenN
	if c.usesWindow() {
		if c.minRequests <= 0 {
			c.minRequests = 1
		}
		if spec.window > 0 {
			c.win = spec.window
		} else if c.win <= 0 {
			c.win = time.Second
		}
		c.curStart = time.Now()
	}
	if strategySwitch {
		c.failures, c.total, c.fails, c.slows = 0, 0, 0, 0
		c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
	}
}

// allow reports whether a request may proceed given the current breaker state.
func (c *circuitBreaker) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openedAt.IsZero() {
		return true // closed
	}
	if time.Since(c.openedAt) < c.openFor {
		return false // open, cooling down
	}
	// Cool-down elapsed: open a half-open gate with the configured number of
	// trial permits if none yet, then take one. A concurrent caller finding the
	// gate empty is treated as still-open — no more than halfOpenN trials (1 by
	// default) are admitted per cool-down.
	if c.halfOpen == nil {
		n := c.halfOpenN
		if n <= 0 {
			n = 1
		}
		c.halfOpenOK = 0
		c.halfOpen = make(chan struct{}, n)
		for range n {
			c.halfOpen <- struct{}{}
		}
		c.notifyLocked(BreakerOpen, BreakerHalfOpen)
	}
	select {
	case <-c.halfOpen:
		return true
	default:
		return false
	}
}

// record folds an attempt's outcome back into the breaker state. dur is how
// long the recorded attempt took; it feeds slow-call counting (ignored when the
// slow-call pair is unset).
func (c *circuitBreaker) record(success bool, dur time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	slow := c.slowThreshold > 0 && dur >= c.slowThreshold
	// Half-open trial resolves first: its outcome closes or re-opens the
	// circuit regardless of strategy. With N admitted trials, every one must
	// succeed to close; the first failure re-opens immediately.
	if c.halfOpen != nil {
		if success {
			c.halfOpenOK++
			n := c.halfOpenN
			if n <= 0 {
				n = 1
			}
			if c.halfOpenOK >= n {
				c.closeLocked()
				c.notifyLocked(BreakerHalfOpen, BreakerClosed)
			}
			return
		}
		// Trial failed: re-open the cool-down window.
		c.halfOpen = nil
		c.openedAt = time.Now()
		c.notifyLocked(BreakerHalfOpen, BreakerOpen)
		return
	}
	if c.usesWindow() {
		c.recordRate(success, slow)
		return
	}
	c.recordConsecutive(success)
}

func (c *circuitBreaker) recordConsecutive(success bool) {
	if success {
		c.failures = 0
		c.openedAt = time.Time{}
		return
	}
	c.failures++
	if c.failures >= c.threshold {
		if c.openedAt.IsZero() {
			c.notifyLocked(BreakerClosed, BreakerOpen)
		}
		c.openedAt = time.Now()
	}
}

// recordRate advances the rolling-window counters and trips the breaker when
// the failure ratio or the slow-call ratio reaches its threshold with enough
// samples. It uses the same weighted two-window estimate as the standalone
// slidingWindow limiter.
func (c *circuitBreaker) recordRate(success, slow bool) {
	now := time.Now()
	elapsed := now.Sub(c.curStart)
	if elapsed >= c.win {
		if elapsed >= 2*c.win {
			c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
		} else {
			c.prevTotal, c.prevFails, c.prevSlows = c.total, c.fails, c.slows
		}
		c.total, c.fails, c.slows = 0, 0, 0
		c.curStart = now
		elapsed = 0
	}
	c.total++
	if !success {
		c.fails++
		// Under the slow-call strategy slowness is the whole signal, and a failed
		// attempt is at least as unhealthy as a slow one — mirror sentinel's
		// RT-based SlowRequestRatio by counting failures as slow too.
		if c.strategy == BreakerSlowCall && slow {
			c.slows++
		}
	} else if slow {
		c.slows++
	}
	weight := float64(c.win-elapsed) / float64(c.win)
	estimateTotal := float64(c.prevTotal)*weight + float64(c.total)
	estimateFails := float64(c.prevFails)*weight + float64(c.fails)
	estimateSlows := float64(c.prevSlows)*weight + float64(c.slows)
	trip := false
	if c.total+c.prevTotal >= c.minRequests && estimateTotal > 0 {
		switch c.strategy {
		case BreakerErrorCount:
			trip = estimateFails >= float64(c.threshold)
		case BreakerSlowCall:
			trip = c.slowRate > 0 && estimateSlows/estimateTotal >= c.slowRate
		default: // BreakerErrorRate: error ratio, with slow successes folded in
			if c.rateThreshold > 0 && estimateFails/estimateTotal >= c.rateThreshold {
				trip = true
			}
			if c.slowRate > 0 && estimateSlows/estimateTotal >= c.slowRate {
				trip = true
			}
		}
	}
	if trip {
		if c.openedAt.IsZero() {
			c.notifyLocked(BreakerClosed, BreakerOpen)
		}
		c.openedAt = now
	}
}

func (c *circuitBreaker) closeLocked() {
	c.failures = 0
	c.openedAt = time.Time{}
	c.halfOpen = nil
	// Reset the windowed counters so a freshly-closed breaker starts clean.
	c.total, c.fails, c.slows = 0, 0, 0
	c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
	c.curStart = time.Now()
}

// notifyLocked emits a from→to transition to the listener, if one is attached.
// Caller holds mu; the listener must not call back into the breaker/executor
// (documented on [BreakerEventListener]).
func (c *circuitBreaker) notifyLocked(from, to BreakerState) {
	if from == to || c.listener == nil {
		return
	}
	c.listener.OnBreakerStateChange(c.service, from, to)
}

// Fallback runs fn through exec and, when the operation is rejected (rate
// limited, circuit open, bulkhead full) or fails after all retries, invokes
// degrade to produce a graceful result instead of surfacing the error. It is
// the degradation stage of the framework and composes with any [chain.Executor]
// regardless of driver: degrade receives the triggering error so it can serve
// cached data for [chain.ErrCircuitOpen] yet propagate a genuine bug, for example.
//
// degrade's own error (or nil) becomes the final result. When exec is nil the
// call is a transparent pass-through: fn runs once and its error, if any, still
// reaches degrade, so wiring stays a no-op until a policy is configured.
//
// The record it hands down describes the CALL that failed — every round, and the
// rejection that triggered the degrade among them. Degradation itself leaves no
// mark on it: a degraded call is a call whose rounds failed and whose caller was
// served something else, and the rounds are the evidence of that. A caller that
// needs to tell degrade's result from a real one has degrade's error, which is
// what this returns.
func Fallback(ctx context.Context, exec chain.Executor, call chain.Operation, rec *chain.Recorder,
	fn func(context.Context) error, degrade func(context.Context, error) error) error {
	var err error
	if exec == nil {
		err = fn(ctx)
	} else {
		err = exec.Execute(ctx, call, rec, fn)
	}
	if err == nil {
		return nil
	}
	return degrade(ctx, err)
}
