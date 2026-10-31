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

	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/matcher"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// configs is the test-side convenience for setting both directions at once. The
// production API arms each direction with its own manager's Refresh; the zero
// kfault.Config is inert, so a test names only the side it means to arm.
type configs struct {
	Client kfault.Config
	Server kfault.Config
}

// fixture holds the two independent authorities side by side, so a test that
// asserts cross-direction isolation can reach both. Production code never puts
// them in one object.
type fixture struct {
	client *Manager
	server *serverfault.Manager
}

func newFixture(c configs) *fixture {
	in := &fixture{client: NewManager(), server: serverfault.NewManager()}
	in.refresh(c)
	return in
}

func (in *fixture) refresh(c configs) {
	in.client.Refresh(c.Client)
	in.server.Refresh(c.Server)
}

// newExec builds a default-driver executor for service under p. The service
// matters: an executor is bound to one service (see [chain.Executor]), and
// the injector matches its rules by that service.
func newExec(t *testing.T, service string, cfg any) chain.Executor {
	t.Helper()
	m := resilience.NewManager(resilience.NewDefaultDriver(nil))
	arm(t, m, cfg)
	t.Cleanup(func() { _ = m.Close() })
	return m.ExecutorFor(service)
}

// arm applies cfg — one capability's policy — to m.
func arm(t *testing.T, m *resilience.Manager, cfg any) {
	t.Helper()
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
}

// countFn returns an operation fn that increments calls and returns a fixed
// error (or nil) so tests can assert how many times the real fn ran.
func countFn(counter *int32, err error) func(context.Context) error {
	return func(context.Context) error {
		atomic.AddInt32(counter, 1)
		return err
	}
}

// TestManager_ZeroConfigIsTransparent: an inert config (no rate, no latency)
// => fn runs, no injection.
func TestManager_ZeroConfigIsTransparent(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{}}) // inert
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_RuleAttribution: the decision names the rule the fire came from —
// the rule's configured Name — so a platform can join its
// own experiment record with the call. With rules-only there is no "top-level
// knobs" fallback: no match means no fire.
func TestManager_RuleAttribution(t *testing.T) {
	// A matched rule: named by its identity.
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{
		{Name: "guarded-svc", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}}})
	f := decidedAt(t, in, kfault.Target{Service: "svc"})
	assert.That(t, f.Fired()).True()
	assert.That(t, f.Name).Equal("guarded-svc")

	// No rule matches: nothing fires, and the decision names nothing.
	in2 := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{
		{Name: "other-rule", Service: "other", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}}})
	f2 := decidedAt(t, in2, kfault.Target{Service: "svc"})
	assert.That(t, f2.Fired()).False()
	assert.That(t, f2.Fired()).False()
}

// decidedAt runs one gated call and returns what the injector decided about it.
// No round is needed: the decision is the return value now, which is the point
// of it being one.
func decidedAt(t *testing.T, in *fixture, tgt kfault.Target) kfault.Result {
	t.Helper()
	d, _ := in.client.NewClient().Fire(context.Background(), tgt)
	return d
}

// TestManager_RateZeroAlwaysSucceeds: Rate 0 => never inject.
func TestManager_RateZeroAlwaysSucceeds(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 0, Class: kfault.ClassTransient, Error: "generic"}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	var calls int32
	for range 10 {
		err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
		assert.Error(t, err).Nil()
	}
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(10))
}

// TestManager_RateOneRetriesInjectedError: every attempt is injected, so the
// real fn never runs and the executor exhausts its retries (MaxRetries+1
// attempts) and returns the injected error.
func TestManager_RateOneRetriesInjectedError(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}}})
	// MaxRetries 2 => 3 attempts; no breaker so retry is the only path.
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.Limits{Retry: chain.RetryPolicy{MaxRetries: 2}}))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0)) // fn never reached
}

