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

// fault_contract_test.go pins what an INJECTED fault does to this layer: it is
// retried, classified and counted exactly like a real failure of the same
// shape. The composition is the production one — resilience outside, the
// injector inside ([Runtime.ExecutorFor]) — so the dependency runs inward
// (resilience → fault) and the fault package's own tests stay free of this one.

package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/stdlib/testing/assert"
)

// faultExec composes the injector under this layer for service "svc", with cfg
// as the whole-service policy of its capability.
func faultExec(t *testing.T, cfg any, conf kfault.Config) chain.Executor {
	t.Helper()
	inj := fault.NewManager()
	inj.Refresh(conf)
	return inj.WrapClientExecutor(newBuiltin(t, cfg))
}

// countFn counts the real call and returns ret — the terminal a test observes.
func countFn(calls *int32, ret error) func(context.Context) error {
	return func(context.Context) error {
		atomic.AddInt32(calls, 1)
		return ret
	}
}

// TestInjectedFaultIsRetried: every attempt is injected, so the real fn never
// runs and the retry loop exhausts MaxRetries+1 attempts.
func TestInjectedFaultIsRetried(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}}, kfault.Config{
		Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}},
	})
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0)) // fn never reached
}

// TestInjectedErrorIsRetryable confirms the Retryable hook is set so the executor
// treats injected faults as retryable.
func TestInjectedErrorIsRetryable(t *testing.T) {
	var r Retryable
	assert.That(t, errors.As(kfault.ErrInjected, &r)).True()
	assert.That(t, r.Retryable()).True()
	assert.That(t, ShouldRetry(kfault.ErrInjected)).True()
}

// TestInjectedFaultTripsBreaker: with a tight breaker, injected faults trip it
// open and subsequent calls are rejected as ErrCircuitOpen without touching fn —
// the closed loop (fault → breaker → neutral sentinel) holds.
func TestInjectedFaultTripsBreaker(t *testing.T) {
	exec := faultExec(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Second}, kfault.Config{
		Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}},
	})
	var calls int32
	_ = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil)) // trips the breaker
	firstCalls := atomic.LoadInt32(&calls)

	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, chain.ErrCircuitOpen)).True()
	// fn not called on the rejected attempt (only the first call's attempts ran).
	assert.That(t, atomic.LoadInt32(&calls)).Equal(firstCalls)
}

// TestInjectedLatencyHonoursTimeout: injected latency sleeps, and the call-level
// timeout cancels the sleep instead of retrying forever.
func TestInjectedLatencyHonoursTimeout(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Timeout: 50 * time.Millisecond}, kfault.Config{
		Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 0, Class: kfault.ClassTransient, Error: kfault.ErrorGeneric, Latency: 200 * time.Millisecond}},
	})
	var calls int32
	start := time.Now()
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	elapsed := time.Since(start)
	// The 200ms latency exceeds the 50ms deadline: the sleep is cancelled and the
	// call fails without fn running.
	assert.Error(t, err).NotNil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0))
	assert.That(t, elapsed < 200*time.Millisecond).True()
}

// TestFireOnAnEarlierAttemptSurvives pins that the fire is written PER ATTEMPT,
// not once after Execute returns: the record is call-level, so a later attempt
// that does NOT fire must not erase an earlier one that did.
//
// MaxAffected 1 makes that case deterministic: the first attempt fires (spending
// the guardrail's budget), the retry sees the budget gone and runs clean, so the
// call SUCCEEDS with a fire on its record.
func TestFireOnAnEarlierAttemptSurvives(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 1}}, kfault.Config{
		Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxAffected: 1}},
	})
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))

	assert.Error(t, err).Nil()                 // the retry succeeded
	assert.Number(t, int(calls)).Equal(1)      // fn ran on the clean retry
	assert.Number(t, len(rec.Rounds)).Equal(2) // two attempts recorded
	assert.That(t, rec.Fault).NotNil()         // the first attempt's fire survived
	assert.String(t, rec.Fault.Error).Equal("generic")
}

// TestInjectedClassClassification pins the two independent questions a class
// answers: may I retry it, and does it count against the breaker.
func TestInjectedClassClassification(t *testing.T) {
	cases := []struct {
		class        string
		retry, count bool
	}{
		{"", true, true}, // zero value keeps the historical behaviour
		{kfault.ClassTransient, true, true},
		{kfault.ClassPermanent, false, true},
		{kfault.ClassBusiness, false, false},
	}
	for _, tc := range cases {
		err := &kfault.InjectedError{Kind: "generic", Class: tc.class}
		assert.That(t, ShouldRetry(err)).Equal(tc.retry)
		assert.That(t, CountsAsFailure(err)).Equal(tc.count)
	}
	assert.That(t, CountsAsFailure(nil)).False()
}

// TestPermanentInjectionIsNotRetried: class=permanent suppresses retry — the
// path "this failure is deterministic, retrying is pointless".
func TestPermanentInjectionIsNotRetried(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 3}}, kfault.Config{
		Rules: []kfault.Rule{{Name: "permanent", Service: "svc", Rate: 1, Error: "generic", Class: kfault.ClassPermanent}},
	})
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, int(calls)).Equal(0)      // fn never reached
	assert.Number(t, len(rec.Rounds)).Equal(1) // exactly one attempt
}

// TestBusinessInjectionDoesNotTripBreaker: class=business is a caller mistake, so
// even a threshold-1 breaker must ignore it and stay closed.
func TestBusinessInjectionDoesNotTripBreaker(t *testing.T) {
	exec := faultExec(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Second}, kfault.Config{
		Rules: []kfault.Rule{{Name: "business", Service: "svc", Rate: 1, Error: "generic", Class: kfault.ClassBusiness}},
	})
	for range 3 {
		err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))
		assert.That(t, kfault.IsInjected(err)).True()
		assert.That(t, errors.Is(err, chain.ErrCircuitOpen)).False()
	}
}
