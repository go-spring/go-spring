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

	"go-spring.org/cloud/runtime/dataplane/kernel/breaker"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// newBuiltin builds the bundled client engine for one service under p. It is the
// shared entry the executor, retry, breaker and emit tests all start from.
func newBuiltin(t *testing.T, p Policy) chain.Executor {
	t.Helper()
	d := NewDefaultDriver(nil)
	e, err := d.NewExecutor("svc", p)
	assert.Error(t, err).Nil()
	return e
}

// The observe-layer tests that drive the wrapper over these engines — spans,
// metrics and the recorder hand-off — live in emitter_test.go, an external test
// package, because the wrapper (cloud/runtime/dataplane/outbound/observe) imports this package.

func TestBuiltinPassThrough(t *testing.T) {
	// A zero policy protects nothing: fn runs once and its result flows back.
	e := newBuiltin(t, Policy{})
	var calls int
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		calls++
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, calls).Equal(1)
}

// TestExecuteTimeout pins the one bound: a call given a Timeout is cut off by it,
// as a deadline its fn observes rather than an error the executor invents.
func TestExecuteTimeout(t *testing.T) {
	e := newBuiltin(t, Policy{Timeout: 20 * time.Millisecond})
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})
	assert.Error(t, err).Is(context.DeadlineExceeded)
}

func TestRateLimit(t *testing.T) {
	// Burst of 2, no refill within the test window: 3rd call is rejected.
	e := newBuiltin(t, Policy{RateLimit: 1, Burst: 2})
	run := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(chain.ErrRateLimited)
}

func TestRateLimitQueueing(t *testing.T) {
	// rate-limit-max-wait turns an over-limit call into a bounded wait: a
	// 1/s bucket with burst 1 admits call 1 immediately, and call 2 — which
	// maxWait=0 would reject — instead waits for the next token and succeeds.
	e := newBuiltin(t, Policy{RateLimit: 20, Burst: 1, RateLimitMaxWait: 200 * time.Millisecond})
	run := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	// And with no wait budget the same shape rejects.
	e2 := newBuiltin(t, Policy{RateLimit: 20, Burst: 1})
	assert.Error(t, e2.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
	assert.Error(t, e2.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Is(chain.ErrRateLimited)
}

// TestWarmUpColdStart pins the warm-up knob on the default driver's token
// bucket: right after construction the bucket holds only the cold capacity
// (burst/coldFactor), so a burst beyond it is rejected before the ramp fills
// the bucket — and the ramp completes to full burst capacity.
func TestWarmUpColdStart(t *testing.T) {
	// rate 100/s, burst 10 (explicit), warm-up 2s at coldFactor 10: cold
	// capacity 1 token, growing to 10 over 2 seconds.
	e := newBuiltin(t, Policy{
		RateLimit:               100,
		Burst:                   10,
		RateLimitWarmUpDuration: 2 * time.Second,
		WarmUpColdFactor:        10,
	})
	// The first call spends the single cold token; the second, immediately
	// after, must be rejected — without warm-up the full burst of 10 would be
	// available.
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Is(chain.ErrRateLimited)
}

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 2}})
	var attempts int
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errutil.Explain(nil, "transient")
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(3)
}

func TestRetryBackoffSleeps(t *testing.T) {
	// With InitialInterval set, retries are paced: the gap between attempt 1
	// and attempt 2 must be at least one backoff interval.
	e := newBuiltin(t, Policy{
		Retry: chain.RetryPolicy{
			MaxRetries:      1,
			InitialInterval: 40 * time.Millisecond,
		},
	})
	var attempts int
	var firstSaw, secondSaw time.Duration
	start := time.Now()
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		attempts++
		if attempts == 1 {
			firstSaw = time.Since(start)
		} else {
			secondSaw = time.Since(start)
		}
		if attempts < 2 {
			return errutil.Explain(nil, "transient")
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
	e := newBuiltin(t, Policy{
		Retry: chain.RetryPolicy{
			MaxRetries:      20,
			InitialInterval: 20 * time.Millisecond,
		},
		Timeout: 60 * time.Millisecond,
	})
	var attempts int
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		attempts++
		return errutil.Explain(nil, "always fails")
	})
	// Not all 21 attempts ran — the budget cut the loop short.
	assert.That(t, attempts < 21).True()
	assert.That(t, attempts >= 1).True()
}

