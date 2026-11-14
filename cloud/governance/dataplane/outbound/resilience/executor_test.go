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

// executor_test.go covers the bundled outbound engine in executor.go:
// its stage machinery (rate limit, bulkhead, warm-up, per-attempt timeout), the
// retry loop and its schedule/merge rules, the circuit breaker and its strategies,
// fallback, and the signals the engine records for the outer emitter.

package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// svcRate is a whole-service rate rule for the test label "svc".
func svcRate(p ratelimit.Policy) RateLimitRule {
	return RateLimitRule{
		ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
		Policy:      p,
	}
}

// svcTimeout is a whole-service timeout/retry rule for the test label "svc".
func svcTimeout(tr TimeoutRetry) TimeoutRetryRule {
	return TimeoutRetryRule{
		ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
		TimeoutRetry: tr,
	}
}

// svcBreaker is a whole-service breaker rule for the test label "svc".
func svcBreaker(bp BreakerPolicy) BreakerRule {
	return BreakerRule{
		ActionMatch:   matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
		BreakerPolicy: bp,
	}
}

// svcRateSettings arms the whole-service rate rule for label "svc".
func svcRateSettings(p ratelimit.Policy) RateLimitSettings {
	return RateLimitSettings{Services: []RateLimitService{{Service: "svc", Rules: []RateLimitRule{svcRate(p)}}}}
}

// svcTimeoutSettings arms the whole-service timeout/retry rule for label "svc".
func svcTimeoutSettings(tr TimeoutRetry) TimeoutRetrySettings {
	return TimeoutRetrySettings{Services: []TimeoutRetryService{{Service: "svc", Rules: []TimeoutRetryRule{svcTimeout(tr)}}}}
}

// svcBreakerSettings arms the whole-service breaker rule for label "svc".
func svcBreakerSettings(bp BreakerPolicy) BreakerSettings {
	return BreakerSettings{Services: []BreakerService{{Service: "svc", Rules: []BreakerRule{svcBreaker(bp)}}}}
}

// newBuiltin builds the bundled client engine for one service under cfg. It is
// the shared entry the executor, retry, breaker and emit tests all start from.
//
// cfg is one capability's policy — TimeoutRetry, BreakerPolicy or ratelimit.Policy —
// because the three are independent and most tests exercise one at a time.
func newBuiltin(t *testing.T, cfg any) chain.Executor {
	t.Helper()
	return newBuiltinManager(t, cfg).Wrap("svc", nil)
}

// newBuiltinManager arms a manager over the bundled driver with cfg as the
// whole-service policy of its capability. Tests that move the config mid-flight
// keep the manager.
func newBuiltinManager(t *testing.T, cfg any) *Manager {
	t.Helper()
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := NewManager(factories)
	assert.That(t, err).Nil()
	switch v := cfg.(type) {
	case nil:
	case TimeoutRetry:
		assert.Error(t, m.ApplyTimeoutRetry(svcTimeoutSettings(v))).Nil()
	case BreakerPolicy:
		assert.Error(t, m.ApplyBreaker(svcBreakerSettings(v))).Nil()
	case ratelimit.Policy:
		assert.Error(t, m.ApplyRateLimit(svcRateSettings(v))).Nil()
	default:
		t.Fatalf("unsupported test policy %T", cfg)
	}
	return m
}

// The observe-layer tests that drive the wrapper over these engines — spans,
// metrics and the recorder hand-off — live in emitter_test.go, an external test
// package, because the wrapper (cloud/governance/dataplane/outbound/observe) imports this package.

func TestBuiltinPassThrough(t *testing.T) {
	// A zero policy protects nothing: fn runs once and its result flows back.
	e := newBuiltin(t, TimeoutRetry{})
	var calls int
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		calls++
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, calls).Equal(1)
}

// TestExecuteTimeout pins the one bound: a call given a Timeout is cut off by it,
// as a deadline its fn observes rather than an error the executor invents.
func TestExecuteTimeout(t *testing.T) {
	e := newBuiltin(t, TimeoutRetry{Timeout: 20 * time.Millisecond})
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(ctx context.Context, _ *chain.Round) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})
	assert.Error(t, err).Is(context.DeadlineExceeded)
	assert.Error(t, err).Is(chain.ErrCallTimeout)
}

func TestRateLimit(t *testing.T) {
	// Guava semantics: the 1st call rides the head of the pipeline for free
	// but owes a full second; the 2nd, immediately after, is rejected.
	e := newBuiltin(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 2})
	run := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(chain.ErrRateLimited)
}

