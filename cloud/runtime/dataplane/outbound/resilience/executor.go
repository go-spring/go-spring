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
// around the shared rate-limit and circuit-breaker organs), the retry
// classification and backoff that pace its attempts, and the [Fallback]
// degradation helper. Every capability the outbound side offers — rate
// limiting, circuit breaking, isolation, retry, timeout — is composed here.
//
// The breaker and the rate budget are not implemented here: they are the
// direction-neutral organs in go-spring.org/cloud/runtime/dataplane/kernel/breaker and
// go-spring.org/cloud/runtime/dataplane/kernel/ratelimit, shared with inbound admission so both directions
// run the same state machine and counters.

package resilience

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"

	"go-spring.org/cloud/runtime/dataplane/kernel/circuitbreaker"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
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
// What it MATERIALIZES is: the per-key breaker and rate budget, plus the
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
// default policy. A call governed by one rule gets its own breaker and rate
// budget, so a downstream that misbehaves for one traffic class cannot trip
// another's protection. Metadata selects a rule but never becomes a key — a key must stay
// bounded by the config. The retry budget is the one organ that is NOT per key:
// it is a per-label guard by design.
type defaultExecutor struct {
	service string

	// fixed is the construction-time policy. It governs every call of an executor
	// built outside a manager — one whose driver does not implement
	// [PolicySourceSetter] — and is otherwise only the pre-first-call value.
	fixed Policy

	// limiter is nil in the ordinary case, and then the rate-limit stage counts
	// in rate; non-nil means every call charges that store instead, under this
	// executor's service as the scope.
	limiter ratelimit.ClusterLimiter

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

	// observe is this executor's own state plane: it publishes the breakers and
	// budgets the executor owns (see observe.go). It is registered lazily on the
	// first call because the global meter provider may be installed after this
	// executor is built, and it is unregistered by Close.
	observeOnce sync.Once
	observeReg  metric.Registration
	closed      bool
}

// execState is one generation of the executor's materialized protection: every
// organ whose knobs come from the config, built together and replaced together.
// The maps fill lazily — a key's state appears when a call first resolves to it —
// but the generation is fixed: state built here never migrates to another
// generation, and another generation's state never leaks in.
type execState struct {
	gen      uint64
	states   map[string]serviceState
	rates    map[string]ratelimit.Budget
	retrySem chan struct{}
}