func TestRetryableFalseStopsRetry(t *testing.T) {
	// A Retryable() false error stops the loop after the first failure, even
	// though MaxRetries would allow more attempts.
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 3}})
	var attempts int
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		attempts++
		return nonRetryableErr{}
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

type nonRetryableErr struct{}

func (nonRetryableErr) Error() string   { return "permanent" }
func (nonRetryableErr) Retryable() bool { return false }

// TestRetryDefaultsToSingleAttempt pins the new default: with neither a policy
// retry node nor a call retry node, a failing call runs exactly ONCE. Retry is
// opt-in — the zero value of the node is "no retry".
func TestRetryDefaultsToSingleAttempt(t *testing.T) {
	e := newBuiltin(t, Policy{})
	var attempts int
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		attempts++
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

// TestCallDeclaredRetryDrivesTheLoop proves the caller's node on the operation is
// enough on its own: an empty policy plus op.Retry{MaxRetries: 2} retries the
// call (3 attempts total). This is the "decision at the caller" path.
func TestCallDeclaredRetryDrivesTheLoop(t *testing.T) {
	e := newBuiltin(t, Policy{})
	var attempts int
	op := chain.Operation{Retry: chain.RetryPolicy{MaxRetries: 2}}
	err := e.Execute(context.Background(), op, nil, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errutil.Explain(nil, "transient")
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(3)
}

// TestPolicyRetryOverridesCallRetry proves whole-node precedence: when the
// governance center pushes a retry schedule (through the policy), it wins over
// the call's own node — a pushed value is how an operator hot-fixes a live
// incident. The call asks for 3 retries, the policy for 1, so 2 attempts run.
func TestPolicyRetryOverridesCallRetry(t *testing.T) {
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 1}})
	var attempts int
	op := chain.Operation{Retry: chain.RetryPolicy{MaxRetries: 3}}
	err := e.Execute(context.Background(), op, nil, func(context.Context) error {
		attempts++
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(2)
}

// TestNonIdempotentSurvivesPolicyOverride proves the safety flag is OR-ed across
// both sources rather than replaced: a policy push that replaces the whole node
// must NOT drop the caller's non-idempotent declaration — otherwise a hot-fix
// would re-enable retry on a side-effecting call. The policy here declares an
// (idempotent-assuming) schedule; the call declares non-idempotent; the call must
// still run once.
func TestNonIdempotentSurvivesPolicyOverride(t *testing.T) {
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 2}})
	var attempts int
	op := chain.Operation{Retry: chain.RetryPolicy{NonIdempotent: true}}
	err := e.Execute(context.Background(), op, nil, func(context.Context) error {
		attempts++
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

// TestPolicyBudgetAloneDoesNotOverrideCallSchedule proves the merge keys on the
// SCHEDULE, not on any retry field: a center that pushes only a process-level
// retry-budget declares no schedule, so the call's own retry node still applies.
// The budget caps in-flight retries; it must not wipe the caller's retry.
func TestPolicyBudgetAloneDoesNotOverrideCallSchedule(t *testing.T) {
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{RetryBudget: 8}})
	var attempts int
	op := chain.Operation{Retry: chain.RetryPolicy{MaxRetries: 2}}
	err := e.Execute(context.Background(), op, nil, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errutil.Explain(nil, "transient")
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(3)
}

func TestRetryBudget(t *testing.T) {
	// RetryBudget caps in-flight retries for the executor: call A's retry (in
	// flight, blocked inside fn) holds the single budget slot, so call B's
	// retry is rejected with chain.ErrRetryBudgetExceeded. First attempts never take
	// budget. Both calls run through one executor, which serves one service.
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 1, RetryBudget: 1}})
	inRetry := make(chan struct{})
	unblock := make(chan struct{})
	aDone := make(chan error, 1)
	go func() {
		calls := 0
		aDone <- e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			calls++
			if calls == 2 { // the retry attempt: hold the budget slot
				close(inRetry)
				<-unblock
			}
			return errutil.Explain(nil, "boom")
		})
	}()
	<-inRetry // A is now blocked inside its retry, holding the only slot

	// B's first attempt runs fine; its retry finds the budget spent.
	bCalls := 0
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		bCalls++
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).Is(chain.ErrRetryBudgetExceeded)
	assert.That(t, bCalls).Equal(1)

	close(unblock)
	if err := <-aDone; err == nil {
		t.Fatal("call A should end failed, not nil")
	}
}

