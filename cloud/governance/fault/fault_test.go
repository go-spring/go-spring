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

package fault

import (
	"context"
	"errors"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// newExec builds a default-driver executor with the given policy.
func newExec(t *testing.T, p resilience.Policy) resilience.Executor {
	t.Helper()
	d, err := resilience.GetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	t.Cleanup(func() { _ = exec.Close() })
	return exec
}

// countFn returns an operation fn that increments calls and returns a fixed
// error (or nil) so tests can assert how many times the real fn ran.
func countFn(counter *int32, err error) func(context.Context) error {
	return func(context.Context) error {
		atomic.AddInt32(counter, 1)
		return err
	}
}

// TestWrapExecutor_NilInputs confirms the zero-config transparency invariants:
// nil inner => nil; nil injector => a lazy faultExecutor that is a transparent
// pass-through until an injector is registered behind InjectorFor() (mirroring
// resilience.ExecutorFor's deferred resolution).
func TestWrapExecutor_NilInputs(t *testing.T) {
	t.Cleanup(func() { RegisterInjector(nil) })
	assert.That(t, WrapExecutorWith(nil, NewInjector(Config{Enabled: true})) == nil).True()
	exec := newExec(t, resilience.Policy{})
	wrapped := WrapExecutor(exec)
	assert.That(t, wrapped == exec).False() // lazy layer wraps, not returns inner
	// With no injector registered, Execute is a transparent pass-through.
	var calls int32
	err := wrapped.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_DisabledIsTransparent: Enabled=false => fn runs, no injection.
func TestInjector_DisabledIsTransparent(t *testing.T) {
	in := NewInjector(Config{}) // Enabled false
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)
	var calls int32
	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_RateZeroAlwaysSucceeds: Rate 0 with Enabled => never inject.
func TestInjector_RateZeroAlwaysSucceeds(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 0, Error: "generic"})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)
	var calls int32
	for range 10 {
		err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
		assert.Error(t, err).Nil()
	}
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(10))
}

// TestInjector_RateOneRetriesInjectedError: every attempt is injected, so the
// real fn never runs and the executor exhausts its retries (MaxRetries+1
// attempts) and returns the injected error.
func TestInjector_RateOneRetriesInjectedError(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"})
	// MaxRetries 2 => 3 attempts; no breaker so retry is the only path.
	exec := WrapExecutorWith(newExec(t, resilience.Policy{MaxRetries: 2}), in)
	var calls int32
	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, IsInjected(err)).True()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0)) // fn never reached
}

// TestInjectedError_Retryable confirms the Retryable hook is set so resilience
// treats injected faults as retryable.
func TestInjectedError_Retryable(t *testing.T) {
	var r resilience.Retryable
	assert.That(t, errors.As(ErrInjected, &r)).True()
	assert.That(t, r.Retryable()).True()
	assert.That(t, resilience.Policy{}.ShouldRetry(ErrInjected)).True()
}

// TestInjector_KindsSurfaceAsFamiliarErrors: the typed kinds wrap a real error
// so errors.Is matches the underlying sentinel (and the observe outcome mapping
// classifies the call the same way a real timeout/reset would).
func TestInjector_KindsSurfaceAsFamiliarErrors(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "timeout"})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)
	err := exec.Execute(context.Background(), "svc", countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True()
	assert.That(t, IsInjected(err)).True()

	in.SetConfig(Config{Enabled: true, Rate: 1, Error: "reset"})
	err = exec.Execute(context.Background(), "svc", countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
}

// TestInjector_BreakerOpensUnderFault: with a tight breaker, injected faults
// trip it open and subsequent calls are rejected as ErrCircuitOpen without
// touching fn — the closed loop (fault → breaker → neutral sentinel) holds.
func TestInjector_BreakerOpensUnderFault(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"})
	// ErrorThreshold 1 opens after the first failed attempt; OpenDuration long
	// enough that the second call sees it still open.
	exec := WrapExecutorWith(newExec(t, resilience.Policy{ErrorThreshold: 1, OpenDuration: time.Second}), in)
	var calls int32
	_ = exec.Execute(context.Background(), "svc", countFn(&calls, nil)) // trips the breaker
	firstCalls := atomic.LoadInt32(&calls)

	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, resilience.ErrCircuitOpen)).True()
	// fn not called on the rejected attempt (only the first call's attempts ran).
	assert.That(t, atomic.LoadInt32(&calls)).Equal(firstCalls)
}

// TestInjector_LatencySleepsAndCancels: injected latency sleeps, and a cancelled
// attempt context surfaces the context error instead of retrying forever.
func TestInjector_LatencySleepsAndCancels(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 0, Latency: 200 * time.Millisecond})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{Timeout: 50 * time.Millisecond}), in)
	var calls int32
	start := time.Now()
	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	elapsed := time.Since(start)
	// The 200ms latency exceeds the 50ms per-attempt timeout: the sleep is
	// cancelled by the attempt deadline and the call fails without fn running.
	assert.Error(t, err).NotNil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0))
	assert.That(t, elapsed < 200*time.Millisecond).True()
}

// TestInjector_HotSwap: toggling Enabled at runtime via SetConfig takes effect
// on the next operation (the Dync-driven path starters use).
func TestInjector_HotSwap(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)
	var calls int32
	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.That(t, IsInjected(err)).True()

	in.SetConfig(Config{Enabled: false}) // turn the fire off
	err = exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_ScopeGatesLoadTestTraffic verifies the Scope config restricts