// TestInjectedError_Retryable confirms the Retryable hook is set so resilience
// treats injected faults as retryable.
func TestInjectedError_Retryable(t *testing.T) {
	var r resilience.Retryable
	assert.That(t, errors.As(kfault.ErrInjected, &r)).True()
	assert.That(t, r.Retryable()).True()
	assert.That(t, resilience.ShouldRetry(kfault.ErrInjected)).True()
}

// TestManager_KindsSurfaceAsFamiliarErrors: the typed kinds wrap a real error
// so errors.Is matches the underlying sentinel (and the observe outcome mapping
// classifies the call the same way a real timeout/reset would).
func TestManager_KindsSurfaceAsFamiliarErrors(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True()
	assert.That(t, kfault.IsInjected(err)).True()

	in.refresh(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "reset"}}}})
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
}

// TestManager_BreakerOpensUnderFault: with a tight breaker, injected faults
// trip it open and subsequent calls are rejected as ErrCircuitOpen without
// touching fn — the closed loop (fault → breaker → neutral sentinel) holds.
func TestManager_BreakerOpensUnderFault(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}}})
	// ErrorThreshold 1 opens after the first failed attempt; OpenDuration long
	// enough that the second call sees it still open.
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Second}))
	var calls int32
	_ = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil)) // trips the breaker
	firstCalls := atomic.LoadInt32(&calls)

	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, chain.ErrCircuitOpen)).True()
	// fn not called on the rejected attempt (only the first call's attempts ran).
	assert.That(t, atomic.LoadInt32(&calls)).Equal(firstCalls)
}

// TestManager_LatencySleepsAndCancels: injected latency sleeps, and a cancelled
// attempt context surfaces the context error instead of retrying forever.
func TestManager_LatencySleepsAndCancels(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 0, Class: kfault.ClassTransient, Error: kfault.ErrorGeneric, Latency: 200 * time.Millisecond}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.Limits{Timeout: 50 * time.Millisecond}))
	var calls int32
	start := time.Now()
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	elapsed := time.Since(start)
	// The 200ms latency exceeds the 50ms per-attempt timeout: the sleep is
	// cancelled by the attempt deadline and the call fails without fn running.
	assert.Error(t, err).NotNil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0))
	assert.That(t, elapsed < 200*time.Millisecond).True()
}

// TestManager_HotSwap: pushing a new config at runtime via Refresh takes
// effect on the next operation (the Dync-driven path starters use) — here,
// arming a fire and then disarming it by pushing an inert config.
func TestManager_HotSwap(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()

	in.refresh(configs{Client: kfault.Config{}}) // push an inert config: fire off
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestRefreshReachesExistingExecutor pins the publish model: an executor created
// while the manager is inert must start firing after a later push, without ever
// asking the manager on the call path. The authority publishes one atomic kernel
// pointer; every call loads whatever generation is current.
func TestRefreshReachesExistingExecutor(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{}}) // unarmed
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil() // inert: the real fn ran
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// Arm AFTER the executor exists.
	in.client.Refresh(kfault.Config{Rules: []kfault.Rule{{Name: "arm", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}})
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1)) // fn never ran
}