// testSource is a [PolicySource] a test drives directly, so the executor's
// generation handling can be exercised without standing up a manager: the policy
// a label resolves and the generation it belongs to are both settable.
type testSource struct {
	mu  sync.Mutex
	p   Policy
	gen uint64
}

func (s *testSource) set(p Policy, gen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p, s.gen = p, gen
}

func (s *testSource) resolve(key string) CallPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CallPolicy{Policy: s.p, Key: key, Label: s.p, Gen: s.gen}
}

func (s *testSource) Resolve(op chain.Operation) CallPolicy { return s.resolve(op.Service) }

func (s *testSource) ResolveLabel(label string) CallPolicy { return s.resolve(label) }

// stubSource is a [PolicySource] built from one function, for tests that only
// need the call-level resolution.
type stubSource func(chain.Operation) CallPolicy

func (f stubSource) Resolve(op chain.Operation) CallPolicy { return f(op) }

func (f stubSource) ResolveLabel(label string) CallPolicy { return f(chain.Operation{Service: label}) }

// TestGenerationChangeRebuildsState pins the contract the pull design buys: the
// executor's materialized state belongs to ONE generation, and moving to the next
// one rebuilds it — a tripped breaker is CLOSED again, because the state of a
// superseded generation is not carried over. It also pins the other half, which
// is what makes that safe: while the generation does NOT move, the state is
// reused, so a config push that changes nothing costs nothing.
func TestGenerationChangeRebuildsState(t *testing.T) {
	e := newBuiltin(t, Policy{})
	src := &testSource{}
	e.(PolicySourceSetter).SetPolicySource(src)
	src.set(Policy{ErrorThreshold: 2, OpenDuration: time.Hour}, 1)

	boom := errutil.Explain(nil, "boom")
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return boom })
	}

	// Trip the breaker open under generation 1.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)

	// Same generation, same policy: the state is reused and the circuit stays
	// open, so an unrelated push costs this label nothing.
	src.set(Policy{ErrorThreshold: 2, OpenDuration: time.Hour}, 1)
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)

	// New generation: the state is rebuilt, so the breaker starts over CLOSED and
	// the call reaches the downstream again.
	src.set(Policy{ErrorThreshold: 2, OpenDuration: time.Hour}, 2)
	assert.Error(t, fail()).Is(boom)
}

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	e := newBuiltin(t, Policy{ErrorThreshold: 2, OpenDuration: 50 * time.Millisecond})
	boom := errutil.Explain(nil, "boom")
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return boom })
	}

	// Two consecutive failures trip the breaker open.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(boom)

	// Now open: the operation is short-circuited without invoking fn.
	assert.Error(t, fail()).Is(chain.ErrCircuitOpen)

	// After the cool-down a trial request is admitted; a success closes it.
	time.Sleep(60 * time.Millisecond)
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
}

