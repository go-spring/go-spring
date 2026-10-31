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

package StarterGovernanceSentinel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	breaker "go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"

	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestDriverBuilds proves the module's backend is usable directly: the exported
// constructor returns a working Driver, so a caller outside the container can
// build executors with no compile-time dependency on the sentinel library. The
// container-level half — that importing this module makes "sentinel" selectable
// by spring.resilience.driver — is covered by the driver-directory test.
func TestDriverBuilds(t *testing.T) {
	d := NewSentinelDriver()
	assert.That(t, d).NotNil()
	m := resilience.NewManager(d)
	e := m.ExecutorFor("svc-builds")
	assert.That(t, e).NotNil()
}

// newExec builds a sentinel executor for service. Each test names its own
// service: sentinel's rule registry AND its per-resource stat window are
// process-global, so a shared resource name would let one test's calls trip
// another test's rule.
func newExec(t *testing.T, service string, cfg any) chain.Executor {
	m := resilience.NewManager(NewSentinelDriver())
	switch v := cfg.(type) {
	case nil:
	case resilience.Limits:
		assert.Error(t, m.ApplyLimits(resilience.LimitsSettings{Default: v})).Nil()
	case resilience.BreakerPolicy:
		assert.Error(t, m.ApplyBreaker(resilience.BreakerSettings{Default: v})).Nil()
	case resilience.RateLimitPolicy:
		assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: v})).Nil()
	default:
		t.Fatalf("unsupported test policy %T", cfg)
	}
	return m.ExecutorFor(service)
}

func TestSentinelPassThrough(t *testing.T) {
	e := newExec(t, "svc-passthrough", nil)
	var calls int
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		calls++
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, calls).Equal(1)
}

func TestSentinelRateLimit(t *testing.T) {
	// Threshold 2 QPS: the third call within the same second is rejected as a
	// flow-control block, surfaced through the neutral ErrRateLimited.
	e := newExec(t, "svc-ratelimit", resilience.RateLimitPolicy{RateLimit: 2})
	run := func() error {
		return e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	}
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(chain.ErrRateLimited)
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

	e := newExec(t, "sentinel:test", resilience.Limits{Timeout: time.Second, Retry: chain.RetryPolicy{MaxRetries: 3}})
	client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, e, "svc", nil)}

	resp, err := client.Get(srv.URL)
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	_ = resp.Body.Close()
	assert.That(t, atomic.LoadInt32(&hits)).Equal(int32(3))
}

// TestSentinelErrorRateBreaker verifies the error-rate strategy is mapped onto
// sentinel's ErrorRatio rule: it trips on failure ratio with enough samples
// even when successes are interleaved (so a consecutive counter would not trip).
func TestSentinelErrorRateBreaker(t *testing.T) {
	e := newExec(t, "svc-errrate", resilience.BreakerPolicy{
		BreakerStrategy:    breaker.StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		BreakerWindow:      time.Second,
		OpenDuration:       time.Minute,
	})
	tripped := false
	for i := range 10 {
		err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
			if i%2 == 0 {
				return errors.New("fail")
			}
			return nil
		})
		if errors.Is(err, chain.ErrCircuitOpen) {
			tripped = true
			break
		}
	}
	assert.That(t, tripped).True()
}

// A caller that walks away is not a verdict about the downstream: sentinel must
// not trace it as a failure. With ErrorThreshold 1, one traced cancel would trip.
func TestSentinelCallerCancelDoesNotTrip(t *testing.T) {
	e := newExec(t, "svc-cancel", resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := e.Execute(ctx, chain.Operation{}, nil, func(ctx context.Context) error {
		return ctx.Err()
	})
	assert.Error(t, err).Is(context.Canceled)

	// Had the cancellation been traced, this call would be short-circuited.
	err = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		return nil
	})
	assert.Error(t, err).Nil()
}