// TestRefreshReachesExistingServer is the inbound counterpart of
// TestRefreshReachesExistingExecutor.
func TestRefreshReachesExistingServer(t *testing.T) {
	in := newFixture(configs{})
	srv := in.server.NewServer()
	_, err := srv.Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.Error(t, err).Nil()

	in.server.Refresh(kfault.Config{Rules: []kfault.Rule{{Name: "arm", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"}}})
	_, err = srv.Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(err)).True()
}

// TestManager_TrafficClassIsPlainMetadata proves a rule targets a traffic class
// with an ordinary metadata condition: the operation's metadata carries
// "traffic" (the seam that built it recorded the class), and the generic subset
// match does the rest.
func TestManager_TrafficClassIsPlainMetadata(t *testing.T) {
	// Rate 1 + generic error => every matching call is faulted.
	mk := func(class string) chain.Executor {
		r := kfault.Rule{Name: "traffic", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}
		if class != "" {
			r.MetadataMatch = matcher.ExactMap{"traffic": class}
		}
		in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{r}}})
		return in.client.WrapClientExecutor(newExec(t, "svc", nil))
	}
	op := func(class string) chain.Operation {
		return chain.Operation{Service: "svc", Request: chain.Request{
			Metadata: map[string]string{"traffic": class},
		}}
	}

	// No traffic condition: both real and load-test traffic get faulted.
	all := mk("")
	assert.That(t, kfault.IsInjected(all.Execute(context.Background(), op("real"), nil, countFn(new(int32), nil)))).True()
	assert.That(t, kfault.IsInjected(all.Execute(context.Background(), op("loadtest"), nil, countFn(new(int32), nil)))).True()

	// match.traffic=real: real traffic faulted, load-test passes through.
	real := mk("real")
	assert.That(t, kfault.IsInjected(real.Execute(context.Background(), op("real"), nil, countFn(new(int32), nil)))).True()
	var ltCalls int32
	assert.Error(t, real.Execute(context.Background(), op("loadtest"), nil, countFn(&ltCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&ltCalls)).Equal(int32(1)) // fn ran, no fault

	// match.traffic=loadtest: load-test faulted, real passes through.
	lt := mk("loadtest")
	assert.That(t, kfault.IsInjected(lt.Execute(context.Background(), op("loadtest"), nil, countFn(new(int32), nil)))).True()
	var realCalls int32
	assert.Error(t, lt.Execute(context.Background(), op("real"), nil, countFn(&realCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&realCalls)).Equal(int32(1))
}

// TestManager_MaxAffectedCapsBlastRadius verifies MaxAffected stops injecting
// after the configured count of affected calls.
func TestManager_MaxAffectedCapsBlastRadius(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxAffected: 3}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))

	// First three calls are faulted.
	for range 3 {
		assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil)))).True()
	}
	// Fourth onward: guardrail tripped, call passes through untouched.
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_MaxDurationAutoOff verifies MaxDuration turns the fire off after
// the window elapses (a forgotten fault self-heals).
func TestManager_MaxDurationAutoOff(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxDuration: 40 * time.Millisecond}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))

	// Within the window: faulted.
	assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil)))).True()
	// After the window: passes through.
	time.Sleep(60 * time.Millisecond)
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_NoGuardrailsUnchanged confirms the default (no guardrails) keeps
// the original behavior: every Rate-1 call faults, indefinitely.
func TestManager_NoGuardrailsUnchanged(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	for range 10 {
		assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil)))).True()
	}
}

// TestApply_ServerSideFault verifies the server-side Fire seam: it injects
// latency+error per the injector's SERVER config and honours Scope vs the
// load-test marker.
func TestFireServer_ServerSideFault(t *testing.T) {
	// Rate 1 on the server side => the injected error is the answer, so the
	// caller never runs its handler. The client side is armed too, with a
	// different config, to pin that Fire reads the server half and not the
	// client one.
	in := newFixture(configs{
		Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 0, Class: kfault.ClassTransient, Error: kfault.ErrorGeneric, Latency: 50 * time.Millisecond}}},
		Server: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}},
	})
	d, err := in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, d.Error).Equal("generic")
	// The client config on the same injector does not reach inbound: an outbound
	// call gets the client side's latency and no error.
	start := time.Now()
	assert.Error(t, in.client.WrapClientExecutor(newExec(t, "svc", nil)).
		Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))).Nil()
	assert.That(t, time.Since(start) >= 50*time.Millisecond).True()

	// match.traffic=loadtest + real call => nothing fires, so the caller proceeds.
	in2 := newFixture(configs{Server: kfault.Config{Rules: []kfault.Rule{{
		Name: "loadtest-only", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic",
		MetadataMatch: matcher.ExactMap{"traffic": "loadtest"},
	}}}})
	srv := in2.server.NewServer()
	_, err = srv.Fire(context.Background(), kfault.Target{Service: "svc", Metadata: map[string]string{"traffic": "real"}})
	assert.Error(t, err).Nil()

	// match.traffic=loadtest + load-test call => injected.
	_, err = srv.Fire(context.Background(), kfault.Target{Service: "svc", Metadata: map[string]string{"traffic": "loadtest"}})
	assert.That(t, kfault.IsInjected(err)).True()
}