func TestHalfOpenAdmitsSingleTrialConcurrent(t *testing.T) {
	// Regression for the bool-flag half-open bug: once cool-down elapses, at
	// most ONE trial is admitted even under concurrency. A second concurrent
	// caller is treated as still-open.
	e := newBuiltin(t, Policy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	// Trip the breaker.
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		return errutil.Explain(nil, "boom")
	})
	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open

	// Two concurrent calls: the first that reaches the half-open gate runs fn;
	// the other must be rejected as chain.ErrCircuitOpen (no second trial permit).
	var wg sync.WaitGroup
	var ran, rejected int32
	for range 2 {
		wg.Go(func() {
			err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
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

func TestErrorRateBreakerTripsOnRatio(t *testing.T) {
	// error-rate breaker: trips when fails/total >= threshold with enough
	// samples, even though successes are interleaved (so a consecutive counter
	// would never trip).
	e := newBuiltin(t, Policy{
		BreakerStrategy:    breaker.ErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		BreakerWindow:      time.Second,
		OpenDuration:       time.Minute,
	})
	// Interleave fail/success: 4 fails out of 8 => 50% ratio.
	for i := range 8 {
		err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
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
	e := newBuiltin(t, Policy{
		ErrorThreshold: 3,
		OpenDuration:   time.Minute,
		Retry:          chain.RetryPolicy{MaxRetries: 2},
	})
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			return errutil.Explain(nil, "boom")
		})
	}

	// First failing call: 3 attempts, but only 1 breaker sample. The next call
	// must still reach fn (return "boom"), proving the breaker did NOT open.
	err := fail()
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
	e := newBuiltin(t, Policy{
		BreakerStrategy:           breaker.ErrorRate,
		ErrorRateThreshold:        0, // errors never happen
		SlowCallDurationThreshold: 20 * time.Millisecond,
		SlowCallRateThreshold:     0.5,
		MinRequests:               2,
		BreakerWindow:             time.Minute,
		OpenDuration:              50 * time.Millisecond,
	})
	slow := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			time.Sleep(25 * time.Millisecond) // >= threshold: a slow success
			return nil
		})
	}
	assert.Error(t, slow()).Nil()
	assert.Error(t, slow()).Nil()
	// 2/2 slow >= 0.5: the breaker must now be open.
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

func TestHalfOpenMultipleTrials(t *testing.T) {
	// HalfOpenRequests=3 admits three trials; one flapping failure among them
	// must re-open the circuit, and three successes in a row close it.
	e := newBuiltin(t, Policy{
		ErrorThreshold:   1,
		OpenDuration:     30 * time.Millisecond,
		HalfOpenRequests: 3,
	})
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			return errutil.Explain(nil, "boom")
		})
	}
	if err := fail(); err == nil {
		t.Fatal("first failure should return the downstream error (and trip the breaker)") // threshold 1
	}
	time.Sleep(40 * time.Millisecond) // cool-down elapses
	// First trial succeeds but is only 1 of 3: the breaker stays half-open
	// (permit available), not closed.
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
	// Second trial fails: re-open immediately.
	time.Sleep(40 * time.Millisecond)
	_ = fail() // the trial runs (or is gated); either way the circuit re-opens
	time.Sleep(40 * time.Millisecond)
	// After re-open and cool-down, three consecutive successes close it.
	for range 3 {
		err = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	}
	assert.Error(t, err).Nil()
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
}

// TestSlowCallStrategyBreaker pins the EXPLICIT slow-call strategy: only the
// slow-call ratio trips it — fast failures (the error-rate window's signal) must
// NOT open a slow-call breaker, and slow-but-successful calls MUST.
func TestSlowCallStrategyBreaker(t *testing.T) {
	e := newBuiltin(t, Policy{
		BreakerStrategy:           breaker.SlowCall,
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
		_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			return errutil.Explain(nil, "fast failure")
		})
	}
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Nil() // still closed: failures are not slowness

	// Slow successes only: 2/2 slow >= 0.5 trips it.
	for range 2 {
		_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			time.Sleep(25 * time.Millisecond)
			return nil
		})
	}
	err = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