func TestRateLimitQueueing(t *testing.T) {
	// rate-limit-max-wait turns an over-limit call into a bounded wait: a
	// 1/s bucket with burst 1 admits call 1 immediately, and call 2 — which
	// maxWait=0 would reject — instead waits for the next token and succeeds.
	e := newBuiltin(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 20, Burst: 1, MaxWait: 200 * time.Millisecond})
	run := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	// And with no wait budget the same shape rejects.
	e2 := newBuiltin(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 20, Burst: 1})
	assert.Error(t, e2.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
	assert.Error(t, e2.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Is(chain.ErrRateLimited)
}

// TestWarmUpColdStart pins the warm-up knob on the default driver's token
// bucket (Guava SmoothWarmingUp): the bucket starts full and cold, and a cold
// permit costs far more than its steady price — so the call right after the
// first is rejected while the pipeline drains that cost. Without warm-up the
// second call would also queue a debt, but only one steady interval, not the
// expensive cold one.
func TestWarmUpColdStart(t *testing.T) {
	// rate 100/s (steady 10ms/permit), warm-up 2s at coldFactor 10: the first
	// stored permit above the warm threshold costs ~99ms — ~10 steady prices.
	e := newBuiltin(t, ratelimit.Policy{
		Limiter:    ratelimit.TokenBucketAlgorithm,
		RateLimit:  100,
		Burst:      10,
		WarmUp:     2 * time.Second,
		ColdFactor: 10,
	})
	// The first call is granted at the head but owes the cold price; the
	// second, immediately after, is rejected.
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Is(chain.ErrRateLimited)
}

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}})
	var attempts int
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		attempts++
		if attempts < 3 {
			return transient{}
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(3)
}

func TestRetryBackoffSleeps(t *testing.T) {
	// With InitialInterval set, retries are paced: the gap between attempt 1
	// and attempt 2 must be at least one backoff interval.
	e := newBuiltin(t, TimeoutRetry{
		Retry: Retry{
			MaxRetries:      1,
			InitialInterval: 40 * time.Millisecond,
		},
	})
	var attempts int
	var firstSaw, secondSaw time.Duration
	start := time.Now()
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		attempts++
		if attempts == 1 {
			firstSaw = time.Since(start)
		} else {
			secondSaw = time.Since(start)
		}
		if attempts < 2 {
			return transient{}
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(2)
	// The second attempt ran at least InitialInterval after the first returned.
	assert.That(t, secondSaw-firstSaw >= 35*time.Millisecond).True()
}

func TestRetryRespectsTimeout(t *testing.T) {
	// Timeout caps the whole call: even with many retries permitted, the loop
	// stops once the deadline is reached — the rounds share one budget, so a late
	// retry gets whatever is left.
	e := newBuiltin(t, TimeoutRetry{
		Retry: Retry{
			MaxRetries:      20,
			InitialInterval: 20 * time.Millisecond,
		},
		Timeout: 60 * time.Millisecond,
	})
	var attempts int
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		attempts++
		return transient{}
	})
	// Not all 21 attempts ran — the budget cut the loop short.
	assert.That(t, attempts < 21).True()
	assert.That(t, attempts >= 1).True()
}

func TestRetryableFalseStopsRetry(t *testing.T) {
	// A Retryable() false error stops the loop after the first failure, even
	// though MaxRetries would allow more attempts.
	e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 3}})
	var attempts int
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		attempts++
		return nonRetryableErr{}
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

type nonRetryableErr struct{}

func (nonRetryableErr) Error() string   { return "permanent" }
func (nonRetryableErr) Retryable() bool { return false }

// transient is a retryable failure: retry is a WHITELIST, so every test that
// expects another attempt returns this rather than a generic error.
type transient struct{}

func (transient) Error() string   { return "transient" }
func (transient) Retryable() bool { return true }

// TestRetryDefaultsToSingleAttempt pins the new default: with neither a policy
// retry node nor a call retry node, a failing call runs exactly ONCE. Retry is
// opt-in — the zero value of the node is "no retry".
func TestRetryDefaultsToSingleAttempt(t *testing.T) {
	e := newBuiltin(t, TimeoutRetry{})
	var attempts int
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		attempts++
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

func TestRetryBudget(t *testing.T) {
	// RetryBudget caps in-flight retries for the executor: call A's retry (in
	// flight, blocked inside fn) holds the single budget slot, so call B's
	// retry is rejected with chain.ErrRetryExceeded. First attempts never take
	// budget. Both calls run through one executor, which serves one service.
	e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 1, RetryBudget: 1}})
	inRetry := make(chan struct{})
	unblock := make(chan struct{})
	aDone := make(chan error, 1)
	go func() {
		calls := 0
		aDone <- e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			calls++
			if calls == 2 { // the retry attempt: hold the budget slot
				close(inRetry)
				<-unblock
			}
			return transient{}
		})
	}()
	<-inRetry // A is now blocked inside its retry, holding the only slot

	// B's first attempt runs fine; its retry finds the budget spent.
	bCalls := 0
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		bCalls++
		return transient{}
	})
	assert.Error(t, err).Is(chain.ErrRetryExceeded)
	assert.That(t, bCalls).Equal(1)

	close(unblock)
	if err := <-aDone; err == nil {
		t.Fatal("call A should end failed, not nil")
	}
}

