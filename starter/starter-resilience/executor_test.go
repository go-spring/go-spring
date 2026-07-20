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

package StarterResilience

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/spring/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestDriverRegistered proves the whole point of the split: importing this
// module makes "sentinel" resolvable through the framework's registry, so any
// adapter selects it by name with no compile-time dependency on sentinel.
func TestDriverRegistered(t *testing.T) {
	d, err := resilience.MustGetDriver("sentinel")
	assert.Error(t, err).Nil()
	assert.That(t, d).NotNil()
}

func newExec(t *testing.T, p resilience.Policy) resilience.Executor {
	d, err := resilience.MustGetDriver("sentinel")
	assert.Error(t, err).Nil()
	e, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return e
}

func TestSentinelPassThrough(t *testing.T) {
	e := newExec(t, resilience.Policy{})
	var calls int
	err := e.Execute(context.Background(), "svc-passthrough", func(context.Context) error {
		calls++
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, calls).Equal(1)
}

func TestSentinelRateLimit(t *testing.T) {
	// Threshold 2 QPS: the third call within the same second is rejected as a
	// flow-control block, surfaced through the neutral ErrRateLimited.
	e := newExec(t, resilience.Policy{RateLimit: 2})
	run := func() error {
		return e.Execute(context.Background(), "svc-ratelimit", func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(resilience.ErrRateLimited)
}

// TestSentinelRoundTripperRetry drives the sentinel executor through the exact
// same resilience.NewRoundTripper seam that starter-oauth2-client uses; the only
// difference from the builtin driver is the registry name. A flaky endpoint that
// 500s twice is recovered within max-retries.
func TestSentinelRoundTripperRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := newExec(t, resilience.Policy{MaxRetries: 3, Timeout: time.Second})
	client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, e, nil)}

	resp, err := client.Get(srv.URL)
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	_ = resp.Body.Close()
	assert.That(t, atomic.LoadInt32(&hits)).Equal(int32(3))
}

// TestSentinelBulkhead proves the neutral MaxConcurrent knob is carried by
// sentinel's isolation (concurrency) rule: with a limit of 1, a second call made
// while the first is still in-flight is rejected as the neutral ErrBulkheadFull.
func TestSentinelBulkhead(t *testing.T) {
	e := newExec(t, resilience.Policy{MaxConcurrent: 1})

	release := make(chan struct{})
	entered := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = e.Execute(context.Background(), "svc-bulkhead", func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()

	<-entered // first call holds the only concurrency slot
	err := e.Execute(context.Background(), "svc-bulkhead", func(context.Context) error { return nil })
	assert.Error(t, err).Is(resilience.ErrBulkheadFull)

	close(release)
	wg.Wait()

	// Slot freed: a subsequent call is admitted again.
	assert.Error(t, e.Execute(context.Background(), "svc-bulkhead", func(context.Context) error { return nil })).Nil()
}
