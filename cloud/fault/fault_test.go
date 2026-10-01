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

	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// newExec builds a default-driver executor for service under p. The service
// matters: an executor is bound to one service (see [resilience.ClientExecutor]), and
// the injector matches its rules by that service.
func newExec(t *testing.T, service string, p resilience.ClientPolicy) resilience.ClientExecutor {
	t.Helper()
	d := resilience.NewDefaultDriver(nil)
	exec, err := d.NewClientExecutor(service, p)
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

// TestInjector_DisabledIsTransparent: Enabled=false => fn runs, no injection.
func TestInjector_DisabledIsTransparent(t *testing.T) {
	in := NewInjector(Configs{Client: Config{}}, nil) // Enabled false
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)
	var calls int32
	err := exec.Execute(context.Background(), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_RateZeroAlwaysSucceeds: Rate 0 with Enabled => never inject.
func TestInjector_RateZeroAlwaysSucceeds(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 0, Error: "generic"}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)
	var calls int32
	for range 10 {
		err := exec.Execute(context.Background(), countFn(&calls, nil))
		assert.Error(t, err).Nil()
	}
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(10))
}

// TestInjector_RateOneRetriesInjectedError: every attempt is injected, so the
// real fn never runs and the executor exhausts its retries (MaxRetries+1
// attempts) and returns the injected error.
func TestInjector_RateOneRetriesInjectedError(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic"}}, nil)
	// MaxRetries 2 => 3 attempts; no breaker so retry is the only path.
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{MaxRetries: 2}), "svc", in)
	var calls int32
	err := exec.Execute(context.Background(), countFn(&calls, nil))
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
	assert.That(t, resilience.ClientPolicy{}.ShouldRetry(ErrInjected)).True()
}

// TestInjector_KindsSurfaceAsFamiliarErrors: the typed kinds wrap a real error
// so errors.Is matches the underlying sentinel (and the observe outcome mapping
// classifies the call the same way a real timeout/reset would).
func TestInjector_KindsSurfaceAsFamiliarErrors(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "timeout"}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)
	err := exec.Execute(context.Background(), countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True()
	assert.That(t, IsInjected(err)).True()

	in.SetConfig(Configs{Client: Config{Enabled: true, Rate: 1, Error: "reset"}})
	err = exec.Execute(context.Background(), countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
}

// TestInjector_BreakerOpensUnderFault: with a tight breaker, injected faults
// trip it open and subsequent calls are rejected as ErrCircuitOpen without
// touching fn — the closed loop (fault → breaker → neutral sentinel) holds.
func TestInjector_BreakerOpensUnderFault(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic"}}, nil)
	// ErrorThreshold 1 opens after the first failed attempt; OpenDuration long
	// enough that the second call sees it still open.
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{ErrorThreshold: 1, OpenDuration: time.Second}), "svc", in)
	var calls int32
	_ = exec.Execute(context.Background(), countFn(&calls, nil)) // trips the breaker
	firstCalls := atomic.LoadInt32(&calls)

	err := exec.Execute(context.Background(), countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, resilience.ErrCircuitOpen)).True()
	// fn not called on the rejected attempt (only the first call's attempts ran).
	assert.That(t, atomic.LoadInt32(&calls)).Equal(firstCalls)
}

// TestInjector_LatencySleepsAndCancels: injected latency sleeps, and a cancelled
// attempt context surfaces the context error instead of retrying forever.
func TestInjector_LatencySleepsAndCancels(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 0, Latency: 200 * time.Millisecond}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{AttemptTimeout: 50 * time.Millisecond}), "svc", in)
	var calls int32
	start := time.Now()
	err := exec.Execute(context.Background(), countFn(&calls, nil))
	elapsed := time.Since(start)
	// The 200ms latency exceeds the 50ms per-attempt timeout: the sleep is
	// cancelled by the attempt deadline and the call fails without fn running.
	assert.Error(t, err).NotNil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0))
	assert.That(t, elapsed < 200*time.Millisecond).True()
}

// TestInjector_LatencyJitterBounds: with LatencyJitter J, every draw lands in
// [Latency-J, Latency+J] clamped to ≥ 0; the midpoint over many draws stays
// near Latency.
func TestInjector_LatencyJitterBounds(t *testing.T) {
	const base, jitter = 100 * time.Millisecond, 80 * time.Millisecond
	in := NewInjector(Configs{Client: Config{Enabled: true, Latency: base, LatencyJitter: jitter}}, nil)
	c := in.ClientConfig()
	var sum time.Duration
	for i := 0; i < 200; i++ {
		_, sleep, _ := in.client.maybe(c, "svc")
		assert.That(t, sleep >= base-jitter).True()
		assert.That(t, sleep <= base+jitter).True()
		sum += sleep
	}
	mid := sum / 200
	assert.That(t, mid > base-40*time.Millisecond && mid < base+40*time.Millisecond).True()
}

