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
	"net/http"
	"net/http/httptest"
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