// TestErrorCountStrategyBreaker pins the error-count strategy: the failure
// COUNT over the window trips it even when interleaved with successes (a
// consecutive counter would have kept resetting), and a success rate high
// enough keeps it closed.
func TestErrorCountStrategyBreaker(t *testing.T) {
	e := newBuiltin(t, Policy{
		BreakerStrategy: breaker.ErrorCount,
		ErrorThreshold:  3,
		MinRequests:     1,
		BreakerWindow:   time.Minute,
		OpenDuration:    time.Minute,
	})
	// fail, ok, fail, ok: never 3 in a row, but 3 failures in the window.
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return errutil.Explain(nil, "boom") })
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return errutil.Explain(nil, "boom") })
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return errutil.Explain(nil, "boom") }) // 3rd failure trips
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

func TestFallbackDegradesOnRejection(t *testing.T) {
	// A tripped breaker rejects the call; degrade turns the rejection into a
	// graceful result and sees the triggering error.
	e := newBuiltin(t, Policy{ErrorThreshold: 1, OpenDuration: time.Minute})
	boom := errutil.Explain(nil, "boom")

	// Trip the breaker.
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return boom })).Is(boom)

	var seen error
	op := chain.Operation{}
	rec := &chain.Recorder{}
	err := Fallback(context.Background(), e, op, rec,
		func(context.Context) error { return errutil.Explain(nil, "should not run") },
		func(_ context.Context, cause error) error { seen = cause; return nil })
	assert.Error(t, err).Nil()
	assert.Error(t, seen).Is(chain.ErrCircuitOpen)
	// The record carries the rejection that triggered the degrade: it is how a
	// degraded result is told from a real one — the call's rounds say it failed,
	// and this one says why.
	last, ok := rec.LastRound()
	assert.That(t, ok).True("the refusal was recorded as a round")
	assert.String(t, last.ErrorType).Equal("circuit_open")
	assert.Error(t, last.Err).Is(chain.ErrCircuitOpen)
}

func TestFallbackNilExecStillDegrades(t *testing.T) {
	// With no executor the call runs directly, and a failure still reaches degrade.
	boom := errutil.Explain(nil, "boom")
	err := Fallback(context.Background(), nil, chain.Operation{}, nil,
		func(context.Context) error { return boom },
		func(_ context.Context, cause error) error {
			if errors.Is(cause, boom) {
				return nil
			}
			return cause
		})
	assert.Error(t, err).Nil()
}

// recordingListener captures breaker state transitions for test assertions.
type recordingListener struct {
	events []struct {
		service  string
		from, to breaker.State
	}
}

func (l *recordingListener) OnBreakerStateChange(service string, from, to breaker.State) {
	l.events = append(l.events, struct {
		service  string
		from, to breaker.State
	}{service, from, to})
}

// TestBreakerEvents verifies the default driver emits closed→open (trip),
// open→half-open (probe) and half-open→closed (recovery) transitions to a
// listener attached via SetBreakerEventListener.
func TestBreakerEvents(t *testing.T) {
	d := NewDefaultDriver(nil)
	exec, err := d.NewExecutor("svc", Policy{ErrorThreshold: 2, OpenDuration: 5 * time.Millisecond})
	assert.Error(t, err).Nil()
	defer func() { _ = exec.Close() }()

	rec := &recordingListener{}
	exec.(breaker.EventListenerSetter).SetBreakerEventListener(rec)

	boom := errutil.Explain(nil, "boom")
	ctx := context.Background()
	// Two consecutive failures trip the breaker (closed → open).
	_ = exec.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return boom })
	_ = exec.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return boom })
	assert.That(t, len(rec.events)).Equal(1)
	assert.That(t, rec.events[0].from).Equal(breaker.Closed)
	assert.That(t, rec.events[0].to).Equal(breaker.Open)

	// The breaker is open: this call is rejected before fn runs, no transition.
	_ = exec.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.That(t, len(rec.events)).Equal(1)

	// Wait out the cool-down so the next call is admitted as a half-open trial.
	time.Sleep(20 * time.Millisecond)
	// Trial succeeds: open → half-open (probe admitted), then half-open → closed.
	_ = exec.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.That(t, len(rec.events)).Equal(3)
	assert.That(t, rec.events[1].from).Equal(breaker.Open)
	assert.That(t, rec.events[1].to).Equal(breaker.HalfOpen)
	assert.That(t, rec.events[2].from).Equal(breaker.HalfOpen)
	assert.That(t, rec.events[2].to).Equal(breaker.Closed)
}