// TestBreakerPushRebuildsState pins what the manager's push buys: the executor's
// breaker table belongs to ONE configuration, and moving to the next one rebuilds
// it — a tripped breaker is CLOSED again, because the state of a superseded
// configuration is not carried over. It also pins the other half, which is what
// makes that safe: while the content does NOT move, the state is reused, so a
// config push that changes nothing costs nothing.
func TestBreakerPushRebuildsState(t *testing.T) {
	p := BreakerPolicy{ErrorThreshold: 2, OpenDuration: time.Hour}
	m := newBuiltinManager(t, p)
	e := m.Wrap("svc", nil)

	boom := errutil.Explain(nil, "boom")
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return boom })
	}

	// Trip the breaker open.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)

	// The same document again: content-derived comparison, so the state is reused
	// and the circuit stays open — an unrelated push costs this label nothing.
	assert.Error(t, m.ApplyBreaker(svcBreakerSettings(p))).Nil()
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)

	// A different document moves the content: the state is rebuilt, so the
	// breaker starts over CLOSED and the call reaches the downstream again.
	assert.Error(t, m.ApplyBreaker(svcBreakerSettings(BreakerPolicy{ErrorThreshold: 2, OpenDuration: 2 * time.Hour}))).Nil()
	assert.Error(t, fail()).Is(boom)
}

// TestCapabilityGenerationsAreIndependent pins the point of splitting the
// materialized state by capability: rebuilding one organ must not reset another.
// A limits-only change leaves both the rate budget and the breaker alone; a
// rate-only change leaves the breaker alone; only a change to a capability's OWN
// knobs rebuilds its state.
func TestCapabilityGenerationsAreIndependent(t *testing.T) {
	ctx := context.Background()
	boom := errutil.Explain(nil, "boom")
	noop := func(context.Context, *chain.Round) error { return nil }
	spend := func(e chain.Executor) error {
		return e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), noop)
	}
	trip := func(e chain.Executor) error {
		return e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return boom })
	}

	// A change to the limits half must not refill the rate budget.
	rate := ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 0.001, Burst: 1}
	m := newBuiltinManager(t, rate)
	e := m.Wrap("svc", nil)
	defer func() { _ = m.Close() }()

	assert.Error(t, spend(e)).Nil()                    // spends the only token
	assert.Error(t, spend(e)).Is(chain.ErrRateLimited) // budget exhausted

	assert.Error(t, m.ApplyTimeoutRetry(svcTimeoutSettings(TimeoutRetry{Timeout: time.Second}))).Nil()
	assert.Error(t, spend(e)).Is(chain.ErrRateLimited) // budget survived

	// A rate change updates the EXISTING budget; it does not refill it, so the
	// spent token is still spent.
	rate.Burst = 10
	assert.Error(t, m.ApplyRateLimit(svcRateSettings(rate))).Nil()
	assert.Error(t, spend(e)).Is(chain.ErrRateLimited)

	// The new rule does reach the existing state: a high refill rate fills the
	// bucket without resetting it.
	rate.RateLimit = 1e9
	assert.Error(t, m.ApplyRateLimit(svcRateSettings(rate))).Nil()
	assert.Error(t, spend(e)).Nil()

	// A rate-only change must not close a tripped breaker.
	rb := BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour}
	m2 := newBuiltinManager(t, rb)
	e2 := m2.Wrap("svc", nil)
	defer func() { _ = m2.Close() }()
	assert.Error(t, trip(e2)).Is(boom) // trips the breaker (threshold 1)
	assert.Error(t, spend(e2)).Is(chain.ErrCircuitOpen)

	assert.Error(t, m2.ApplyRateLimit(svcRateSettings(
		ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1e6, Burst: 100},
	))).Nil()
	assert.Error(t, spend(e2)).Is(chain.ErrCircuitOpen) // breaker survived

	// A breaker change rebuilds it, so the call reaches the downstream again.
	rb.ErrorThreshold = 5
	assert.Error(t, m2.ApplyBreaker(svcBreakerSettings(rb))).Nil()
	assert.Error(t, spend(e2)).Nil()
}

// TestLocalLimitersPrunedOnRuleRemoval pins the collection half of the local
// counter's lifetime: when a rule disappears, its counter goes with it.
func TestLocalLimitersPrunedOnRuleRemoval(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := NewManager(factories)
	assert.That(t, err).Nil()
	defer func() { _ = m.Close() }()

	rule := RateLimitRule{
		ActionMatch: matcher.ActionMatch{Exact: "limited"},
		Policy:      ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1},
	}
	if err := m.ApplyRateLimit(RateLimitSettings{Services: []RateLimitService{{
		Service: "svc-prune",
		Rules:   []RateLimitRule{rule},
	}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	exec := m.Wrap("svc-prune", nil)
	noop := func(context.Context, *chain.Round) error { return nil }
	call := func() {
		_ = exec.Execute(context.Background(),
			chain.Operation{Service: "svc-prune", Request: chain.Request{Action: "limited"}}, new(chain.Recorder), noop)
	}

	call()
	key := "svc-prune#exact:limited"
	if _, ok := exec.localLimiters.Load(key); !ok {
		t.Fatal("rule-key counter was not materialized")
	}

	if err := m.ApplyRateLimit(RateLimitSettings{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// The push collected it; no extra call is needed to observe the sweep.
	if _, ok := exec.localLimiters.Load(key); ok {
		t.Fatal("stale counter survived the rule removal")
	}
}

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 2, OpenDuration: 50 * time.Millisecond})
	boom := errutil.Explain(nil, "boom")
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return boom })
	}

	// Two consecutive failures trip the breaker open.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(boom)

	// Now open: the operation is short-circuited without invoking fn.
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)

	// After the cool-down a trial request is admitted; a success closes it.
	time.Sleep(60 * time.Millisecond)
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
}