// TestManager_DirectionsAreIndependent pins the core of the direction split: a
// fire on one side leaves the other untouched, including the guardrails — each
// direction counts its own MaxAffected, so a client fire that has spent its blast
// radius cannot silence a server fire.
func TestManager_DirectionsAreIndependent(t *testing.T) {
	in := newFixture(configs{
		Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}},
		Server: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "reset"}}},
	})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))

	// Outbound gets the client's kind; inbound gets the server's.
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).False()
	assert.That(t, kfault.IsInjected(err)).True()

	_, srvErr := in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, errors.Is(srvErr, syscall.ECONNRESET)).True()

	// One side off (a push that disables the client) must not touch the other.
	in.refresh(configs{
		Client: kfault.Config{},
		Server: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "reset"}}},
	})
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))).Nil()
	_, srvErr = in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(srvErr)).True()
}

// TestManager_GuardrailsPerDirection pins that the safety guardrails are counted
// PER DIRECTION: an outbound fire that has spent its MaxAffected budget cannot
// silence the inbound one, and each self-heals on its own.
func TestManager_GuardrailsPerDirection(t *testing.T) {
	in := newFixture(configs{
		Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxAffected: 2}}},
		Server: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "reset", MaxAffected: 1}}},
	})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))

	// Spend the client side's budget (2 of 3 calls are affected).
	for range 2 {
		assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil)))).True()
	}
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))).Nil()

	// The server side is still armed: its own counter never saw those calls.
	_, srvErr := in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(srvErr)).True()
	// ...and it self-heals after its own single affected call.
	_, srvErr = in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.Error(t, srvErr).Nil()
}