// TestInjector_HotSwap: toggling Enabled at runtime via SetConfig takes effect
// on the next operation (the Dync-driven path starters use).
func TestInjector_HotSwap(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic"}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)
	var calls int32
	err := exec.Execute(context.Background(), countFn(&calls, nil))
	assert.That(t, IsInjected(err)).True()

	in.SetConfig(Configs{Client: Config{Enabled: false}}) // turn the fire off
	err = exec.Execute(context.Background(), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_ScopeGatesLoadTestTraffic verifies the Scope config restricts
// injection by the load-test marker on the call's context.
func TestInjector_ScopeGatesLoadTestTraffic(t *testing.T) {
	// Rate 1 + generic error => every in-scope call is faulted.
	mk := func(scope string) resilience.ClientExecutor {
		in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic", Scope: scope}}, nil)
		return WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)
	}
	realCtx := context.Background()
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	loadCtx := prop.WithLoadTest(context.Background())

	// Scope "" (default): both real and load-test traffic get faulted.
	all := mk("")
	assert.That(t, IsInjected(all.Execute(realCtx, countFn(new(int32), nil)))).True()
	assert.That(t, IsInjected(all.Execute(loadCtx, countFn(new(int32), nil)))).True()

	// Scope "real": real traffic faulted, load-test traffic passes through.
	real := mk("real")
	assert.That(t, IsInjected(real.Execute(realCtx, countFn(new(int32), nil)))).True()
	var ltCalls int32
	assert.Error(t, real.Execute(loadCtx, countFn(&ltCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&ltCalls)).Equal(int32(1)) // fn ran, no fault

	// Scope "loadtest": load-test traffic faulted, real traffic passes through.
	lt := mk("loadtest")
	assert.That(t, IsInjected(lt.Execute(loadCtx, countFn(new(int32), nil)))).True()
	var realCalls int32
	assert.Error(t, lt.Execute(realCtx, countFn(&realCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&realCalls)).Equal(int32(1))
}

// TestInjector_MaxAffectedCapsBlastRadius verifies MaxAffected stops injecting
// after the configured count of affected calls.
func TestInjector_MaxAffectedCapsBlastRadius(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic", MaxAffected: 3}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)

	// First three calls are faulted.
	for range 3 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), countFn(new(int32), nil)))).True()
	}
	// Fourth onward: guardrail tripped, call passes through untouched.
	var calls int32
	err := exec.Execute(context.Background(), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_MaxDurationAutoOff verifies MaxDuration turns the fire off after
// the window elapses (a forgotten fault self-heals).
func TestInjector_MaxDurationAutoOff(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic", MaxDuration: 40 * time.Millisecond}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)

	// Within the window: faulted.
	assert.That(t, IsInjected(exec.Execute(context.Background(), countFn(new(int32), nil)))).True()
	// After the window: passes through.
	time.Sleep(60 * time.Millisecond)
	var calls int32
	err := exec.Execute(context.Background(), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestInjector_NoGuardrailsUnchanged confirms the default (no guardrails) keeps
// the original behavior: every Rate-1 call faults, indefinitely.
func TestInjector_NoGuardrailsUnchanged(t *testing.T) {
	in := NewInjector(Configs{Client: Config{Enabled: true, Rate: 1, Error: "generic"}}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)
	for range 10 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), countFn(new(int32), nil)))).True()
	}
}

// TestApply_ServerSideFault verifies the server-side ApplyServer seam: it injects
// latency+error per the injector's SERVER config, honours Scope vs the load-test
// marker, and is a transparent pass-through for nil injector.
func TestApply_ServerSideFault(t *testing.T) {
	// nil injector => fn runs untouched.
	ran := false
	assert.Error(t, ApplyServer(context.Background(), nil, "svc", func() error { ran = true; return nil })).Nil()
	assert.That(t, ran).True()

	// Rate 1 on the server side => injected error returned, fn NOT called. The
	// client side is armed too, with a different config, to pin that ApplyServer reads
	// the server half and not the client one.
	in := NewInjector(Configs{
		Client: Config{Enabled: true, Rate: 0, Latency: 50 * time.Millisecond},
		Server: Config{Enabled: true, Rate: 1, Error: "generic"},
	}, nil)
	ran = false
	err := ApplyServer(context.Background(), in, "svc", func() error { ran = true; return nil })
	assert.That(t, IsInjected(err)).True()
	assert.That(t, ran).False()
	// The client config on the same injector does not reach inbound: an outbound
	// call gets the client side's latency and no error.
	start := time.Now()
	assert.Error(t, WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in).
		Execute(context.Background(), countFn(new(int32), nil))).Nil()
	assert.That(t, time.Since(start) >= 50*time.Millisecond).True()

	// Scope "loadtest" + plain ctx => fn runs (scope excludes real traffic).
	in2 := NewInjector(Configs{Server: Config{Enabled: true, Rate: 1, Error: "generic", Scope: "loadtest"}}, nil)
	ran = false
	assert.Error(t, ApplyServer(context.Background(), in2, "svc", func() error { ran = true; return nil })).Nil()
	assert.That(t, ran).True()

	// Scope "loadtest" + load-test ctx => injected.
	ran = false
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	err = ApplyServer(prop.WithLoadTest(context.Background()), in2, "svc", func() error { ran = true; return nil })
	assert.That(t, IsInjected(err)).True()
	assert.That(t, ran).False()
}