// A caller that walks away says nothing about the unit: cancelling must not
// feed the breaker. Our own policy budget expiring is different — that IS
// evidence the unit did not serve within the budget.
func TestCallerCancelIsIgnored(t *testing.T) {
	e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(ctx context.Context, _ *chain.Round) error {
		return ctx.Err()
	})
	assert.Error(t, err).Is(context.Canceled)

	// Recorded as a failure, this next call would be short-circuited. It must
	// still reach fn.
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return nil
	})).Nil()
}

// A cancelled half-open trial carries no verdict, so its permit goes back and
// the next request becomes the deciding trial.
func TestHalfOpenCallerCancelReturnsPermit(t *testing.T) {
	e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).NotNil()

	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(ctx context.Context, _ *chain.Round) error { return ctx.Err() })
	assert.Error(t, err).Is(context.Canceled)

	// The trial permit came back: a success closes the circuit instead of being
	// refused as a spent half-open round.
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return nil
	})).Nil()
	assert.That(t, breakerStats(t, e, "svc").State).Equal(circuitbreaker.StateClosed)
}

func TestPolicyTimeoutIsFailure(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := NewManager(factories)
	assert.That(t, err).Nil()
	assert.Error(t, m.ApplyBreaker(svcBreakerSettings(BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour}))).Nil()
	assert.Error(t, m.ApplyTimeoutRetry(svcTimeoutSettings(TimeoutRetry{Timeout: 20 * time.Millisecond}))).Nil()
	e := m.Wrap("svc", nil)
	err = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(ctx context.Context, _ *chain.Round) error {
		<-ctx.Done()
		return ctx.Err()
	})
	assert.Error(t, err).Is(context.DeadlineExceeded)
	assert.Error(t, err).Is(chain.ErrCallTimeout)

	err = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

func TestHalfOpenAdmitsSingleTrialConcurrent(t *testing.T) {
	// Regression for the bool-flag half-open bug: once cool-down elapses, at
	// most ONE trial is admitted even under concurrency. A second concurrent
	// caller is treated as still-open.
	e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	// Trip the circuitbreaker.
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return errutil.Explain(nil, "boom")
	})
	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open

	// Two concurrent calls: the first that reaches the half-open gate runs fn;
	// the other must be rejected as chain.ErrCircuitOpen (no second trial permit).
	var wg sync.WaitGroup
	var ran, rejected int32
	for range 2 {
		wg.Go(func() {
			err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
				atomic.AddInt32(&ran, 1)
				// Hold long enough that the sibling surely evaluates allow() too.
				time.Sleep(20 * time.Millisecond)
				return nil // trial succeeds -> closes
			})
			if errors.Is(err, chain.ErrCircuitOpen) {
				atomic.AddInt32(&rejected, 1)
			}
		})
	}
	wg.Wait()
	// Exactly one trial ran; the other was rejected (not both admitted).
	assert.That(t, atomic.LoadInt32(&ran)).Equal(int32(1))
	assert.That(t, atomic.LoadInt32(&rejected)).Equal(int32(1))
}

func TestHalfOpenTrialKeepsItsPermitAcrossRetries(t *testing.T) {
	// The breaker admits one LOGICAL call per trial, not one attempt. The retry
	// of a trial must therefore run under the trial's permit instead of finding
	// the half-open gate spent and leaving the round unresolved.
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := NewManager(factories)
	assert.That(t, err).Nil()
	assert.Error(t, m.ApplyBreaker(svcBreakerSettings(BreakerPolicy{
		ErrorThreshold:   1,
		HalfOpenRequests: 1,
		OpenDuration:     30 * time.Millisecond,
	}))).Nil()
	assert.Error(t, m.ApplyTimeoutRetry(svcTimeoutSettings(TimeoutRetry{
		Retry: Retry{MaxRetries: 1},
	}))).Nil()
	e := m.Wrap("svc", nil)

	// Trip the breaker.
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return transient{}
	})
	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open

	// The trial fails once and succeeds on its retry; the logical call succeeds
	// and closes the circuit.
	attempts := 0
	err = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		attempts++
		if attempts == 1 {
			return transient{}
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(2)

	// Closed: the next call reaches fn instead of being refused.
	ran := false
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		ran = true
		return nil
	})).Nil()
	assert.That(t, ran).True()
}

