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
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestDriverRegistered proves the whole point of the split: importing this
// module makes "sentinel" resolvable through the framework's registry, so any
// adapter selects it by name with no compile-time dependency on sentinel.
func TestDriverRegistered(t *testing.T) {
	d, err := resilience.GetDriver("sentinel")
	assert.Error(t, err).Nil()
	assert.That(t, d).NotNil()
}

func newExec(t *testing.T, p resilience.Policy) resilience.Executor {
	d, err := resilience.GetDriver("sentinel")
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

// TestSentinelBulkheadHeldAcrossRetries verifies the isolation slot is held for
// the whole Execute (retries included), matching the builtin driver and
// DESIGN.md §3. With MaxConcurrent=1 and a retrying call in flight, a second
// call must be rejected for the entire retry sequence — not admitted in the
// gap between attempts.
func TestSentinelBulkheadHeldAcrossRetries(t *testing.T) {
	e := newExec(t, resilience.Policy{MaxConcurrent: 1, MaxRetries: 3, InitialInterval: 20 * time.Millisecond})

	release := make(chan struct{})
	entered := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = e.Execute(context.Background(), "svc-bulk-retry", func(context.Context) error {
			close(entered)
			<-release // park with the slot held across what would be retries
			return nil
		})
	}()

	<-entered
	// While the first call holds the slot (parked inside fn), the second is
	// rejected — proving the bulkhead is held for the duration of Execute.
	err := e.Execute(context.Background(), "svc-bulk-retry", func(context.Context) error { return nil })
	assert.Error(t, err).Is(resilience.ErrBulkheadFull)

	close(release)
	wg.Wait()
}

// TestSentinelErrorRateBreaker verifies the error-rate strategy is mapped onto
// sentinel's ErrorRatio rule: it trips on failure ratio with enough samples
// even when successes are interleaved (so a consecutive counter would not trip).
func TestSentinelErrorRateBreaker(t *testing.T) {
	e := newExec(t, resilience.Policy{
		BreakerStrategy:    resilience.BreakerErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		BreakerWindow:      time.Second,
		OpenDuration:       time.Minute,
	})
	tripped := false
	for i := range 10 {
		err := e.Execute(context.Background(), "svc-errrate", func(context.Context) error {
			if i%2 == 0 {
				return errors.New("fail")
			}
			return nil
		})
		if errors.Is(err, resilience.ErrCircuitOpen) {
			tripped = true
			break
		}
	}
	assert.That(t, tripped).True()
}

// TestSentinelHalfOpenRecovery verifies sentinel's half-open behavior maps onto
// the neutral Policy: after cool-down a successful trial closes the circuit, and
// a failing trial re-opens it. The breaker is configured with ProbeNum=1 (see
// loadBreakerRule) so a single probe decides recovery — the closest sentinel
// equivalent to the builtin's strict single-permit half-open gate.
func TestSentinelHalfOpenRecovery(t *testing.T) {
	e := newExec(t, resilience.Policy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	// Trip the breaker.
	_ = e.Execute(context.Background(), "svc-halfopen", func(context.Context) error {
		return errors.New("boom")
	})
	// Open: short-circuited.
	err := e.Execute(context.Background(), "svc-halfopen", func(context.Context) error { return nil })
	assert.Error(t, err).Is(resilience.ErrCircuitOpen)

	// After cool-down a successful trial closes the circuit.
	time.Sleep(40 * time.Millisecond)
	err = e.Execute(context.Background(), "svc-halfopen", func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
	// Now closed: a normal call runs.
	err = e.Execute(context.Background(), "svc-halfopen", func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
}

// TestSentinelHalfOpenReopensOnFailedTrial verifies the other half of the
// half-open contract: a failing trial re-opens the cool-down window.
func TestSentinelHalfOpenReopensOnFailedTrial(t *testing.T) {
	e := newExec(t, resilience.Policy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	_ = e.Execute(context.Background(), "svc-reopen", func(context.Context) error {
		return errors.New("boom")
	})
	time.Sleep(40 * time.Millisecond) // half-open

	// Failing trial re-opens the circuit.
	_ = e.Execute(context.Background(), "svc-reopen", func(context.Context) error {
		return errors.New("still bad")
	})
	err := e.Execute(context.Background(), "svc-reopen", func(context.Context) error { return nil })
	assert.Error(t, err).Is(resilience.ErrCircuitOpen)
}

// sentinelRec captures breaker state transitions routed from sentinel-golang via
// the global routeListener, for TestSentinelBreakerEvents.
type sentinelRec struct {
	events []struct{ resource, from, to string }
}

func (l *sentinelRec) OnBreakerStateChange(resource string, from, to resilience.BreakerState) {
	l.events = append(l.events, struct{ resource, from, to string }{resource, from.String(), to.String()})
}

// TestSentinelBreakerEvents verifies the sentinel driver emits the same
// closed->open->half-open->closed transitions as the builtin, routed through the
// global routeListener to a listener attached via SetBreakerEventListener.
func TestSentinelBreakerEvents(t *testing.T) {
	e := newExec(t, resilience.Policy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})
	rec := &sentinelRec{}
	e.(resilience.BreakerEventListenerSetter).SetBreakerEventListener(rec)

	// Trip: closed -> open.
	_ = e.Execute(context.Background(), "svc-evt", func(context.Context) error { return errors.New("boom") })
	assert.That(t, len(rec.events)).Equal(1)
	assert.That(t, rec.events[0].to).Equal("open")

	// Breaker open: rejected before fn runs, no transition.
	_ = e.Execute(context.Background(), "svc-evt", func(context.Context) error { return nil })
	assert.That(t, len(rec.events)).Equal(1)

	// After cool-down a successful trial: open -> half-open -> closed.
	time.Sleep(40 * time.Millisecond)
	_ = e.Execute(context.Background(), "svc-evt", func(context.Context) error { return nil })
	assert.That(t, len(rec.events)).Equal(3)
	assert.That(t, rec.events[1].to).Equal("half_open")
	assert.That(t, rec.events[2].to).Equal("closed")
}