// injection by the load-test marker on the call's context.
func TestInjector_ScopeGatesLoadTestTraffic(t *testing.T) {
	// Rate 1 + generic error => every in-scope call is faulted.
	mk := func(scope string) resilience.Executor {
		in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic", Scope: scope})
		return WrapExecutorWith(newExec(t, resilience.Policy{}), in)
	}
	realCtx := context.Background()
	loadCtx := traffic.WithLoadTest(context.Background(), "test")

	// Scope "" (default): both real and load-test traffic get faulted.
	all := mk("")
	assert.That(t, IsInjected(all.Execute(realCtx, "svc", countFn(new(int32), nil)))).True()
	assert.That(t, IsInjected(all.Execute(loadCtx, "svc", countFn(new(int32), nil)))).True()

	// Scope "real": real traffic faulted, load-test traffic passes through.
	real := mk("real")
	assert.That(t, IsInjected(real.Execute(realCtx, "svc", countFn(new(int32), nil)))).True()
	var ltCalls int32
	assert.Error(t, real.Execute(loadCtx, "svc", countFn(&ltCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&ltCalls)).Equal(int32(1)) // fn ran, no fault

	// Scope "loadtest": load-test traffic faulted, real traffic passes through.
	lt := mk("loadtest")
	assert.That(t, IsInjected(lt.Execute(loadCtx, "svc", countFn(new(int32), nil)))).True()
	var realCalls int32
	assert.Error(t, lt.Execute(realCtx, "svc", countFn(&realCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&realCalls)).Equal(int32(1))
}

// TestInjector_MaxAffectedCapsBlastRadius verifies MaxAffected stops injecting
// after the configured count of affected calls.
func TestInjector_MaxAffectedCapsBlastRadius(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic", MaxAffected: 3})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)

	// First three calls are faulted.
	for range 3 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), "svc", countFn(new(int32), nil)))).True()
	}
	// Fourth onward: guardrail tripped, call passes through untouched.
	var calls int32
	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_MaxDurationAutoOff verifies MaxDuration turns the fire off after
// the window elapses (a forgotten fault self-heals).
func TestInjector_MaxDurationAutoOff(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic", MaxDuration: 40 * time.Millisecond})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)

	// Within the window: faulted.
	assert.That(t, IsInjected(exec.Execute(context.Background(), "svc", countFn(new(int32), nil)))).True()
	// After the window: passes through.
	time.Sleep(60 * time.Millisecond)
	var calls int32
	err := exec.Execute(context.Background(), "svc", countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_NoGuardrailsUnchanged confirms the default (no guardrails) keeps
// the original behavior: every Rate-1 call faults, indefinitely.
func TestInjector_NoGuardrailsUnchanged(t *testing.T) {
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)
	for range 10 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), "svc", countFn(new(int32), nil)))).True()
	}
}

// TestApply_ServerSideFault verifies the server-side Apply seam: it injects
// latency+error per the injector's rules, honours Scope vs the load-test
// marker, and is a transparent pass-through for nil injector.
func TestApply_ServerSideFault(t *testing.T) {
	// nil injector => fn runs untouched.
	ran := false
	assert.Error(t, Apply(context.Background(), nil, "svc", func() error { ran = true; return nil })).Nil()
	assert.That(t, ran).True()

	// Rate 1 => injected error returned, fn NOT called.
	in := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"})
	ran = false
	err := Apply(context.Background(), in, "svc", func() error { ran = true; return nil })
	assert.That(t, IsInjected(err)).True()
	assert.That(t, ran).False()

	// Scope "loadtest" + plain ctx => fn runs (scope excludes real traffic).
	in2 := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic", Scope: "loadtest"})
	ran = false
	assert.Error(t, Apply(context.Background(), in2, "svc", func() error { ran = true; return nil })).Nil()
	assert.That(t, ran).True()

	// Scope "loadtest" + load-test ctx => injected.
	ran = false
	err = Apply(traffic.WithLoadTest(context.Background(), "test"), in2, "svc", func() error { ran = true; return nil })
	assert.That(t, IsInjected(err)).True()
	assert.That(t, ran).False()
}

// TestInjector_PerResourceRules verifies a matching Rule overrides the global
// rate/error for that resource, while non-matching resources fall back to the
// global. First matching rule wins; empty-resources is a catch-all.
func TestInjector_PerResourceRules(t *testing.T) {
	// Global rate 0 (no faults) but a rule faults svc-a at rate 1.
	in := NewInjector(Config{
		Enabled: true,
		Rate:    0,
		Rules: []Rule{
			{Resources: []string{"svc-a"}, Rate: 1, Error: "generic"},
		},
	})
	exec := WrapExecutorWith(newExec(t, resilience.Policy{}), in)

	// svc-a matches the rule => faulted.
	assert.That(t, IsInjected(exec.Execute(context.Background(), "svc-a", countFn(new(int32), nil)))).True()
	// svc-b matches no rule, global rate 0 => passes through.
	var calls int32
	assert.Error(t, exec.Execute(context.Background(), "svc-b", countFn(&calls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// Catch-all rule overrides global for any resource. Specific rule still wins
	// over the catch-all when listed first.
	in2 := NewInjector(Config{Enabled: true, Rate: 0, Rules: []Rule{
		{Resources: []string{"svc-a"}, Rate: 1, Error: "timeout"},
		{Resources: nil, Rate: 1, Error: "generic"}, // catch-all
	}})
	exec2 := WrapExecutorWith(newExec(t, resilience.Policy{}), in2)
	err := exec2.Execute(context.Background(), "svc-a", countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True() // svc-a => timeout kind
	// svc-c falls through to the catch-all => generic.
	err = exec2.Execute(context.Background(), "svc-c", countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).False() // generic, not timeout
}