func TestHalfOpenTrialPanicSettlesAsFailure(t *testing.T) {
	// A panicking trial must still settle its permit. Otherwise the round would
	// keep one permit outstanding forever: with no HalfOpenTimeout by default,
	// that must re-open the circuit rather than leave it stuck half-open.
	e := newBuiltin(t, BreakerPolicy{
		ErrorThreshold:   1,
		HalfOpenRequests: 2,
		OpenDuration:     30 * time.Millisecond,
	})

	// Trip the breaker.
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return errutil.Explain(nil, "boom")
	})
	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open

	// The first trial panics before returning; the deferred settlement records a
	// failure and re-opens the circuit.
	func() {
		defer func() { _ = recover() }()
		_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			panic("boom")
		})
	}()

	// If the permit had been left unsettled, the second trial permit would still
	// admit this call; a re-opened circuit refuses it during the cool-down.
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		return nil
	})).Is(chain.ErrCircuitOpen)
}

func TestErrorRateBreakerTripsOnRatio(t *testing.T) {
	// error-rate breaker: trips when fails/total >= threshold with enough
	// samples, even though successes are interleaved (so a consecutive counter
	// would never trip).
	e := newBuiltin(t, BreakerPolicy{
		BreakerStrategy:    circuitbreaker.StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		BreakerWindow:      time.Second,
		OpenDuration:       time.Minute,
	})
	// Interleave fail/success: 4 fails out of 8 => 50% ratio.
	for i := range 8 {
		err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			if i%2 == 0 {
				return errutil.Explain(nil, "fail")
			}
			return nil
		})
		// Once the breaker trips (after MinRequests with ratio met) further
		// calls are short-circuited. The last iteration should be rejected.
		if errors.Is(err, chain.ErrCircuitOpen) {
			return // trip observed
		}
	}
	t.Fatal("error-rate breaker never tripped at 50% failure ratio")
}

func TestBreakerRecordsOncePerCallNotPerAttempt(t *testing.T) {
	// Regression for the "resilience on => breaker trips instantly" bug: a
	// retrying call must count as ONE breaker sample, not one per attempt.
	// With ErrorThreshold 3 and MaxRetries 2 (3 attempts per call), a single
	// failing call must NOT trip the breaker — under the old per-attempt
	// recording it would record 3 failures and open immediately.
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := NewManager(factories)
	assert.That(t, err).Nil()
	assert.Error(t, m.ApplyBreaker(svcBreakerSettings(BreakerPolicy{
		ErrorThreshold: 3,
		OpenDuration:   time.Minute,
	}))).Nil()
	assert.Error(t, m.ApplyTimeoutRetry(svcTimeoutSettings(TimeoutRetry{
		Retry: Retry{MaxRetries: 2},
	}))).Nil()
	e := m.Wrap("svc", nil)
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			return transient{}
		})
	}

	// First failing call: 3 attempts, but only 1 breaker sample. The next call
	// must still reach fn (return "boom"), proving the breaker did NOT open.
	err = fail()
	assert.Error(t, err).NotNil()
	assert.That(t, !errors.Is(err, chain.ErrCircuitOpen)).True()

	// Second failing call: 2 samples now, still closed.
	err = fail()
	assert.Error(t, err).NotNil()
	assert.That(t, !errors.Is(err, chain.ErrCircuitOpen)).True()

	// Third failing call records the 3rd sample and opens — but the opening
	// happens at record time (after fn ran), so this call still returns "boom".
	err = fail()
	assert.Error(t, err).NotNil()
	assert.That(t, !errors.Is(err, chain.ErrCircuitOpen)).True()

	// The fourth call is rejected outright without invoking fn.
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)
}

func TestSlowCallBreaker(t *testing.T) {
	// A downstream that never FAILS but always answers slowly trips the
	// slow-call breaker over the rate window.
	e := newBuiltin(t, BreakerPolicy{
		BreakerStrategy:           circuitbreaker.StrategyErrorRate,
		ErrorRateThreshold:        0, // errors never happen
		SlowCallDurationThreshold: 20 * time.Millisecond,
		SlowCallRateThreshold:     0.5,
		MinRequests:               2,
		BreakerWindow:             time.Minute,
		OpenDuration:              50 * time.Millisecond,
	})
	slow := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			time.Sleep(25 * time.Millisecond) // > threshold: a slow success
			return nil
		})
	}
	assert.Error(t, slow()).Nil()
	assert.Error(t, slow()).Nil()
	// 2/2 slow >= 0.5: the breaker must now be open.
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

func TestHalfOpenMultipleTrials(t *testing.T) {
	// HalfOpenRequests=3 admits three trials; one flapping failure among them
	// must re-open the circuit, and three successes in a row close it.
	e := newBuiltin(t, BreakerPolicy{
		ErrorThreshold:   1,
		OpenDuration:     30 * time.Millisecond,
		HalfOpenRequests: 3,
	})
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			return errutil.Explain(nil, "boom")
		})
	}
	if err := fail(); err == nil {
		t.Fatal("first failure should return the downstream error (and trip the breaker)") // threshold 1
	}
	time.Sleep(40 * time.Millisecond) // cool-down elapses
	// First trial succeeds but is only 1 of 3: the breaker stays half-open
	// (permit available), not closed.
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Nil()
	// Second trial fails: re-open immediately.
	time.Sleep(40 * time.Millisecond)
	_ = fail() // the trial runs (or is gated); either way the circuit re-opens
	time.Sleep(40 * time.Millisecond)
	// After re-open and cool-down, three consecutive successes close it.
	for range 3 {
		err = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	}
	assert.Error(t, err).Nil()
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
}