// TestManager_PerServiceRules verifies a rule only governs the service it
// names: a non-matching service is not fired at all; every rule names its
// target, so a call only ever meets rules written for its service.
func TestManager_PerServiceRules(t *testing.T) {
	// Only svc-a is armed.
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{
		{Name: "svc-a-rule", Service: "svc-a", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}}})
	// One executor per service: an executor is bound to one service now, so each
	// service under test needs its own. The injector is shared, because fault
	// rules are matched by service inside the gate.
	execA := in.client.WrapClientExecutor(newExec(t, "svc-a", nil))
	execB := in.client.WrapClientExecutor(newExec(t, "svc-b", nil))

	// svc-a matches the rule => faulted.
	assert.That(t, kfault.IsInjected(execA.Execute(context.Background(), chain.Operation{Service: "svc-a"}, nil, countFn(new(int32), nil)))).True()
	// svc-b matches no rule, global rate 0 => passes through.
	var calls int32
	assert.Error(t, execB.Execute(context.Background(), chain.Operation{Service: "svc-b"}, nil, countFn(&calls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// A second rule names a different service; each service meets its own rule.
	in2 := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{
		{Name: "svc-a-timeout", Service: "svc-a", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"},
		{Name: "svc-c-generic", Service: "svc-c", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}}})
	exec2A := in2.client.WrapClientExecutor(newExec(t, "svc-a", nil))
	exec2C := in2.client.WrapClientExecutor(newExec(t, "svc-c", nil))
	err := exec2A.Execute(context.Background(), chain.Operation{Service: "svc-a"}, nil, countFn(new(int32), nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True() // svc-a => timeout kind
	// svc-c meets its own rule => generic.
	err = exec2C.Execute(context.Background(), chain.Operation{Service: "svc-c"}, nil, countFn(new(int32), nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).False() // generic, not timeout
}

// TestInjectedFaultIsTaggedOnFacts: an armed client fault writes the injection
// onto the call's record, so a layer above (observability, an audit, a chaos
// controller) sees a call was set on fire rather than failing on its own — the
// tag that closes the "injected fault is indistinguishable from a real failure"
// gap.
func TestInjectedFaultIsTaggedOnFacts(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	op := chain.Operation{Service: "svc"}
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), op, rec, countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.Number(t, int(calls)).Equal(0)
	// The client adapter wrote the decision it got back onto the call's record.
	assert.That(t, rec.Fault).NotNil()
	assert.String(t, rec.Fault.Error).Equal("timeout")
	assert.That(t, rec.Fault.Fired()).True() // the rule's configured identity
	assert.String(t, rec.Fault.Name).Equal("fire")
}

// TestFireOnAnEarlierAttemptSurvives pins that the fire is written PER ATTEMPT,
// not once after Execute returns: the record is call-level, so a later attempt
// that does NOT fire must not erase an earlier one that did.
//
// MaxAffected 1 makes that case deterministic: the first attempt fires (spending
// the guardrail's budget), the retry sees the budget gone and runs clean, so the
// call SUCCEEDS with a fire on its record. A write made after Execute from the
// final decision alone would find "not fired" and record nothing.
func TestFireOnAnEarlierAttemptSurvives(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxAffected: 1}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.Limits{
		Retry: chain.RetryPolicy{MaxRetries: 1},
	}))
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))

	assert.Error(t, err).Nil()                   // the retry succeeded
	assert.Number(t, int(calls)).Equal(1)        // fn ran on the clean retry
	assert.Number(t, len(rec.Rounds())).Equal(2) // two attempts recorded
	assert.That(t, rec.Fault).NotNil()           // the first attempt's fire survived
	assert.String(t, rec.Fault.Error).Equal("generic")
}

// TestFaultRuleMatchesMetadata: a fault rule whose Match names a metadata
// dimension fires only for calls that carry it — a rule can target one tenant or
// region of a service, not just the whole service.
func TestFaultRuleMatchesMetadata(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{
		Name:          "acme-only",
		Service:       "svc",
		MetadataMatch: map[string]string{"tenant": "acme"},
		Rate:          1,
	}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))

	// A call tagged tenant=acme is faulted.
	var a int32
	opA := chain.Operation{Service: "svc", Request: chain.Request{Metadata: map[string]string{"tenant": "acme"}}}
	assert.Error(t, exec.Execute(context.Background(), opA, nil, countFn(&a, nil))).NotNil()
	assert.Number(t, int(a)).Equal(0)

	// Another tenant falls through to the (inert) default: untouched.
	var b int32
	opB := chain.Operation{Service: "svc", Request: chain.Request{Metadata: map[string]string{"tenant": "other"}}}
	assert.Error(t, exec.Execute(context.Background(), opB, nil, countFn(&b, nil))).Nil()
	assert.Number(t, int(b)).Equal(1)
}

// TestFaultRuleMatchesAction: a fault rule whose Action names an interface fires
// only for calls of that interface — one endpoint can be set on fire without
// touching another.
func TestFaultRuleMatchesAction(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{
		Name:        "insert-only",
		Service:     "svc",
		ActionMatch: kfault.ActionMatch{Exact: "insert"},
		Rate:        1,
	}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))

	// The named interface is faulted.
	var a int32
	opA := chain.Operation{Service: "svc", Request: chain.Request{Action: "insert"}}
	assert.Error(t, exec.Execute(context.Background(), opA, nil, countFn(&a, nil))).NotNil()
	assert.Number(t, int(a)).Equal(0)

	// A different interface falls through to the (inert) default: untouched.
	var b int32
	opB := chain.Operation{Service: "svc", Request: chain.Request{Action: "select"}}
	assert.Error(t, exec.Execute(context.Background(), opB, nil, countFn(&b, nil))).Nil()
	assert.Number(t, int(b)).Equal(1)
}