// TestInjector_DirectionsAreIndependent pins the core of the direction split: a
// fire on one side leaves the other untouched, including the guardrails — each
// direction counts its own MaxAffected, so a client fire that has spent its blast
// radius cannot silence a server fire.
func TestInjector_DirectionsAreIndependent(t *testing.T) {
	in := NewInjector(Configs{
		Client: Config{Enabled: true, Rate: 1, Error: "generic"},
		Server: Config{Enabled: true, Rate: 1, Error: "reset"},
	}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)

	// Outbound gets the client's kind; inbound gets the server's.
	err := exec.Execute(context.Background(), countFn(new(int32), nil))
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).False()
	assert.That(t, IsInjected(err)).True()

	srvRan := false
	srvErr := ApplyServer(context.Background(), in, "svc", func() error { srvRan = true; return nil })
	assert.That(t, errors.Is(srvErr, syscall.ECONNRESET)).True()
	assert.That(t, srvRan).False()

	// One side off (a push that disables the client) must not touch the other.
	in.SetConfig(Configs{
		Client: Config{Enabled: false},
		Server: Config{Enabled: true, Rate: 1, Error: "reset"},
	})
	assert.Error(t, exec.Execute(context.Background(), countFn(new(int32), nil))).Nil()
	assert.That(t, IsInjected(ApplyServer(context.Background(), in, "svc", func() error { return nil }))).True()
}

// TestInjector_GuardrailsPerDirection pins that the safety guardrails are counted
// PER DIRECTION: an outbound fire that has spent its MaxAffected budget cannot
// silence the inbound one, and each self-heals on its own.
func TestInjector_GuardrailsPerDirection(t *testing.T) {
	in := NewInjector(Configs{
		Client: Config{Enabled: true, Rate: 1, Error: "generic", MaxAffected: 2},
		Server: Config{Enabled: true, Rate: 1, Error: "reset", MaxAffected: 1},
	}, nil)
	exec := WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}), "svc", in)

	// Spend the client side's budget (2 of 3 calls are affected).
	for range 2 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), countFn(new(int32), nil)))).True()
	}
	assert.Error(t, exec.Execute(context.Background(), countFn(new(int32), nil))).Nil()

	// The server side is still armed: its own counter never saw those calls.
	assert.That(t, IsInjected(ApplyServer(context.Background(), in, "svc", func() error { return nil }))).True()
	// ...and it self-heals after its own single affected call.
	assert.Error(t, ApplyServer(context.Background(), in, "svc", func() error { return nil })).Nil()
}

// TestInjector_PerServiceRules verifies a matching Rule overrides the global
// rate/error for that service, while non-matching services fall back to the
// global. First matching rule wins; an empty service is a catch-all.
func TestInjector_PerServiceRules(t *testing.T) {
	// Global rate 0 (no faults) but a rule faults svc-a at rate 1.
	in := NewInjector(Configs{Client: Config{
		Enabled: true,
		Rate:    0,
		Rules: []Rule{
			{Service: "svc-a", Rate: 1, Error: "generic"},
		},
	}}, nil)
	// One executor per service: an executor is bound to one service now, so each
	// service under test needs its own. The injector is shared, because fault
	// rules are matched by service inside the gate.
	execA := WrapClientExecutor(newExec(t, "svc-a", resilience.ClientPolicy{}), "svc-a", in)
	execB := WrapClientExecutor(newExec(t, "svc-b", resilience.ClientPolicy{}), "svc-b", in)

	// svc-a matches the rule => faulted.
	assert.That(t, IsInjected(execA.Execute(context.Background(), countFn(new(int32), nil)))).True()
	// svc-b matches no rule, global rate 0 => passes through.
	var calls int32
	assert.Error(t, execB.Execute(context.Background(), countFn(&calls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// Catch-all rule overrides global for any service. Specific rule still wins
	// over the catch-all when listed first.
	in2 := NewInjector(Configs{Client: Config{Enabled: true, Rate: 0, Rules: []Rule{
		{Service: "svc-a", Rate: 1, Error: "timeout"},
		{Service: "", Rate: 1, Error: "generic"}, // catch-all
	}}}, nil)
	exec2A := WrapClientExecutor(newExec(t, "svc-a", resilience.ClientPolicy{}), "svc-a", in2)
	exec2C := WrapClientExecutor(newExec(t, "svc-c", resilience.ClientPolicy{}), "svc-c", in2)
	err := exec2A.Execute(context.Background(), countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True() // svc-a => timeout kind
	// svc-c falls through to the catch-all => generic.
	err = exec2C.Execute(context.Background(), countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).False() // generic, not timeout
}