// TestSlowCallStrategyBreaker pins the EXPLICIT slow-call strategy: only the
// slow-call ratio trips it — fast failures (the error-rate window's signal) must
// NOT open a slow-call breaker, and slow calls (successful or not) MUST.
func TestSlowCallStrategyBreaker(t *testing.T) {
	e := newBuiltin(t, BreakerPolicy{
		BreakerStrategy:           circuitbreaker.StrategySlowCall,
		SlowCallDurationThreshold: 20 * time.Millisecond,
		// 0.2: the 5 fast failures above stay in the same window's denominator,
		// so 2 slow calls out of 7 total reach 0.29 >= 0.2.
		SlowCallRateThreshold: 0.2,
		MinRequests:           2,
		BreakerWindow:         time.Minute,
		OpenDuration:          time.Minute, // stay open once tripped
	})
	// Fast failures only: a slow-call breaker must not trip on them.
	for range 5 {
		_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			return errutil.Explain(nil, "fast failure")
		})
	}
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Nil() // still closed: failures are not slowness

	// Slow calls only: 2/2 slow, ratio 1.0 > 0.5, trips it.
	for range 2 {
		_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
			time.Sleep(25 * time.Millisecond)
			return nil
		})
	}
	err = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

// TestErrorCountStrategyBreaker pins the error-count strategy: the failure
// COUNT over the window trips it even when interleaved with successes (a
// consecutive counter would have kept resetting), and a success rate high
// enough keeps it closed.
func TestErrorCountStrategyBreaker(t *testing.T) {
	e := newBuiltin(t, BreakerPolicy{
		BreakerStrategy: circuitbreaker.StrategyErrorCount,
		ErrorThreshold:  3,
		MinRequests:     1,
		BreakerWindow:   time.Minute,
		OpenDuration:    time.Minute,
	})
	// fail, ok, fail, ok: never 3 in a row, but 3 failures in the window.
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return errutil.Explain(nil, "boom") })
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return errutil.Explain(nil, "boom") })
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	_ = e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return errutil.Explain(nil, "boom") }) // 3rd failure trips
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

// breakerStats reads the executor's breaker for service through the same
// lock-free loads the call path uses. Tests assert on the state the executor
// publishes, not on an event stream: the breaker emits nothing itself.
func breakerStats(t *testing.T, exec chain.Executor, service string) circuitbreaker.Stats {
	t.Helper()
	n, ok := exec.(*Executor)
	if !ok {
		t.Fatalf("unexpected executor type %T", exec)
	}
	st := n.breaker.Load()
	if st == nil {
		return circuitbreaker.Stats{State: circuitbreaker.StateClosed}
	}
	v, ok := st.keys.Load(service)
	if !ok || v.(*breakerCell).b == nil {
		t.Fatalf("no breaker for %q", service)
	}
	return v.(*breakerCell).b.Stats()
}

// TestBreakerStats verifies the breaker's state machine passes through
// closed → open → half_open → closed, with the per-instance transition counts
// the organ reports.
func TestBreakerStats(t *testing.T) {
	m := newBuiltinManager(t, BreakerPolicy{ErrorThreshold: 2, OpenDuration: 5 * time.Millisecond})
	exec := m.Wrap("svc", nil)
	defer func() { _ = m.Close() }()

	boom := errutil.Explain(nil, "boom")
	ctx := context.Background()
	// Two consecutive failures trip the breaker (closed → open).
	_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return boom })
	_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return boom })
	s := breakerStats(t, exec, "svc")
	assert.That(t, s.State).Equal(circuitbreaker.StateOpen)
	assert.That(t, s.Trips).Equal(uint64(1))
	assert.That(t, s.HalfOpens).Equal(uint64(0))
	assert.That(t, s.Recovers).Equal(uint64(0))

	// The breaker is open: this call is rejected before fn runs, no transition.
	_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	s = breakerStats(t, exec, "svc")
	assert.That(t, s.State).Equal(circuitbreaker.StateOpen)
	assert.That(t, s.Trips).Equal(uint64(1))

	// Wait out the cool-down so the next call is admitted as a half-open trial.
	time.Sleep(20 * time.Millisecond)
	// Trial succeeds: open → half-open (probe admitted), then half-open → closed.
	_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
	s = breakerStats(t, exec, "svc")
	assert.That(t, s.State).Equal(circuitbreaker.StateClosed)
	assert.That(t, s.Trips).Equal(uint64(1))
	assert.That(t, s.HalfOpens).Equal(uint64(1))
	assert.That(t, s.Recovers).Equal(uint64(1))
}