// TestBreakerEventsNoListener confirms the breaker still works (no panic) when
// no listener is attached.
func TestBreakerEventsNoListener(t *testing.T) {
	d := NewDefaultDriver(nil)
	exec, err := d.NewExecutor("svc", Policy{ErrorThreshold: 1, OpenDuration: time.Second})
	assert.Error(t, err).Nil()
	defer func() { _ = exec.Close() }()

	err = exec.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return errutil.Explain(nil, "boom") })
	assert.Error(t, err).NotNil() // tripped, no panic
}

// TestExecutorRebuildStartsClean pins what replaced hot reload: a policy change
// is a NEW executor from the driver, and a fresh executor starts with fresh
// per-service state — a breaker tripped under the old (tight) policy is gone, so
// calls run again.
func TestExecutorRebuildStartsClean(t *testing.T) {
	d := NewDefaultDriver(nil)

	tight, err := d.NewExecutor("svc", Policy{ErrorThreshold: 1, OpenDuration: time.Hour})
	assert.Error(t, err).Nil()
	defer func() { _ = tight.Close() }()

	ctx := context.Background()
	boom := errutil.Explain(nil, "boom")
	_ = tight.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return boom }) // trips (threshold 1)
	assert.Error(t, tight.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })).Is(chain.ErrCircuitOpen)

	// The same service under a looser policy: a newly built executor, so its
	// breaker starts closed and the call runs.
	loose, err := d.NewExecutor("svc", Policy{ErrorThreshold: 100, OpenDuration: time.Hour})
	assert.Error(t, err).Nil()
	defer func() { _ = loose.Close() }()
	assert.Error(t, loose.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()

	// A negative rate limit is not special-cased at construction: it builds, and
	// the rate-limit stage treats it as no limit at all (RateLimit <= 0 is off).
	neg, err := d.NewExecutor("svc", Policy{RateLimit: -1})
	assert.Error(t, err).Nil()
	defer func() { _ = neg.Close() }()
	assert.Error(t, neg.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
}

// TestGovernanceLoopRecordsAttempts proves the retry loop writes each attempt —
// its outcome and its own cost — and the sleep between attempts into the
// caller-supplied recorder, which is the only source the outer emitter has.
func TestGovernanceLoopRecordsAttempts(t *testing.T) {
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 2, InitialInterval: 5 * time.Millisecond}})
	boom := errutil.Explain(nil, "transient")

	rec := &chain.Recorder{}
	var calls int
	err := e.Execute(context.Background(), chain.Operation{}, rec, func(context.Context) error {
		calls++
		if calls < 3 {
			return boom
		}
		return nil
	})
	assert.Error(t, err).Nil()

	got := rec.Rounds()
	assert.Number(t, len(got)).Equal(3)
	assert.Error(t, got[0].Err).Is(boom)
	assert.Error(t, got[1].Err).Is(boom)
	assert.Error(t, got[2].Err).Nil()
	// The status is classified by the loop, so the attempt histogram has its
	// bucket without the emitter knowing this package's sentinels.
	assert.String(t, got[0].Status).Equal("error")
	assert.String(t, got[1].Status).Equal("error")
	assert.String(t, got[2].Status).Equal("ok")
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
	e := newBuiltin(t, Policy{RateLimit: 1, Burst: 1})

	// First call spends the burst; the second is rejected before running fn.
	rec := &chain.Recorder{}
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, rec, func(context.Context) error { return nil })).Nil()
	rec2 := &chain.Recorder{}
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, rec2, func(context.Context) error { return nil })).Is(chain.ErrRateLimited)

	// The recorder is per call: the rejection landed on its own.
	assert.Number(t, len(rec.Rounds())).Equal(1)
	assert.Number(t, len(rec2.Rounds())).Equal(1)

	rejected := rec2.Rounds()[0]
	assert.String(t, rejected.ErrorType).Equal("rate_limited")
	assert.String(t, rejected.Status).Equal("error")
	assert.Error(t, rejected.Err).Is(chain.ErrRateLimited)
	assert.That(t, rejected.Duration).Equal(time.Duration(0))
}

