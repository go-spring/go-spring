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
	d, err := MustGetDriver("default")
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
	assert.Panic(t, func() { RegisterDriver("default", builtinDriver{}) }, "already registered")
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

func TestHandlerNilExecIsPassThrough(t *testing.T) {
	var served bool
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true })
	h := NewHandler(next, nil, nil)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, served).True()
}

func TestHandlerRateLimitReturns429(t *testing.T) {
	// Burst of 1, no refill in the test window: the second request is rejected
	// at admission with 429 and never reaches the business handler.
	e := newBuiltin(t, Policy{RateLimit: 1, Burst: 1})
	var served int32
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&served, 1)
		_, _ = w.Write([]byte("ok"))
	})
	h := NewHandler(next, e, func(*http.Request) string { return "svc" })

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, rec1.Code).Equal(http.StatusOK)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, rec2.Code).Equal(http.StatusTooManyRequests)
	assert.That(t, atomic.LoadInt32(&served)).Equal(int32(1))
}

func TestHandler5xxTripsBreakerTo503(t *testing.T) {
	// The handler always 500s; after ErrorThreshold failures the breaker opens
	// and admission returns 503 without invoking the handler again.
	e := newBuiltin(t, Policy{ErrorThreshold: 1, OpenDuration: time.Minute})
	var served int32
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&served, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	h := NewHandler(next, e, func(*http.Request) string { return "svc" })

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, rec1.Code).Equal(http.StatusInternalServerError) // served, breaker records failure

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, rec2.Code).Equal(http.StatusServiceUnavailable) // now short-circuited
	assert.That(t, atomic.LoadInt32(&served)).Equal(int32(1))
}