// TestSentinelHalfOpenRecovery verifies sentinel's half-open behavior maps onto
// the neutral Policy: after cool-down a successful trial closes the circuit, and
// a failing trial re-opens it. The breaker is configured with ProbeNum=1 (see
// loadBreakerRule) so a single probe decides recovery — the closest sentinel
// equivalent to the builtin's strict single-permit half-open gate.
func TestSentinelHalfOpenRecovery(t *testing.T) {
	e := newExec(t, "svc-halfopen", resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	// Trip the breaker.
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		return errors.New("boom")
	})
	// Open: short-circuited.
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)

	// After cool-down a successful trial closes the circuit.
	time.Sleep(40 * time.Millisecond)
	err = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
	// Now closed: a normal call runs.
	err = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
}

// TestSentinelHalfOpenReopensOnFailedTrial verifies the other half of the
// half-open contract: a failing trial re-opens the cool-down window.
func TestSentinelHalfOpenReopensOnFailedTrial(t *testing.T) {
	e := newExec(t, "svc-reopen", resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})

	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		return errors.New("boom")
	})
	time.Sleep(40 * time.Millisecond) // half-open

	// Failing trial re-opens the circuit.
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		return errors.New("still bad")
	})
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}

// breakerStats reads the driver's own mirror for a service. The sentinel driver
// cannot query sentinel-golang's state, so it records transitions from the
// process-wide listener; that mirror is what the driver's metrics publish.
func breakerStats(t *testing.T, service string) breaker.Stats {
	t.Helper()
	v, ok := breakerRoutes.Load(service)
	if !ok {
		t.Fatalf("no breaker mirror for %q", service)
	}
	return v.(*breakerMirror).stats()
}

// TestSentinelBreakerState verifies the sentinel driver records the same
// closed -> open -> half_open -> closed progression as the builtin, in the
// mirror its metrics report.
func TestSentinelBreakerState(t *testing.T) {
	e := newExec(t, "svc-evt", resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: 30 * time.Millisecond})
	defer func() { _ = e.Close() }()

	// Trip: closed -> open.
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return errors.New("boom") })
	s := breakerStats(t, "svc-evt")
	assert.That(t, s.State).Equal(breaker.StateOpen)
	assert.That(t, s.Trips).Equal(uint64(1))

	// Breaker open: rejected before fn runs, no transition.
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	s = breakerStats(t, "svc-evt")
	assert.That(t, s.Trips).Equal(uint64(1))

	// After cool-down a successful trial: open -> half_open -> closed.
	time.Sleep(40 * time.Millisecond)
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	s = breakerStats(t, "svc-evt")
	assert.That(t, s.State).Equal(breaker.StateClosed)
	assert.That(t, s.HalfOpens).Equal(uint64(1))
	assert.That(t, s.Recovers).Equal(uint64(1))
}

// The rule-mapping tests below pin the TRANSLATION (rules.go): after one
// Execute the sentinel registry must hold exactly the rule shapes the neutral
// policy asks for — unit-normalized thresholds, the right control behaviors and
// breaker strategies — because a silently wrong field is worse than an absent
// rule: it protects against the wrong thing with confidence.

// runOnce drives one Execute purely to trigger lazy rule loading; the call's
// own outcome is irrelevant here (a warm-up rule, for instance, legitimately
// rejects the very first call — cold start is the point of warm-up).
func runOnce(t *testing.T, e chain.Executor) {
	t.Helper()
	_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
}

// TestSentinelFlowRuleQueueing pins the queueing translation: RateLimitMaxWait
// selects Throttling with MaxQueueingTimeMs, and Window scales both the stat
// interval AND the threshold (per-second → per-interval) so the effective rate
// is unchanged.
func TestSentinelFlowRuleQueueing(t *testing.T) {
	const svc = "svc-rule-queue"
	e := newExec(t, svc, resilience.RateLimitPolicy{
		RateLimit:        10,
		Window:           2 * time.Second,
		RateLimitMaxWait: 250 * time.Millisecond,
	})
	runOnce(t, e)

	rules := flow.GetRulesOfResource(svc)
	assert.That(t, len(rules)).Equal(1)
	r := rules[0]
	assert.That(t, r.StatIntervalInMs).Equal(uint32(2000))
	assert.That(t, r.Threshold).Equal(float64(20)) // 10/s * 2s
	assert.That(t, r.ControlBehavior).Equal(flow.Throttling)
	assert.That(t, r.MaxQueueingTimeMs).Equal(uint32(250))
}