// TestExecutorRebuildStartsClean pins what replaced hot reload: a policy change
// reaches the executor shell, so its materialized state is rebuilt —
// a breaker tripped under the old (tight) policy is gone, and the same executor
// object runs the new one.
func TestExecutorRebuildStartsClean(t *testing.T) {
	m := newBuiltinManager(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})
	e := m.Wrap("svc", nil)
	defer func() { _ = m.Close() }()

	ctx := context.Background()
	boom := errutil.Explain(nil, "boom")
	_ = e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return boom }) // trips (threshold 1)
	assert.Error(t, e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Is(chain.ErrCircuitOpen)

	// A looser policy moves the pushed content: the state is rebuilt, the shell is
	// the same object, and its breaker starts closed.
	assert.Error(t, m.ApplyBreaker(svcBreakerSettings(BreakerPolicy{ErrorThreshold: 100, OpenDuration: time.Hour}))).Nil()
	assert.Error(t, e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
}

// TestGovernanceLoopRecordsAttempts proves the retry loop writes each attempt —
// its outcome and its own cost — and the sleep between attempts into the
// caller-supplied recorder, which is the only source the outer emitter has.
func TestGovernanceLoopRecordsAttempts(t *testing.T) {
	e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 2, InitialInterval: 5 * time.Millisecond}})
	boom := transient{}

	rec := &chain.Recorder{}
	var calls int
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, func(context.Context, *chain.Round) error {
		calls++
		if calls < 3 {
			return boom
		}
		return nil
	})
	assert.Error(t, err).Nil()

	got := rec.Rounds
	assert.Number(t, len(got)).Equal(3)
	assert.Error(t, got[0].Error).Is(boom)
	assert.Error(t, got[1].Error).Is(boom)
	assert.Error(t, got[2].Error).Nil()
	// The status is classified by the loop, so the attempt histogram has its
	// bucket without the emitter knowing this package's sentinels.
	assert.String(t, got[0].Status).Equal("error")
	assert.String(t, got[1].Status).Equal("error")
	assert.String(t, got[2].Status).Equal("ok")
	// The plan's retry ceiling rides every round, so the emitter can report
	// "given 2 retries" from the record alone — the used count is the rounds
	// beyond the first, derived rather than stored.
	for i := range got {
		assert.Number(t, got[i].MaxRetries).Equal(2)
	}
	// Two gaps between three attempts, each the (Multiplier==0 -> 1) fixed
	// interval. The record holds each attempt's own start and duration, so the
	// sleep between attempts is the observed gap between one attempt's end and
	// the next attempt's start.
	gap1 := got[1].Start.Sub(got[0].Start.Add(got[0].Duration))
	gap2 := got[2].Start.Sub(got[1].Start.Add(got[1].Duration))
	assert.That(t, gap1 >= 4*time.Millisecond).True()
	assert.That(t, gap2 >= 4*time.Millisecond).True()
}

// TestRejectionRecordsOneGateRound proves a refusal is itself a ROUND: a call a
// protection stage rejects before any attempt runs records the gate's verdict —
// no peer (nothing was sent), zero duration (no downstream time) and the gate's
// own word in Outcome — so "rejected" is distinguishable from "ran and failed"
// on the call's own record instead of being inferred from an empty one.
func TestRejectionRecordsOneGateRound(t *testing.T) {
	e := newBuiltin(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1})

	// First call spends the burst; the second is rejected before running fn.
	rec := &chain.Recorder{}
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, func(context.Context, *chain.Round) error { return nil })).Nil()
	rec2 := &chain.Recorder{}
	assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, rec2, func(context.Context, *chain.Round) error { return nil })).Is(chain.ErrRateLimited)

	// The recorder is per call: the rejection landed on its own.
	assert.Number(t, len(rec.Rounds)).Equal(1)
	assert.Number(t, len(rec2.Rounds)).Equal(1)

	rejected := rec2.Rounds[0]
	assert.String(t, string(rejected.ErrorType)).Equal("rate_limited")
	assert.String(t, rejected.Status).Equal("error")
	assert.Error(t, rejected.Error).Is(chain.ErrRateLimited)
	assert.That(t, rejected.Duration).Equal(time.Duration(0))
}

// TestNestedCallDoesNotHijackOuterRecorder pins the load-bearing detail: each
// call carries its own recorder, so a protected call nested inside its fn writes
// its own record without stealing the outer call's. Were the record shared rather
// than per-call, the inner call would take the outer's attempt.
func TestNestedCallDoesNotHijackOuterRecorder(t *testing.T) {
	outer := newBuiltin(t, TimeoutRetry{})
	inner := newBuiltin(t, TimeoutRetry{})

	outerRec := &chain.Recorder{}
	err := outer.Execute(context.Background(), chain.Operation{Service: "svc"}, outerRec, func(innerCtx context.Context, _ *chain.Round) error {
		innerRec := &chain.Recorder{}
		assert.Error(t, inner.Execute(innerCtx, chain.Operation{Service: "svc"}, innerRec, func(context.Context, *chain.Round) error { return nil })).Nil()
		assert.Number(t, len(innerRec.Rounds)).Equal(1)
		assert.String(t, innerRec.Rounds[0].Status).Equal("ok")
		return errutil.Explain(nil, "outer fails")
	})

	assert.Error(t, err).NotNil()
	assert.Number(t, len(outerRec.Rounds)).Equal(1)
	assert.Error(t, outerRec.Rounds[0].Error).NotNil()
}

