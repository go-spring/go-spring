// executor_default.go is the bundled "default" executor — the implementation
// the "default" driver (executor.go) builds for one service from a [ClientPolicy]. It
// holds that service's breaker/bulkhead state and threads each call through the
// stages with retry + timeout; the rate-limit stage counts in a budget of its
// own unless the driver was handed a [Counters] to count in instead. The stage
// primitives it composes live in executor_default_breaker.go, ratelimit.go and
// executor_default_retry.go.

package resilience

import (
	"context"
	"errors"
	"go-spring.org/cloud/chain"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/stdlib/timeutil"
)

// warnedNonIdempotent records the services already told that a configured retry
// was dropped because their operations are non-idempotent, so the warning is one
// per service rather than one per call.
var warnedNonIdempotent sync.Map

// defaultExecutor protects ONE service — the one it was built for — so that a
// misbehaving downstream does not trip protection for any other. Being bound to
// its service is what makes every stage's state single: one breaker, one
// bulkhead, one retry budget, one rate bucket (or, when the driver was handed a
// store, that store's budget for this service — which is what carries a limit
// past the process when the store is over a shared backend).
type defaultExecutor struct {
	service string
	policy  ClientPolicy

	// counters is nil in the ordinary case, and then the rate-limit stage counts
	// in rate; non-nil means every call charges that store instead, under this
	// executor's service as the scope.
	counters Counters

	// mu guards state, rate and built, which the first call establishes as a set.
	mu       sync.Mutex
	built    bool
	state    serviceState
	rate     rateState
	listener BreakerEventListener

	// retrySem caps in-flight RETRY attempts across the executor (the
	// retry-budget stage); nil when RetryBudget is 0.
	retrySem chan struct{}
}

// newClientExecutor builds the executor for service. Only the policy and the
// retry budget are established here; the state that a call runs on (breaker,
// bulkhead, rate budget) is built on first use by [defaultExecutor.snapshot],
// because it must see the breaker listener — and [WrapClientExecutor] attaches that
// after this returns.
func newClientExecutor(service string, p ClientPolicy, counters Counters) *defaultExecutor {
	e := &defaultExecutor{service: service, policy: p, counters: counters}
	if p.RetryBudget > 0 {
		e.retrySem = make(chan struct{}, p.RetryBudget)
	}
	return e
}

// snapshot returns the state, rate budget and policy a call should run on,
// building the state on first use. Deferring the build to the first call is
// load-bearing: [WrapClientExecutor] attaches the breaker listener while the
// executor is still private (see [BreakerEventListenerSetter]), so the breaker
// must be created against a listener that is already in place.
//
// The three are returned together, under one lock, so a call never sees a
// half-built set.
func (e *defaultExecutor) snapshot() (serviceState, rateState, ClientPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.built {
		e.state = e.buildState()
		e.rate = newRateState(e.policy.RateSpec())
		e.built = true
	}
	return e.state, e.rate, e.policy
}

