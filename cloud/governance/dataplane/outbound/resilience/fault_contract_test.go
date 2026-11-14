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
func faultExec(t *testing.T, cfg any, conf fault.ClientConfig) chain.Executor {
	t.Helper()
	inj := fault.NewManager()
	inj.Refresh(conf)
	return inj.WrapClientExecutor(newBuiltin(t, cfg))
}

// countFn counts the real call and returns ret — the terminal a test observes.
func countFn(calls *int32, ret error) func(context.Context, *chain.Round) error {
	return func(context.Context, *chain.Round) error {
		atomic.AddInt32(calls, 1)
		return ret
	}
}

// TestInjectedFaultIsRetried: every attempt is injected, so the real fn never
// runs and the retry loop exhausts MaxRetries+1 attempts.
func TestInjectedFaultIsRetried(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}}, fault.ClientConfig{Rules: []fault.ClientRule{{RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Rate: 1}, Class: kfault.ClassTransient, Error: "generic"}}})
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
	exec := faultExec(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Second}, fault.ClientConfig{Rules: []fault.ClientRule{{RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Rate: 1}, Class: kfault.ClassTransient, Error: "generic"}}})
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
	exec := faultExec(t, TimeoutRetry{Timeout: 50 * time.Millisecond}, fault.ClientConfig{Rules: []fault.ClientRule{{RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Rate: 0, Latency: 200 * time.Millisecond}, Class: kfault.ClassTransient, Error: kfault.ErrorGeneric}}})
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
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 1}}, fault.ClientConfig{Rules: []fault.ClientRule{{RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Rate: 1, MaxAffected: 1}, Class: kfault.ClassTransient, Error: "generic"}}})
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))

	assert.Error(t, err).Nil()                   // the retry succeeded
	assert.Number(t, int(calls)).Equal(1)        // fn ran on the clean retry
	assert.Number(t, len(rec.Rounds)).Equal(2)   // two attempts recorded
	assert.That(t, rec.Rounds[0].Fault).NotNil() // the first attempt's fire survived
	assert.String(t, rec.Rounds[0].Fault.Error).Equal("generic")
}

// TestInjectedClassRetryClassification pins the one question an OUTBOUND class
// answers: may I retry it. business is an inbound-only class — the document
// validation rejects it on the client side (see controlplane/fault).
func TestInjectedClassRetryClassification(t *testing.T) {
	cases := []struct {
		class string
		retry bool
	}{
		{"", true}, // zero value keeps the historical behaviour
		{kfault.ClassTransient, true},
		{kfault.ClassPermanent, false},
	}
	for _, tc := range cases {
		err := &kfault.InjectedError{Kind: kfault.ErrorGeneric, Class: tc.class}
		assert.That(t, ShouldRetry(err)).Equal(tc.retry)
	}
}

// TestPermanentInjectionIsNotRetried: class=permanent suppresses retry — the
// path "this failure is deterministic, retrying is pointless".
func TestPermanentInjectionIsNotRetried(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 3}}, fault.ClientConfig{Rules: []fault.ClientRule{{RuleCore: kfault.RuleCore{Name: "permanent", Service: "svc", Rate: 1}, Error: "generic", Class: kfault.ClassPermanent}}})
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, int(calls)).Equal(0)      // fn never reached
	assert.Number(t, len(rec.Rounds)).Equal(1) // exactly one attempt
}

// retryableFailure is a real failure the retry loop will retry, used to make an
// attempt sequence deterministic without using the injector's own error.
type retryableFailure struct{}

func (retryableFailure) Error() string   { return "transient failure" }
func (retryableFailure) Retryable() bool { return true }

// TestInjectedEveryCountsAttempts pins the per-ATTEMPT semantics of the dose
// knobs: `every` counts each attempt, and two logical calls with one attempt
// each consume two sequence numbers (the second call fires, the first does not).
func TestInjectedEveryCountsAttempts(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{}, fault.ClientConfig{
		Rules: []fault.ClientRule{{
			RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Every: 2},
			Error:    kfault.ErrorGeneric, Class: kfault.ClassTransient,
		}},
	})
	var calls int32

	// Call 1: attempt #1 does not fire; the real fn runs and succeeds.
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{Service: "svc"},
		new(chain.Recorder), countFn(&calls, nil))).Nil()
	assert.Number(t, int(atomic.LoadInt32(&calls))).Equal(1)

	// Call 2: attempt #2 fires; the real fn does not run.
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"},
		new(chain.Recorder), countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, int(atomic.LoadInt32(&calls))).Equal(1)
}

// TestInjectedEveryCountsRetryAttempts pins the other half: a RETRY is another
// attempt, so the injector can fire on the second attempt of the SAME logical
// call. This is the per-request model Envoy/Istio use, and it is why the
// call-level failure rate is a composition, not the rule's own rate.
func TestInjectedEveryCountsRetryAttempts(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 1}}, fault.ClientConfig{
		Rules: []fault.ClientRule{{
			RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Every: 2},
			Error:    kfault.ErrorGeneric, Class: kfault.ClassTransient,
		}},
	})
	var calls int32
	fn := func(context.Context, *chain.Round) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			return retryableFailure{} // the first attempt fails for real -> retry
		}
		return nil
	}
	rec := &chain.Recorder{}
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, fn)

	// Attempt 1 did not fire (fn ran); attempt 2 fired (fn did not run).
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, int(atomic.LoadInt32(&calls))).Equal(1)
	assert.Number(t, len(rec.Rounds)).Equal(2)
	assert.That(t, rec.Rounds[0].Fault).Nil()
	assert.That(t, rec.Rounds[1].Fault).NotNil()
	assert.That(t, rec.Rounds[1].Fault.Inject).True()
}

// TestInjectedMaxAffectedCountsAttempts pins that the blast-radius cap counts
// ATTEMPTS: the first attempt spends the only unit, so the retry runs clean and
// the call succeeds. A call-level count would have kept firing.
func TestInjectedMaxAffectedCountsAttempts(t *testing.T) {
	exec := faultExec(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}}, fault.ClientConfig{
		Rules: []fault.ClientRule{{
			RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Rate: 1, MaxAffected: 1},
			Error:    kfault.ErrorGeneric, Class: kfault.ClassTransient,
		}},
	})
	var calls int32
	rec := &chain.Recorder{}
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))

	assert.Error(t, err).Nil()                               // the clean retry succeeded
	assert.Number(t, int(atomic.LoadInt32(&calls))).Equal(1) // fn ran on the retry
	assert.Number(t, len(rec.Rounds)).Equal(2)               // injected attempt + clean retry
	assert.That(t, rec.Rounds[0].Fault).NotNil()             // the first attempt was injected
	assert.That(t, rec.Rounds[1].Fault).Nil()                // the retry ran clean
}