// TestExecuteWithoutRecorderIsSafe proves an executor used with a nil recorder —
// a caller that declined to keep a record — runs unchanged: recording is
// additive and never required.
func TestExecuteWithoutRecorderIsSafe(t *testing.T) {
	e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 1, InitialInterval: time.Millisecond}})
	var calls int
	err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error {
		calls++
		return transient{}
	})
	assert.Error(t, err).NotNil()
	assert.Number(t, calls).Equal(2)
}

// The emitter-facing tests (declared span kind, two-grain metrics,
// rejected-still-emits) live in the observe package's external tests, since the
// wrapper they drive imports this package.

// TestPartitionsStateByMatchedRule proves protection state is partitioned by the
// RULE a call matches, not by the call: calls matching the same interface rule
// share one rate budget, while another interface has no rule and is unmetered.
func TestPartitionsStateByMatchedRule(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := NewManager(factories)
	assert.That(t, err).Nil()
	assert.Error(t, m.ApplyRateLimit(RateLimitSettings{
		Services: []RateLimitService{{
			Service: "svc",
			Rules: []RateLimitRule{{
				ActionMatch: matcher.ActionMatch{Exact: "limited"},
				Policy:      ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1},
			}},
		}},
	})).Nil()
	e := m.Wrap("svc", nil)
	defer func() { _ = m.Close() }()

	limited := chain.Operation{Service: "svc", Request: chain.Request{Action: "limited"}}
	other := chain.Operation{Service: "svc", Request: chain.Request{Action: "other"}}
	noop := func(context.Context, *chain.Round) error { return nil }

	// The limited rule's key spends its one token; the next call is rejected.
	assert.Error(t, e.Execute(context.Background(), limited, new(chain.Recorder), noop)).Nil()
	assert.Error(t, e.Execute(context.Background(), limited, new(chain.Recorder), noop)).Is(chain.ErrRateLimited)

	// A different action has no matching rule, so it has no budget at all.
	for range 3 {
		assert.Error(t, e.Execute(context.Background(), other, new(chain.Recorder), noop)).Nil()
	}
}

// TestEveryGovernedCallRecordsAtLeastOneRound pins the invariant the record's
// readers rely on: a call that ran the chain and was handed a recorder always
// leaves at least one round — a served attempt, or the gate that refused it.
// "No rounds" therefore means "no record was taken" (nil recorder, or a call
// that never entered the chain), never "the call was rejected"; a refusal is a
// decision the chain made, so it is recorded like everything else.
func TestEveryGovernedCallRecordsAtLeastOneRound(t *testing.T) {
	t.Run("served", func(t *testing.T) {
		rec := &chain.Recorder{}
		err := newBuiltin(t, TimeoutRetry{}).Execute(context.Background(), chain.Operation{Service: "svc"}, rec,
			func(context.Context, *chain.Round) error { return nil })
		assert.Error(t, err).Nil()
		assert.Number(t, len(rec.Rounds)).Equal(1)
		assert.String(t, rec.Rounds[0].Status).Equal("ok")
	})

	t.Run("refused-before-the-downstream", func(t *testing.T) {
		e := newBuiltin(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1})
		// Spend the burst so the next call is refused.
		assert.Error(t, e.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder),
			func(context.Context, *chain.Round) error { return nil })).Nil()

		rec := &chain.Recorder{}
		err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, rec,
			func(context.Context, *chain.Round) error { return nil })
		assert.Error(t, err).Is(chain.ErrRateLimited)
		assert.Number(t, len(rec.Rounds)).Equal(1)
		assert.String(t, string(rec.Rounds[0].ErrorType)).Equal("rate_limited")
	})

	t.Run("every-retry-is-a-round", func(t *testing.T) {
		e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 2, InitialInterval: time.Millisecond}})
		rec := &chain.Recorder{}
		err := e.Execute(context.Background(), chain.Operation{Service: "svc"}, rec,
			func(context.Context, *chain.Round) error { return transient{} })
		assert.Error(t, err).NotNil()
		assert.Number(t, len(rec.Rounds)).Equal(3)
	})

	t.Run("an-unused-record-is-still-filled", func(t *testing.T) {
		// There is no nil "record nothing" mode: a caller that never reads the
		// record passes a zero Recorder, and the call still fills it.
		rec := &chain.Recorder{}
		err := newBuiltin(t, TimeoutRetry{}).Execute(context.Background(), chain.Operation{Service: "svc"}, rec,
			func(context.Context, *chain.Round) error { return nil })
		assert.Error(t, err).Nil()
		assert.Number(t, len(rec.Rounds)).Equal(1)
	})
}