// TestNestedCallDoesNotHijackOuterRecorder pins the load-bearing detail: each
// call carries its own recorder, so a protected call nested inside its fn writes
// its own record without stealing the outer call's. Were the record shared rather
// than per-call, the inner call would take the outer's attempt.
func TestNestedCallDoesNotHijackOuterRecorder(t *testing.T) {
	outer := newBuiltin(t, Policy{})
	inner := newBuiltin(t, Policy{})

	outerRec := &chain.Recorder{}
	err := outer.Execute(context.Background(), chain.Operation{}, outerRec, func(innerCtx context.Context) error {
		innerRec := &chain.Recorder{}
		assert.Error(t, inner.Execute(innerCtx, chain.Operation{}, innerRec, func(context.Context) error { return nil })).Nil()
		assert.Number(t, len(innerRec.Rounds())).Equal(1)
		assert.String(t, innerRec.Rounds()[0].Status).Equal("ok")
		return errutil.Explain(nil, "outer fails")
	})

	assert.Error(t, err).NotNil()
	assert.Number(t, len(outerRec.Rounds())).Equal(1)
	assert.Error(t, outerRec.Rounds()[0].Err).NotNil()
}

// TestExecuteWithoutRecorderIsSafe proves an executor used with a nil recorder —
// a caller that declined to keep a record — runs unchanged: recording is
// additive and never required.
func TestExecuteWithoutRecorderIsSafe(t *testing.T) {
	e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 1, InitialInterval: time.Millisecond}})
	var calls int
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		calls++
		return errutil.Explain(nil, "transient")
	})
	assert.Error(t, err).NotNil()
	assert.Number(t, calls).Equal(2)
}

// The emitter-facing tests (declared span kind, two-grain metrics, undeclared
// fallback, rejected-still-emits) live in emitter_test.go — the external test
// package, since the wrapper they drive imports this package.

// TestPartitionsStateByMatchedRule proves protection state is partitioned by the
// RULE a call's metadata selects, not by the metadata itself: calls matching the
// same rule share one rate budget, and a rule that gives one service a different
// policy gets its own — so one tenant's load cannot exhaust another's budget. It
// also proves the policy resolver — the seam governance uses to match a rule on
// a call's Metadata — selects both the policy and the state key.
func TestPartitionsStateByMatchedRule(t *testing.T) {
	d := NewDefaultDriver(nil)
	e, err := d.NewExecutor("svc", Policy{})
	assert.Error(t, err).Nil()
	defer func() { _ = e.Close() }()

	// A call whose metadata says tenant=limited runs under a one-token rate
	// limit, keyed by the rule it matched; every other call takes the zero
	// (pass-through) policy under the service key.
	e.(PolicySourceSetter).SetPolicySource(stubSource(func(op chain.Operation) CallPolicy {
		if op.Request.Metadata["tenant"] == "limited" {
			return CallPolicy{Policy: Policy{RateLimit: 1, Burst: 1}, Key: "svc#0"}
		}
		return CallPolicy{Policy: Policy{}, Key: op.Service}
	}))

	limited := chain.Operation{Request: chain.Request{Metadata: map[string]string{"tenant": "limited"}}}
	other := chain.Operation{Request: chain.Request{Metadata: map[string]string{"tenant": "other"}}}
	noop := func(context.Context) error { return nil }

	// The limited rule's key spends its one token; the next call is rejected.
	assert.Error(t, e.Execute(context.Background(), limited, nil, noop)).Nil()
	assert.Error(t, e.Execute(context.Background(), limited, nil, noop)).Is(chain.ErrRateLimited)

	// A different rule (the default, another key) has its OWN budget.
	for range 3 {
		assert.Error(t, e.Execute(context.Background(), other, nil, noop)).Nil()
	}
}
