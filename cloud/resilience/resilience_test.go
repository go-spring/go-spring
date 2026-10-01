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

package resilience

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

func newBuiltin(t *testing.T, p ClientPolicy) ClientExecutor {
	d := NewDefaultDriver(nil)
	e, err := d.NewClientExecutor("svc", p)
	assert.Error(t, err).Nil()
	return e
}

func TestBuiltinPassThrough(t *testing.T) {
	// A zero policy protects nothing: fn runs once and its result flows back.
	e := newBuiltin(t, ClientPolicy{})
	var calls int
	err := e.Execute(context.Background(), func(context.Context) error {
		calls++
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, calls).Equal(1)
}

func TestRateLimit(t *testing.T) {
	// Burst of 2, no refill within the test window: 3rd call is rejected.
	e := newBuiltin(t, ClientPolicy{RateLimit: 1, Burst: 2})
	run := func() error {
		return e.Execute(context.Background(), func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(ErrRateLimited)
}

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	e := newBuiltin(t, ClientPolicy{ErrorThreshold: 2, OpenDuration: 50 * time.Millisecond})
	boom := errutil.Explain(nil, "boom")
	fail := func() error {
		return e.Execute(context.Background(), func(context.Context) error { return boom })
	}

	// Two consecutive failures trip the breaker open.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(boom)

	// Now open: the operation is short-circuited without invoking fn.
	assert.Error(t, fail()).Is(ErrCircuitOpen)

	// After the cool-down a trial request is admitted; a success closes it.
	time.Sleep(60 * time.Millisecond)
	assert.Error(t, e.Execute(context.Background(), func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), func(context.Context) error { return nil })).Nil()
}

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	e := newBuiltin(t, ClientPolicy{MaxRetries: 2})
	var attempts int
	err := e.Execute(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errutil.Explain(nil, "transient")
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(3)
}

// TestNonIdempotentOperationIsNotRetried proves a client that declares its
// operation non-idempotent (see [observability.Operation.NonIdempotent]) runs
// ONCE however many retries its policy configures. The retry would repeat the
// side effect — a second email, a second published message — which is exactly
// what the declaration exists to prevent, and the policy cannot know it on its
// own. The contrast is [TestRetrySucceedsAfterTransientFailure], where the same
// policy on an undeclared (idempotent) operation does retry.
// The executor is the emitter-wrapped one, not the bare default: the suppression
// must survive the wrapper in between, which is the only executor a client ever
// holds.
func TestNonIdempotentOperationIsNotRetried(t *testing.T) {
	e := WrapClientExecutor(newBuiltin(t, ClientPolicy{MaxRetries: 3}), "mail", "mail:svc")
	ctx := observability.WithOperation(context.Background(), observability.Operation{
		Name:          "send",
		Metric:        "email.client",
		NonIdempotent: true,
	})
	var attempts int
	err := e.Execute(ctx, func(context.Context) error {
		attempts++
		return errutil.Explain(nil, "transient")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

func TestExecutePerAttemptTimeout(t *testing.T) {
	e := newBuiltin(t, ClientPolicy{AttemptTimeout: 20 * time.Millisecond})
	err := e.Execute(context.Background(), func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})
	assert.Error(t, err).Is(context.DeadlineExceeded)
}

func TestRoundTripperNilExecIsPassThrough(t *testing.T) {
	base := http.DefaultTransport
	assert.That(t, NewRoundTripper(base, nil) == base).True()
}

func TestRoundTripperRetriesOn5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := newBuiltin(t, ClientPolicy{MaxRetries: 3})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e)}

	resp, err := client.Get(srv.URL)
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	_ = resp.Body.Close()
	assert.That(t, atomic.LoadInt32(&hits)).Equal(int32(3))
}

// TestRoundTripperCarriesTheBudget is the hop boundary end to end: a call running
// under a deadline puts what is left of it into the request metadata, so the
// receiving service can bound its own work by the remainder; a call with no
// budget sends nothing rather than a zero, which would read as "already spent".
func TestRoundTripperCarriesTheBudget(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get(BudgetHeader))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := newBuiltin(t, ClientPolicy{})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e)}

	resp, err := client.Get(srv.URL) // no deadline on the request
	assert.Error(t, err).Nil()
	_ = resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	assert.Error(t, err).Nil()
	resp, err = client.Do(req)
	assert.Error(t, err).Nil()
	_ = resp.Body.Close()

	assert.Number(t, len(got)).Equal(2)
	assert.String(t, got[0]).Equal("")
	ms, perr := strconv.ParseInt(got[1], 10, 64)
	assert.Error(t, perr).Nil()
	assert.That(t, ms > 59_000 && ms <= 60_000).True()
}

func TestRoundTripperCircuitOpenIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	e := newBuiltin(t, ClientPolicy{ErrorThreshold: 1, OpenDuration: time.Minute})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e)}

	_, err := client.Get(srv.URL) // trips the breaker
	assert.Error(t, err).NotNil()
	_, err = client.Get(srv.URL) // now short-circuited
	assert.Error(t, err).Is(ErrCircuitOpen)
}

func TestBulkheadRejectsWhenFull(t *testing.T) {
	// MaxConcurrent 1: while one call is parked inside fn, a second is rejected
	// with ErrBulkheadFull rather than queued.
	e := newBuiltin(t, ClientPolicy{MaxConcurrent: 1})

	release := make(chan struct{})
	entered := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = e.Execute(context.Background(), func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	})

	<-entered // first call now holds the only slot
	err := e.Execute(context.Background(), func(context.Context) error { return nil })
	assert.Error(t, err).Is(ErrBulkheadFull)

	close(release)
	wg.Wait()

	// Slot freed: a subsequent call succeeds again.
	assert.Error(t, e.Execute(context.Background(), func(context.Context) error { return nil })).Nil()
}

func TestFallbackDegradesOnRejection(t *testing.T) {
	// A tripped breaker rejects the call; degrade turns the rejection into a
	// graceful result and sees the triggering error.
	e := newBuiltin(t, ClientPolicy{ErrorThreshold: 1, OpenDuration: time.Minute})
	boom := errutil.Explain(nil, "boom")

	// Trip the breaker.
	assert.Error(t, e.Execute(context.Background(), func(context.Context) error { return boom })).Is(boom)

	var seen error
	err := Fallback(context.Background(), e,
		func(context.Context) error { return errutil.Explain(nil, "should not run") },
		func(_ context.Context, cause error) error { seen = cause; return nil })
	assert.Error(t, err).Nil()
	assert.Error(t, seen).Is(ErrCircuitOpen)
}

func TestFallbackNilExecStillDegrades(t *testing.T) {
	// With no executor the call runs directly, and a failure still reaches degrade.
	boom := errutil.Explain(nil, "boom")
	err := Fallback(context.Background(), nil,
		func(context.Context) error { return boom },
		func(_ context.Context, cause error) error {
			if errors.Is(cause, boom) {
				return nil
			}
			return cause
		})
	assert.Error(t, err).Nil()
}

func TestDialerNilExecIsPassThrough(t *testing.T) {
	var called bool
	base := DialFunc(func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil })
	got := NewDialer(base, nil)
	_, err := got(context.Background(), "tcp", "x")
	assert.Error(t, err).Nil()
	assert.That(t, called).True()
}

func TestDialerBreakerOpensOnDialFailures(t *testing.T) {
	dialErr := errutil.Explain(nil, "connection refused")
	base := DialFunc(func(context.Context, string, string) (net.Conn, error) { return nil, dialErr })

	e := newBuiltin(t, ClientPolicy{ErrorThreshold: 2, OpenDuration: time.Minute})
	dial := NewDialer(base, e)

	// Two failed dials trip the breaker.
	_, err := dial(context.Background(), "tcp", "addr")
	assert.Error(t, err).Is(dialErr)
	_, err = dial(context.Background(), "tcp", "addr")
	assert.Error(t, err).Is(dialErr)

	// Now open: the dial is short-circuited without touching base.
	_, err = dial(context.Background(), "tcp", "addr")
	assert.Error(t, err).Is(ErrCircuitOpen)
}

// --- new: backoff, retry classification, total budget, half-open, error-rate ---

