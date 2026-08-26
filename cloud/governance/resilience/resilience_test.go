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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func newBuiltin(t *testing.T, p Policy) Executor {
	d, err := GetDriver("default")
	assert.Error(t, err).Nil()
	e, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return e
}

func TestBuiltinPassThrough(t *testing.T) {
	// A zero policy protects nothing: fn runs once and its result flows back.
	e := newBuiltin(t, Policy{})
	var calls int
	err := e.Execute(context.Background(), "svc", func(context.Context) error {
		calls++
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, calls).Equal(1)
}

func TestRateLimit(t *testing.T) {
	// Burst of 2, no refill within the test window: 3rd call is rejected.
	e := newBuiltin(t, Policy{RateLimit: 1, Burst: 2})
	run := func() error {
		return e.Execute(context.Background(), "svc", func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(ErrRateLimited)
}

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	e := newBuiltin(t, Policy{ErrorThreshold: 2, OpenDuration: 50 * time.Millisecond})
	boom := errors.New("boom")
	fail := func() error {
		return e.Execute(context.Background(), "svc", func(context.Context) error { return boom })
	}

	// Two consecutive failures trip the breaker open.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, fail()).Is(boom)

	// Now open: the operation is short-circuited without invoking fn.
	assert.Error(t, fail()).Is(ErrCircuitOpen)

	// After the cool-down a trial request is admitted; a success closes it.
	time.Sleep(60 * time.Millisecond)
	assert.Error(t, e.Execute(context.Background(), "svc", func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), "svc", func(context.Context) error { return nil })).Nil()
}

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	e := newBuiltin(t, Policy{MaxRetries: 2})
	var attempts int
	err := e.Execute(context.Background(), "svc", func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient")
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, attempts).Equal(3)
}

func TestExecutePerAttemptTimeout(t *testing.T) {
	e := newBuiltin(t, Policy{Timeout: 20 * time.Millisecond})
	err := e.Execute(context.Background(), "svc", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})
	assert.Error(t, err).Is(context.DeadlineExceeded)
}

func TestRegisterDriverDuplicatePanics(t *testing.T) {
	assert.Panic(t, func() { RegisterDriver("default", defaultDriver{}) }, "already registered")
}

func TestRoundTripperNilExecIsPassThrough(t *testing.T) {
	base := http.DefaultTransport
	assert.That(t, NewRoundTripper(base, nil, nil) == base).True()
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

	e := newBuiltin(t, Policy{MaxRetries: 3})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e, nil)}

	resp, err := client.Get(srv.URL)
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	_ = resp.Body.Close()
	assert.That(t, atomic.LoadInt32(&hits)).Equal(int32(3))
}

func TestRoundTripperCircuitOpenIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	e := newBuiltin(t, Policy{ErrorThreshold: 1, OpenDuration: time.Minute})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e, nil)}

	_, err := client.Get(srv.URL) // trips the breaker
	assert.Error(t, err).NotNil()
	_, err = client.Get(srv.URL) // now short-circuited
	assert.Error(t, err).Is(ErrCircuitOpen)
}