// TestFireServer_RuleMatchesActionAndMetadata pins the capability the server
// seam did not have while it handed the injector a zero chain.Operation: an
// inbound rule can name ONE interface of the server and one slice of its
// traffic, instead of only being able to fire the whole entrance.
func TestFireServer_RuleMatchesActionAndMetadata(t *testing.T) {
	in := newFixture(configs{Server: kfault.Config{Rules: []kfault.Rule{{
		Name:          "users-acme",
		Service:       "gin",
		ActionMatch:   kfault.ActionMatch{Prefix: "GET /users"},
		MetadataMatch: map[string]string{"tenant": "acme"},
		Rate:          1,
	}}}})

	fires := func(tgt kfault.Target) bool {
		_, err := in.server.NewServer().Fire(context.Background(), tgt)
		return kfault.IsInjected(err)
	}

	// The named route of the named tenant fires.
	assert.That(t, fires(kfault.Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme"}})).True()
	// Another route of the same server: the interface filter holds it back.
	assert.That(t, fires(kfault.Target{Service: "gin", Action: "POST /orders",
		Metadata: map[string]string{"tenant": "acme"}})).False()
	// The same route of another tenant: the metadata filter holds it back.
	assert.That(t, fires(kfault.Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "other"}})).False()
}

// TestInjectedClass_Classification pins the two independent questions a class
// answers: may I retry it, and does it count against the breaker.
func TestInjectedClass_Classification(t *testing.T) {
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
		assert.That(t, resilience.ShouldRetry(err)).Equal(tc.retry)
		assert.That(t, resilience.CountsAsFailure(err)).Equal(tc.count)
	}
	assert.That(t, resilience.CountsAsFailure(nil)).False()
}

// TestManager_PermanentClassIsNotRetried: class=permanent must suppress retry,
// which the hard-wired retryable injection could not do — the path "this failure
// is deterministic, retrying is pointless" is now testable.
func TestManager_PermanentClassIsNotRetried(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{
		Name: "permanent", Service: "svc", Rate: 1, Error: "generic", Class: kfault.ClassPermanent,
	}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.Limits{
		Retry: chain.RetryPolicy{MaxRetries: 3},
	}))
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, int(calls)).Equal(0)        // fn never reached
	assert.Number(t, len(rec.Rounds())).Equal(1) // exactly one attempt
}

// TestManager_BusinessClassDoesNotTripBreaker: class=business is a caller
// mistake, so even a threshold-1 breaker must ignore it and stay closed — the
// "a 4xx must not open the circuit" path.
func TestManager_BusinessClassDoesNotTripBreaker(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{
		Name: "business", Service: "svc", Rate: 1, Error: "generic", Class: kfault.ClassBusiness,
	}}}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.BreakerPolicy{
		ErrorThreshold: 1, OpenDuration: time.Second,
	}))
	for range 3 {
		err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))
		assert.That(t, kfault.IsInjected(err)).True()
		assert.That(t, errors.Is(err, chain.ErrCircuitOpen)).False()
	}
}

// TestInjectedRunIDOnRecord: the document's run id rides through the decision to
// the call record, so this fire can be grouped with the other replicas' fires
// from the same drill.
func TestInjectedRunIDOnRecord(t *testing.T) {
	in := newFixture(configs{Client: kfault.Config{
		RunID: "drill-9",
		Rules: []kfault.Rule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}},
	}})
	exec := in.client.WrapClientExecutor(newExec(t, "svc", nil))
	rec := &chain.Recorder{}
	_ = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(new(int32), nil))
	assert.That(t, rec.Fault).NotNil()
	assert.String(t, rec.Fault.RunID).Equal("drill-9")
}