// buildState builds the stages whose state belongs to this executor alone,
// against its policy and listener. Callers must hold mu. The knobs reach each
// stage in the stage's own vocabulary (see [breakerSpec], [RateSpec]), so the
// organs are shared with the inbound engine without either policy being
// privileged.
func (e *defaultExecutor) buildState() serviceState {
	state := serviceState{}
	spec := e.policy.breakerSpec()
	if spec.active() {
		open := e.policy.OpenDuration
		if open <= 0 {
			open = 5 * time.Second
		}
		state.breaker = newCircuitBreaker(spec, open, e.service, e.listener)
	}
	if e.policy.MaxConcurrent > 0 {
		// A buffered channel is a non-blocking counting semaphore: a full buffer
		// means the bulkhead is at capacity, so excess calls are rejected rather
		// than queued.
		state.sem = make(chan struct{}, e.policy.MaxConcurrent)
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
// [Manager.build], so the executor never escapes unobserved.
func (e *defaultExecutor) SetBreakerEventListener(l BreakerEventListener) {
	e.listener = l
}

// serviceState is the state of the stages that belong to ONE service or route and
// one executor alone — the breaker and the bulkhead. Both engines hold one (see
// [defaultExecutor], [serverExecutor]); nothing about it knows a direction.
type serviceState struct {
	breaker *circuitBreaker
	sem     chan struct{}
}

// retryBudgetRelease returns a held retry-budget slot (no-op for -1/nil).
func (e *defaultExecutor) retryBudgetRelease(held int) {
	if e.retrySem == nil || held < 0 {
		return
	}
	<-e.retrySem
}

func (e *defaultExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	s, rate, policy := e.snapshot()

	// The bulkhead bounds concurrent in-flight calls to the service; one slot
	// is held for the whole Execute (retries included) and released when done.
	if s.sem != nil {
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		default:
			return chain.ErrBulkheadFull
		}
	}

	// MaxDuration caps the whole call across retries. It only tightens the
	// caller's own context; per-attempt Timeout is applied inside runOnce
	// against budgetCtx, so the effective per-attempt bound is min(Timeout,
	// remaining MaxDuration).
	budgetCtx := ctx
	if policy.MaxDuration > 0 {
		var cancel context.CancelFunc
		budgetCtx, cancel = context.WithTimeout(ctx, policy.MaxDuration)
		defer cancel()
	}

	// rec is the per-call accumulator the outer emitter reads. It is nil when no
	// governance layer installed one (a bare executor used directly); every write
	// below is nil-safe, so the loop records unconditionally.
	rec := observability.RecorderFrom(budgetCtx)

	attempts := policy.MaxRetries + 1
	// A non-idempotent operation is run once whatever the policy says: its retry
	// would repeat the side effect — a second email, a second published message —
	// and no downstream stage can undo that. The client declared it on the
	// operation (see [observability.Operation.NonIdempotent]); dropping a
	// configured retry here is the framework acting on that declaration, and it
	// says so once per service, because a silently ignored rule is worse than one
	// that was never written.
	if attempts > 1 {
		if op, ok := observability.OperationFrom(ctx); ok && op.NonIdempotent {
			attempts = 1
			if _, seen := warnedNonIdempotent.LoadOrStore(e.service, struct{}{}); !seen {
				log.Warn(ctx, log.TagAppDef, log.String("service", e.service),
					log.Msg("resilience: retries suppressed — operations are non-idempotent, "+
						"so a retry would repeat the side effect rather than the attempt"))
			}
		}
	}
	var err error
	var attemptDur time.Duration // duration of the last run attempt, for slow-call counting
	ran := false                 // whether at least one attempt actually invoked fn this call
	budgetHeld := -1             // attempt index currently holding a retry-budget slot
	for i := range attempts {
		// The retry budget gates attempts after the first: when the executor-
		// wide in-flight retry cap is exhausted, this call stops with
		// [chain.ErrRetryBudgetExceeded] instead of piling one more retry onto a
		// downstream that is already being retried by everyone.
		if e.retrySem != nil && i > 0 {
			select {
			case e.retrySem <- struct{}{}:
				budgetHeld = i
			default:
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
		if lerr := allowRate(budgetCtx, rate, policy.RateSpec(), e.service, e.counters); lerr != nil {
			if !errors.Is(lerr, chain.ErrRateLimited) {
				log.Warn(budgetCtx, log.TagAppDef, log.Err(lerr),
					log.Msg("resilience: rate-limit counters unavailable, allowing call"))
			} else {
				e.retryBudgetRelease(budgetHeld)
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
			e.retryBudgetRelease(budgetHeld)
			return chain.ErrCircuitOpen
		}

		start := time.Now()
		err = e.runOnce(budgetCtx, fn)
		attemptDur = time.Since(start)
		ran = true
		// The attempt's own cost and outcome, recorded where only this loop can
		// see them. The status is classified HERE, not at the emitter: the words
		// are this package's (the sentinels above), and an emitter that had to
		// classify would have to depend on this package — the reverse of the
		// import direction. The raw error rides along for the failure log.
		rec.AddAttempt(attemptDur, observability.ClassifyStatus(err), err)
		if err == nil {
			break
		}
		// A cancelled/Expired budget (caller cancel or MaxDuration) ends the
		// loop before the predicate is consulted: there is no budget left for
		// another attempt regardless of why the last one failed.
		if budgetCtx.Err() != nil {
			break
		}
		if !e.policy.ShouldRetry(err) {
			break
		}
		if i == attempts-1 {
			break // last attempt — no backoff sleep after it
		}
		backoff := e.policy.Backoff(i)
		if !timeutil.Sleep(budgetCtx, backoff) {
			break // backoff interrupted by ctx cancellation
		}
		// Time spent sleeping between attempts is the retry policy's own cost:
		// the downstream never saw it, and it is the one thing a caller-visible
		// duration cannot decompose without this record.
		rec.AddBackoff(backoff)
		e.retryBudgetRelease(budgetHeld)
		budgetHeld = -1 // the slot covers the attempt, not the backoff sleep
	}
	e.retryBudgetRelease(budgetHeld)
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

// runOnce applies the per-attempt timeout, if any, around fn. The ctx it
// receives is already bounded by the Execute-level MaxDuration budget.
func (e *defaultExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.AttemptTimeout <= 0 {
		return fn(ctx)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, e.policy.AttemptTimeout)
	defer cancel()
	return fn(attemptCtx)
}

func (e *defaultExecutor) Close() error { return nil }