func TestBulkheadRejectsWhenFull(t *testing.T) {
	// MaxConcurrent 1: while one call is parked inside fn, a second is rejected
	// with ErrBulkheadFull rather than queued.
	e := newBuiltin(t, Policy{MaxConcurrent: 1})

	release := make(chan struct{})
	entered := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = e.Execute(context.Background(), "svc", func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()

	<-entered // first call now holds the only slot
	err := e.Execute(context.Background(), "svc", func(context.Context) error { return nil })
	assert.Error(t, err).Is(ErrBulkheadFull)

	close(release)
	wg.Wait()

	// Slot freed: a subsequent call succeeds again.
	assert.Error(t, e.Execute(context.Background(), "svc", func(context.Context) error { return nil })).Nil()
}

func TestFallbackDegradesOnRejection(t *testing.T) {
	// A tripped breaker rejects the call; degrade turns the rejection into a
	// graceful result and sees the triggering error.
	e := newBuiltin(t, Policy{ErrorThreshold: 1, OpenDuration: time.Minute})
	boom := errors.New("boom")

	// Trip the breaker.
	assert.Error(t, e.Execute(context.Background(), "svc", func(context.Context) error { return boom })).Is(boom)

	var seen error
	err := Fallback(context.Background(), e, "svc",
		func(context.Context) error { return errors.New("should not run") },
		func(_ context.Context, cause error) error { seen = cause; return nil })
	assert.Error(t, err).Nil()
	assert.Error(t, seen).Is(ErrCircuitOpen)
}

func TestFallbackNilExecStillDegrades(t *testing.T) {
	// With no executor the call runs directly, and a failure still reaches degrade.
	boom := errors.New("boom")
	err := Fallback(context.Background(), nil, "svc",
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
	got := NewDialer(base, nil, "svc")
	_, err := got(context.Background(), "tcp", "x")
	assert.Error(t, err).Nil()
	assert.That(t, called).True()
}

func TestDialerBreakerOpensOnDialFailures(t *testing.T) {
	dialErr := errors.New("connection refused")
	base := DialFunc(func(context.Context, string, string) (net.Conn, error) { return nil, dialErr })

	e := newBuiltin(t, Policy{ErrorThreshold: 2, OpenDuration: time.Minute})
	dial := NewDialer(base, e, "svc")

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
	e := newBuiltin(t, Policy{
		MaxRetries:      1,
		InitialInterval: 40 * time.Millisecond,
	})
	var attempts int
	var firstSaw, secondSaw time.Duration
	start := time.Now()
	err := e.Execute(context.Background(), "svc", func(context.Context) error {
		attempts++
		if attempts == 1 {
			firstSaw = time.Since(start)
		} else {
			secondSaw = time.Since(start)
		}
		if attempts < 2 {
			return errors.New("transient")
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
	e := newBuiltin(t, Policy{
		MaxRetries:      20,
		InitialInterval: 20 * time.Millisecond,
		MaxDuration:     60 * time.Millisecond,
	})
	var attempts int
	_ = e.Execute(context.Background(), "svc", func(context.Context) error {
		attempts++
		return errors.New("always fails")
	})
	// Not all 21 attempts ran — the budget cut the loop short.
	assert.That(t, attempts < 21).True()
	assert.That(t, attempts >= 1).True()
}

func TestRetryPredicateSuppressesNonRetryable(t *testing.T) {
	// A predicate that says "do not retry" stops the loop after the first
	// failure, even though MaxRetries would allow more attempts.
	e := newBuiltin(t, Policy{
		MaxRetries:     3,
		RetryPredicate: func(error) bool { return false },
	})
	var attempts int
	err := e.Execute(context.Background(), "svc", func(context.Context) error {
		attempts++
		return errors.New("nope")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

type nonRetryableErr struct{}

func (nonRetryableErr) Error() string   { return "permanent" }
func (nonRetryableErr) Retryable() bool { return false }

func TestRetryableErrorOverridesPredicate(t *testing.T) {
	// A Retryable() false error wins over a predicate that would retry it.
	e := newBuiltin(t, Policy{
		MaxRetries:     3,
		RetryPredicate: func(error) bool { return true },
	})
	var attempts int
	_ = e.Execute(context.Background(), "svc", func(context.Context) error {
		attempts++
		return nonRetryableErr{}
	})
	assert.That(t, attempts).Equal(1)
}

func TestHalfOpenAdmitsSingleTrialConcurrent(t *testing.T) {
	// Regression for the bool-flag half-open bug: once cool-down elapses, at
	// most ONE trial is admitted even under concurrency. A second concurrent
	// caller is treated as still-open.
	e := newBuiltin(t, Policy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	// Trip the breaker.
	_ = e.Execute(context.Background(), "svc", func(context.Context) error {
		return errors.New("boom")
	})
	time.Sleep(40 * time.Millisecond) // cool-down elapses -> half-open

	// Two concurrent calls: the first that reaches the half-open gate runs fn;
	// the other must be rejected as ErrCircuitOpen (no second trial permit).
	var wg sync.WaitGroup
	var ran, rejected int32
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := e.Execute(context.Background(), "svc", func(context.Context) error {
				atomic.AddInt32(&ran, 1)
				// Hold long enough that the sibling surely evaluates allow() too.
				time.Sleep(20 * time.Millisecond)
				return nil // trial succeeds -> closes
			})
			if errors.Is(err, ErrCircuitOpen) {
				atomic.AddInt32(&rejected, 1)
			}
		}()
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
		BreakerStrategy:    BreakerErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		BreakerWindow:      time.Second,
		OpenDuration:       time.Minute,
	})
	// Interleave fail/success: 4 fails out of 8 => 50% ratio.
	for i := range 8 {
		err := e.Execute(context.Background(), "svc", func(context.Context) error {
			if i%2 == 0 {
				return errors.New("fail")
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
	e := newBuiltin(t, Policy{
		ErrorThreshold: 3,
		MaxRetries:     2,
		OpenDuration:   time.Minute,
		RetryPredicate: func(error) bool { return true },
	})
	fail := func() error {
		return e.Execute(context.Background(), "svc", func(context.Context) error {
			return errors.New("boom")
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