// newExecutor builds the executor for service. Nothing is materialized
// here — not even the retry budget, which now belongs to the first generation:
// the organs are built on first use by [defaultExecutor.snapshot]. The
// executor's own state observer registers on that first call too (see
// observe.go), because the global meter provider may be installed after this
// returns.
func newExecutor(service string, p Policy, limiter ratelimit.ClusterLimiter) *defaultExecutor {
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
// first call costs nothing to composition: an organ is built from the policy
// alone, and the executor's own state observer reads whatever generation is
// current at collection time.
//
// The four are returned together, under one lock, so a call never sees a
// half-built set — and so the retry budget is read under the same lock that
// publishes it, rather than off a field a rebuild could be writing.
func (e *defaultExecutor) snapshot(op chain.Operation) (serviceState, ratelimit.Budget, Policy, chan struct{}) {
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
			rates:  map[string]ratelimit.Budget{},
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
		s = e.buildState(c.Policy)
		st.states[c.Key] = s
		st.rates[c.Key] = ratelimit.NewBudget(c.Policy.RateSpec())
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
// against its policy. Callers must hold mu. The knobs reach each stage in the
// stage's own vocabulary (see [circuitbreaker.Spec], [ratelimit.Spec]), so the
// organs are shared with the inbound engine without either policy being
// privileged.
func (e *defaultExecutor) buildState(p Policy) serviceState {
	if !p.BreakerEnabled() {
		return serviceState{} // nil breaker: the OFF stage, like a nil *Bulkhead
	}
	b, err := circuitbreaker.New(p.BreakerSpec())
	if err != nil {
		// The policy is validated before publication; reaching this means the
		// validator and the materializer drifted.
		panic("resilience: validated policy produced an invalid breaker: " + err.Error())
	}
	return serviceState{breaker: b}
}

// serviceState is the state of the stages that belong to ONE service or route and
// one executor alone. Today that is the breaker; nothing about it knows a
// direction, and the breaker inside is the shared
// go-spring.org/cloud/runtime/dataplane/kernel/circuitbreaker.
type serviceState struct {
	breaker *circuitbreaker.Breaker
}

// releaseRetryBudget returns a held retry-budget slot to the channel it was
// taken from (no-op for nil/-1). Releasing into the CAPTURED channel — not
// whichever budget the executor is on now — is what makes a generation swap
// safe mid-flight: a call that took a slot from the old generation's budget
// returns it there, and the swap stays race-free.
func releaseRetryBudget(sem chan struct{}, held int) {
	if sem == nil || held < 0 {
		return
	}
	<-sem
}

// CallerAborted reports whether err is the CALLER's own cancellation rather
// than the downstream's verdict. Only the parent context counts: a timeout
// derived from the policy budget means the unit did not serve within the budget
// and remains a failure, while a caller that walked away says nothing about the
// unit's health and must carry no verdict. Engines use it to exclude such a call
// from the breaker's sample.
func CallerAborted(ctx context.Context, err error) bool {
	return ctx.Err() != nil && errors.Is(err, ctx.Err())
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
// the gate exists to prevent. A nil recorder is a no-op.
func refusedRound(rec *chain.Recorder, err error) {
	rec.AddRound(chain.Round{
		Start:     time.Now(),
		Status:    chain.ClassifyStatus(err),
		ErrorType: chain.ClassifyErrorType(err),
		Err:       err,
	})
}

func (e *defaultExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	e.readyObserve()
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

	// The rate-limit stage charges every attempt — each attempt is a real
	// downstream request the budget exists to bound. The first charge happens
	// before the breaker gate so a rate-limited call never spends a half-open
	// trial permit. A store failure is not a budget decision: the call proceeds
	// rather than turning an unreachable counter backend into an outage.
	if admitted, lerr := allowRate(callCtx, rate, policy.RateSpec(), e.service, e.limiter); lerr != nil {
		log.Warn(callCtx, log.TagAppDef, "resilience: rate-limit counters unavailable, allowing call",
			log.Err(lerr))
	} else if !admitted {
		refusedRound(rec, chain.ErrRateLimited)
		return chain.ErrRateLimited
	}

	// The breaker admits the LOGICAL call once, not each retry: the permit
	// settles exactly one sample, which is what the breaker counts. Recording
	// once per Execute also stops a retrying client from amplifying one
	// downstream failure into N breaker samples (the "resilience on => breaker
	// trips instantly" symptom). The deferred Record runs on every return path,
	// so an admitted call is never left unsettled — a half-open trial whose
	// outcome vanished would pin the round until its deadline.
	if br := s.breaker; br != nil {
		permit, ok := br.Allow()
		if !ok {
			refusedRound(rec, chain.ErrCircuitOpen)
			return chain.ErrCircuitOpen
		}
		defer func() {
			// One sample per LOGICAL call. A panic before fn returns leaves ran
			// false: settle it as a failure rather than leaving a half-open trial
			// unspent. A caller cancellation (or the caller's own deadline) is not
			// evidence about the unit, so it carries no verdict; a timeout from our
			// own policy budget is evidence, and stays a failure.
			switch {
			case !ran:
				br.Record(permit, false, attemptDur)
			case err == nil:
				br.Record(permit, true, attemptDur)
			case CallerAborted(ctx, err):
				br.RecordIgnored(permit)
			default:
				br.Record(permit, false, attemptDur)
			}
		}()
	}

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
		// Every attempt past the first is charged here; the first was charged
		// before the breaker gate.
		if i > 0 {
			if admitted, lerr := allowRate(callCtx, rate, policy.RateSpec(), e.service, e.limiter); lerr != nil {
				log.Warn(callCtx, log.TagAppDef, "resilience: rate-limit counters unavailable, allowing call",
					log.Err(lerr))
			} else if !admitted {
				refusedRound(rec, chain.ErrRateLimited)
				releaseRetryBudget(budgetSem, budgetHeld)
				return chain.ErrRateLimited
			}
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
		// A cancelled/expired deadline (caller cancel or [Policy.Timeout])
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
	return err
}

// Close releases the executor's state observer and marks it closed. A later
// Execute still runs (governance degrades to the zero policy), but it will not
// re-register the observer.
func (e *defaultExecutor) Close() error {
	e.closeObserve()
	return nil
}

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