// TestSentinelFlowRuleWarmUp pins the warm-up translation: WarmUpPeriodSec and
// ColdFactor land on the rule, and warm-up forces Reject (sentinel pairs WarmUp
// with Reject only) even when a max-wait was also set.
func TestSentinelFlowRuleWarmUp(t *testing.T) {
	const svc = "svc-rule-warmup"
	e := newExec(t, svc, resilience.RateLimitPolicy{
		RateLimit:               5,
		RateLimitWarmUpDuration: 30 * time.Second,
		WarmUpColdFactor:        10,
		RateLimitMaxWait:        time.Second, // must lose to warm-up
	})
	runOnce(t, e)

	rules := flow.GetRulesOfResource(svc)
	assert.That(t, len(rules)).Equal(1)
	r := rules[0]
	assert.That(t, r.TokenCalculateStrategy).Equal(flow.WarmUp)
	assert.That(t, r.ControlBehavior).Equal(flow.Reject)
	assert.That(t, r.WarmUpPeriodSec).Equal(uint32(30))
	assert.That(t, r.WarmUpColdFactor).Equal(uint32(10))
}

// TestSentinelBreakerRuleSlowCall pins the slow-call translation: the resolved
// strategy selects sentinel's SlowRequestRatio with MaxAllowedRtMs from the
// slow-call threshold, and HalfOpenRequests > 1 widens ProbeNum.
func TestSentinelBreakerRuleSlowCall(t *testing.T) {
	const svc = "svc-rule-slowcall"
	e := newExec(t, svc, resilience.BreakerPolicy{
		BreakerStrategy:           breaker.StrategySlowCall,
		SlowCallDurationThreshold: 200 * time.Millisecond,
		SlowCallRateThreshold:     0.5,
		MinRequests:               3,
		BreakerWindow:             2 * time.Second,
		OpenDuration:              time.Minute,
		HalfOpenRequests:          2,
	})
	runOnce(t, e)

	rules := circuitbreaker.GetRulesOfResource(svc)
	assert.That(t, len(rules)).Equal(1)
	r := rules[0]
	assert.That(t, r.Strategy).Equal(circuitbreaker.SlowRequestRatio)
	assert.That(t, r.MaxAllowedRtMs).Equal(uint64(200))
	assert.That(t, r.Threshold).Equal(0.5)
	assert.That(t, r.MinRequestAmount).Equal(uint64(3))
	assert.That(t, r.StatIntervalMs).Equal(uint32(2000))
	assert.That(t, r.ProbeNum).Equal(uint64(2))
}

// TestSentinelBreakerRuleErrorCount pins the error-count translation: an exact
// windowed count with MinRequests honored, unlike the consecutive approximation.
func TestSentinelBreakerRuleErrorCount(t *testing.T) {
	const svc = "svc-rule-errcount"
	e := newExec(t, svc, resilience.BreakerPolicy{
		BreakerStrategy:  breaker.StrategyErrorCount,
		ErrorThreshold:   7,
		MinRequests:      2,
		HalfOpenRequests: 3,
	})
	runOnce(t, e)

	rules := circuitbreaker.GetRulesOfResource(svc)
	assert.That(t, len(rules)).Equal(1)
	r := rules[0]
	assert.That(t, r.Strategy).Equal(circuitbreaker.ErrorCount)
	assert.That(t, r.Threshold).Equal(float64(7))
	assert.That(t, r.MinRequestAmount).Equal(uint64(2))
	assert.That(t, r.ProbeNum).Equal(uint64(3))
}