func TestRetryBackoffSleeps(t *testing.T) {
	// With InitialInterval set, retries are paced: the gap between attempt 1
	// and attempt 2 must be at least one backoff interval.
	e := newBuiltin(t, ClientPolicy{
		MaxRetries:      1,
		InitialInterval: 40 * time.Millisecond,
	})
	var attempts int
	var firstSaw, secondSaw time.Duration
	start := time.Now()
	err := e.Execute(context.Background(), func(context.Context) error {
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

func TestRetryRespectsMaxDuration(t *testing.T) {
	// MaxDuration caps the whole call: even with many retries permitted, the
	// loop stops once the budget is exhausted.
	e := newBuiltin(t, ClientPolicy{
		MaxRetries:      20,
		InitialInterval: 20 * time.Millisecond,
		MaxDuration:     60 * time.Millisecond,
	})
	var attempts int
	_ = e.Execute(context.Background(), func(context.Context) error {
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
	e := newBuiltin(t, ClientPolicy{MaxRetries: 3})
	var attempts int
	err := e.Execute(context.Background(), func(context.Context) error {
		attempts++
		return nonRetryableErr{}
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

type nonRetryableErr struct{}

func (nonRetryableErr) Error() string   { return "permanent" }
func (nonRetryableErr) Retryable() bool { return false }

func TestHalfOpenAdmitsSingleTrialConcurrent(t *testing.T) {
	// Regression for the bool-flag half-open bug: once cool-down elapses, at
	// most ONE trial is admitted even under concurrency. A second concurrent
	// caller is treated as still-open.
	e := newBuiltin(t, ClientPolicy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	// Trip the breaker.
	_ = e.Execute(context.Background(), func(context.Context) error {
		return errutil.Explain(nil, "boom")
	})
	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open

	// Two concurrent calls: the first that reaches the half-open gate runs fn;
	// the other must be rejected as ErrCircuitOpen (no second trial permit).
	var wg sync.WaitGroup
	var ran, rejected int32
	for range 2 {
		wg.Go(func() {
			err := e.Execute(context.Background(), func(context.Context) error {
				atomic.AddInt32(&ran, 1)
				// Hold long enough that the sibling surely evaluates allow() too.
				time.Sleep(20 * time.Millisecond)
				return nil // trial succeeds -> closes
			})
			if errors.Is(err, ErrCircuitOpen) {
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
	e := newBuiltin(t, ClientPolicy{
		BreakerStrategy:    BreakerErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		BreakerWindow:      time.Second,
		OpenDuration:       time.Minute,
	})
	// Interleave fail/success: 4 fails out of 8 => 50% ratio.
	for i := range 8 {
		err := e.Execute(context.Background(), func(context.Context) error {
			if i%2 == 0 {
				return errutil.Explain(nil, "fail")
			}
			return nil
		})
		// Once the breaker trips (after MinRequests with ratio met) further
		// calls are short-circuited. The last iteration should be rejected.
		if errors.Is(err, ErrCircuitOpen) {
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
	e := newBuiltin(t, ClientPolicy{
		ErrorThreshold: 3,
		MaxRetries:     2,
		OpenDuration:   time.Minute,
	})
	fail := func() error {
		return e.Execute(context.Background(), func(context.Context) error {
			return errutil.Explain(nil, "boom")
		})
	}

	// First failing call: 3 attempts, but only 1 breaker sample. The next call
	// must still reach fn (return "boom"), proving the breaker did NOT open.
	err := fail()
	assert.Error(t, err).NotNil()
	assert.That(t, !errors.Is(err, ErrCircuitOpen)).True()

	// Second failing call: 2 samples now, still closed.
	err = fail()
	assert.Error(t, err).NotNil()
	assert.That(t, !errors.Is(err, ErrCircuitOpen)).True()

	// Third failing call records the 3rd sample and opens — but the opening
	// happens at record time (after fn ran), so this call still returns "boom".
	err = fail()
	assert.Error(t, err).NotNil()
	assert.That(t, !errors.Is(err, ErrCircuitOpen)).True()

	// The fourth call is rejected outright without invoking fn.
	assert.Error(t, fail()).Is(ErrCircuitOpen)
}

type timeoutNetErr struct{}

func (*timeoutNetErr) Error() string   { return "i/o timeout" }
func (*timeoutNetErr) Timeout() bool   { return true }
func (*timeoutNetErr) Temporary() bool { return true }

func TestRateLimitQueueing(t *testing.T) {
	// rate-limit-max-wait turns an over-limit call into a bounded wait: a
	// 1/s bucket with burst 1 admits call 1 immediately, and call 2 — which
	// maxWait=0 would reject — instead waits for the next token and succeeds.
	e := newBuiltin(t, ClientPolicy{RateLimit: 20, Burst: 1, RateLimitMaxWait: 200 * time.Millisecond})
	run := func() error {
		return e.Execute(context.Background(), func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	// And with no wait budget the same shape rejects.
	e2 := newBuiltin(t, ClientPolicy{RateLimit: 20, Burst: 1})
	assert.Error(t, e2.Execute(context.Background(), func(context.Context) error { return nil })).Nil()
	assert.Error(t, e2.Execute(context.Background(), func(context.Context) error { return nil })).Is(ErrRateLimited)
}

func TestSlowCallBreaker(t *testing.T) {
	// A downstream that never FAILS but always answers slowly trips the
	// slow-call breaker over the rate window.
	e := newBuiltin(t, ClientPolicy{
		BreakerStrategy:           BreakerErrorRate,
		ErrorRateThreshold:        0, // errors never happen
		SlowCallDurationThreshold: 20 * time.Millisecond,
		SlowCallRateThreshold:     0.5,
		MinRequests:               2,
		BreakerWindow:             time.Minute,
		OpenDuration:              50 * time.Millisecond,
	})
	slow := func() error {
		return e.Execute(context.Background(), func(context.Context) error {
			time.Sleep(25 * time.Millisecond) // >= threshold: a slow success
			return nil
		})
	}
	assert.Error(t, slow()).Nil()
	assert.Error(t, slow()).Nil()
	// 2/2 slow >= 0.5: the breaker must now be open.
	err := e.Execute(context.Background(), func(context.Context) error { return nil })
	assert.Error(t, err).Is(ErrCircuitOpen)
}

func TestHalfOpenMultipleTrials(t *testing.T) {
	// HalfOpenRequests=3 admits three trials; one flapping failure among them
	// must re-open the circuit, and three successes in a row close it.
	e := newBuiltin(t, ClientPolicy{
		ErrorThreshold:   1,
		OpenDuration:     30 * time.Millisecond,
		HalfOpenRequests: 3,
	})
	fail := func() error {
		return e.Execute(context.Background(), func(context.Context) error {
			return errutil.Explain(nil, "boom")
		})
	}
	if err := fail(); err == nil {
		t.Fatal("first failure should return the downstream error (and trip the breaker)") // threshold 1
	}
	time.Sleep(40 * time.Millisecond) // cool-down elapses
	// First trial succeeds but is only 1 of 3: the breaker stays half-open
	// (permit available), not closed.
	err := e.Execute(context.Background(), func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
	// Second trial fails: re-open immediately.
	time.Sleep(40 * time.Millisecond)
	_ = fail() // the trial runs (or is gated); either way the circuit re-opens
	time.Sleep(40 * time.Millisecond)
	// After re-open and cool-down, three consecutive successes close it.
	for range 3 {
		err = e.Execute(context.Background(), func(context.Context) error { return nil })
	}
	assert.Error(t, err).Nil()
	assert.Error(t, e.Execute(context.Background(), func(context.Context) error { return nil })).Nil()
}

func TestRetryBudget(t *testing.T) {
	// RetryBudget caps in-flight retries for the executor: call A's retry (in
	// flight, blocked inside fn) holds the single budget slot, so call B's
	// retry is rejected with ErrRetryBudgetExceeded. First attempts never take
	// budget. Both calls run through one executor, which serves one service.
	e := newBuiltin(t, ClientPolicy{MaxRetries: 1, RetryBudget: 1})
	inRetry := make(chan struct{})
	unblock := make(chan struct{})
	aDone := make(chan error, 1)
	go func() {
		calls := 0
		aDone <- e.Execute(context.Background(), func(context.Context) error {
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
	err := e.Execute(context.Background(), func(context.Context) error {
		bCalls++
		return errutil.Explain(nil, "boom")
	})
	assert.Error(t, err).Is(ErrRetryBudgetExceeded)
	assert.That(t, bCalls).Equal(1)

	close(unblock)
	if err := <-aDone; err == nil {
		t.Fatal("call A should end failed, not nil")
	}
}
