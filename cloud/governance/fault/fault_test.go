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
	"go-spring.org/cloud/chain"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// configs is the test-side convenience for setting both directions at once. The
// production API arms each direction with its own manager's Refresh; the zero
// Config is inert, so a test names only the side it means to arm.
type configs struct {
	Client Config
	Server Config
}

// fixture holds the two independent authorities side by side, so a test that
// asserts cross-direction isolation can reach both. Production code never puts
// them in one object.
type fixture struct {
	client *ClientManager
	server *ServerManager
}

func newFixture(c configs, prop traffic.Propagator) *fixture {
	in := &fixture{client: NewClientManager(prop), server: NewServerManager(prop)}
	in.client.Refresh(c.Client)
	in.server.Refresh(c.Server)
	return in
}

func (in *fixture) refresh(c configs) {
	in.client.Refresh(c.Client)
	in.server.Refresh(c.Server)
}

// newExec builds a default-driver executor for service under p. The service
// matters: an executor is bound to one service (see [chain.Executor]), and
// the injector matches its rules by that service.
func newExec(t *testing.T, service string, p resilience.ClientPolicy) chain.Executor {
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

// TestManager_ZeroConfigIsTransparent: an inert config (no rate, no latency)
// => fn runs, no injection.
func TestManager_ZeroConfigIsTransparent(t *testing.T) {
	in := newFixture(configs{Client: Config{}}, nil) // inert
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestConfig_Inert pins the derived replacement for the former Enabled switch:
// only a rule carrying a rate or a latency can inject, so everything else leaves
// the direction inert.
func TestConfig_Inert(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"empty", Config{}, true},
		{"error only, no rate", Config{Rules: []Rule{{Error: "generic"}}}, true},
		{"guardrails only", Config{Rules: []Rule{{MaxAffected: 3, MaxDuration: time.Second}}}, true},
		{"rule with error only", Config{Rules: []Rule{{Service: "svc", Error: "generic"}}}, true},
		{"rate", Config{Rules: []Rule{{Rate: 0.5}}}, false},
		{"latency", Config{Rules: []Rule{{Latency: time.Millisecond}}}, false},
		{"service rule rate", Config{Rules: []Rule{{Service: "svc", Rate: 1}}}, false},
		{"service rule latency", Config{Rules: []Rule{{Service: "svc", Latency: time.Millisecond}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.That(t, tc.cfg.Inert()).Equal(tc.want)
		})
	}
}

// TestManager_RuleAttribution: the decision names the rule the fire came from —
// the rule's service (or the call's own for a catch-all) plus its index —
// mirroring chain.Injection.Rule's convention on the governance side. With
// rules-only there is no "top-level knobs" fallback: no match means no fire.
func TestManager_RuleAttribution(t *testing.T) {
	// A matched rule: named by service + index.
	in := newFixture(configs{Client: Config{Rules: []Rule{
		{Service: "svc", Rate: 1, Error: "generic"},
	}}}, nil)
	f := decidedAt(t, in, Target{Service: "svc"})
	assert.That(t, f.Rule).Equal("svc[0]")

	// No rule matches: nothing fires, and the decision names nothing.
	in2 := newFixture(configs{Client: Config{Rules: []Rule{
		{Service: "other", Rate: 1, Error: "generic"},
	}}}, nil)
	f2 := decidedAt(t, in2, Target{Service: "svc"})
	assert.That(t, f2.Fired()).False()
	assert.String(t, f2.Rule).Equal("")
}

// decidedAt runs one gated call and returns what the injector decided about it.
// No round is needed: the decision is the return value now, which is the point
// of it being one.
func decidedAt(t *testing.T, in *fixture, tgt Target) Decision {
	t.Helper()
	d, _ := in.client.NewClient().Fire(context.Background(), tgt)
	return d
}

// TestManager_RateZeroAlwaysSucceeds: Rate 0 => never inject.
func TestManager_RateZeroAlwaysSucceeds(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 0, Error: "generic"}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	var calls int32
	for range 10 {
		err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
		assert.Error(t, err).Nil()
	}
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(10))
}

// TestManager_RateOneRetriesInjectedError: every attempt is injected, so the
// real fn never runs and the executor exhausts its retries (MaxRetries+1
// attempts) and returns the injected error.
func TestManager_RateOneRetriesInjectedError(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic"}}}}, nil)
	// MaxRetries 2 => 3 attempts; no breaker so retry is the only path.
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{Retry: chain.RetryPolicy{MaxRetries: 2}}))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
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
	assert.That(t, resilience.ShouldRetry(ErrInjected)).True()
}

// TestManager_KindsSurfaceAsFamiliarErrors: the typed kinds wrap a real error
// so errors.Is matches the underlying sentinel (and the observe outcome mapping
// classifies the call the same way a real timeout/reset would).
func TestManager_KindsSurfaceAsFamiliarErrors(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "timeout"}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True()
	assert.That(t, IsInjected(err)).True()

	in.refresh(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "reset"}}}})
	err = exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
}

// TestManager_BreakerOpensUnderFault: with a tight breaker, injected faults
// trip it open and subsequent calls are rejected as ErrCircuitOpen without
// touching fn — the closed loop (fault → breaker → neutral sentinel) holds.
func TestManager_BreakerOpensUnderFault(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic"}}}}, nil)
	// ErrorThreshold 1 opens after the first failed attempt; OpenDuration long
	// enough that the second call sees it still open.
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{ErrorThreshold: 1, OpenDuration: time.Second}))
	var calls int32
	_ = exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil)) // trips the breaker
	firstCalls := atomic.LoadInt32(&calls)

	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, chain.ErrCircuitOpen)).True()
	// fn not called on the rejected attempt (only the first call's attempts ran).
	assert.That(t, atomic.LoadInt32(&calls)).Equal(firstCalls)
}

// TestManager_LatencySleepsAndCancels: injected latency sleeps, and a cancelled
// attempt context surfaces the context error instead of retrying forever.
func TestManager_LatencySleepsAndCancels(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 0, Latency: 200 * time.Millisecond}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{Timeout: 50 * time.Millisecond}))
	var calls int32
	start := time.Now()
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	elapsed := time.Since(start)
	// The 200ms latency exceeds the 50ms per-attempt timeout: the sleep is
	// cancelled by the attempt deadline and the call fails without fn running.
	assert.Error(t, err).NotNil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(0))
	assert.That(t, elapsed < 200*time.Millisecond).True()
}

// TestManager_LatencyJitterBounds: with LatencyJitter J, every draw lands in
// [Latency-J, Latency+J] clamped to ≥ 0; the midpoint over many draws stays
// near Latency.
func TestManager_LatencyJitterBounds(t *testing.T) {
	const base, jitter = 100 * time.Millisecond, 80 * time.Millisecond
	r := Rule{Latency: base, LatencyJitter: jitter}
	var sum time.Duration
	for i := 0; i < 200; i++ {
		d := r.decide("svc[0]")
		assert.That(t, d.Delay >= base-jitter).True()
		assert.That(t, d.Delay <= base+jitter).True()
		sum += d.Delay
	}
	mid := sum / 200
	assert.That(t, mid > base-40*time.Millisecond && mid < base+40*time.Millisecond).True()
}

// TestClientManager_HotSwap: pushing a new config at runtime via Refresh takes
// effect on the next operation (the Dync-driven path starters use) — here,
// arming a fire and then disarming it by pushing an inert config.
func TestManager_HotSwap(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic"}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	assert.That(t, IsInjected(err)).True()

	in.refresh(configs{Client: Config{}}) // push an inert config: fire off
	err = exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestRefreshPushesIntoExistingShell pins the ownership model: a shell created
// while the manager is inert must start firing after a later push, without ever
// asking the manager on the call path. The manager PUSHES the new kernel into
// every registered shell; the shell reads its own pointer.
func TestRefreshPushesIntoExistingShell(t *testing.T) {
	in := newFixture(configs{Client: Config{}}, nil) // unarmed
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil() // inert: the real fn ran
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// Arm AFTER the shell exists.
	in.client.Refresh(Config{Rules: []Rule{{Service: "svc", Rate: 1, Error: "generic"}}})
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(&calls, nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1)) // fn never ran
}

// TestCloseUnregisters pins the lifecycle half: Close removes the shell from the
// manager's registry, while a sibling keeps receiving pushes.
func TestCloseUnregisters(t *testing.T) {
	in := newFixture(configs{Client: Config{}}, nil)
	a := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	b := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	assert.Number(t, len(in.client.dir.states)).Equal(2)

	err := a.Close()
	assert.Error(t, err).Nil()
	assert.Number(t, len(in.client.dir.states)).Equal(1)

	// The survivor still receives the push and fires.
	in.client.Refresh(Config{Rules: []Rule{{Service: "svc", Rate: 1, Error: "generic"}}})
	err = b.Execute(context.Background(), chain.Operation{Service: "svc"}, nil, countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
}

// TestRefreshPushesIntoServerShell is the inbound counterpart of
// TestRefreshPushesIntoExistingShell.
func TestRefreshPushesIntoServerShell(t *testing.T) {
	in := newFixture(configs{}, nil)
	srv := in.server.NewServer()
	_, err := srv.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()

	in.server.Refresh(Config{Rules: []Rule{{Service: "svc", Rate: 1, Error: "timeout"}}})
	_, err = srv.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
}

// TestManager_ScopeGatesLoadTestTraffic verifies the Scope config restricts
// injection by the load-test marker on the call's context.
func TestManager_ScopeGatesLoadTestTraffic(t *testing.T) {
	prop := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	// Rate 1 + generic error => every in-scope call is faulted.
	mk := func(scope string) chain.Executor {
		in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic", Scope: scope}}}}, prop)
		return in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	}
	realCtx := context.Background()
	loadCtx := prop.WithLoadTest(context.Background())

	// Scope "" (default): both real and load-test traffic get faulted.
	all := mk("")
	assert.That(t, IsInjected(all.Execute(realCtx, chain.Operation{}, nil, countFn(new(int32), nil)))).True()
	assert.That(t, IsInjected(all.Execute(loadCtx, chain.Operation{}, nil, countFn(new(int32), nil)))).True()

	// Scope "real": real traffic faulted, load-test traffic passes through.
	real := mk("real")
	assert.That(t, IsInjected(real.Execute(realCtx, chain.Operation{}, nil, countFn(new(int32), nil)))).True()
	var ltCalls int32
	assert.Error(t, real.Execute(loadCtx, chain.Operation{}, nil, countFn(&ltCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&ltCalls)).Equal(int32(1)) // fn ran, no fault

	// Scope "loadtest": load-test traffic faulted, real traffic passes through.
	lt := mk("loadtest")
	assert.That(t, IsInjected(lt.Execute(loadCtx, chain.Operation{}, nil, countFn(new(int32), nil)))).True()
	var realCalls int32
	assert.Error(t, lt.Execute(realCtx, chain.Operation{}, nil, countFn(&realCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&realCalls)).Equal(int32(1))
}

// TestManager_MaxAffectedCapsBlastRadius verifies MaxAffected stops injecting
// after the configured count of affected calls.
func TestManager_MaxAffectedCapsBlastRadius(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic", MaxAffected: 3}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))

	// First three calls are faulted.
	for range 3 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil)))).True()
	}
	// Fourth onward: guardrail tripped, call passes through untouched.
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_MaxDurationAutoOff verifies MaxDuration turns the fire off after
// the window elapses (a forgotten fault self-heals).
func TestManager_MaxDurationAutoOff(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic", MaxDuration: 40 * time.Millisecond}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))

	// Within the window: faulted.
	assert.That(t, IsInjected(exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil)))).True()
	// After the window: passes through.
	time.Sleep(60 * time.Millisecond)
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_NoGuardrailsUnchanged confirms the default (no guardrails) keeps
// the original behavior: every Rate-1 call faults, indefinitely.
func TestManager_NoGuardrailsUnchanged(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic"}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	for range 10 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil)))).True()
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
		Client: Config{Rules: []Rule{{Rate: 0, Latency: 50 * time.Millisecond}}},
		Server: Config{Rules: []Rule{{Rate: 1, Error: "generic"}}},
	}, nil)
	d, err := in.server.NewServer().Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	assert.That(t, d.Inject).True()
	assert.That(t, d.Kind).Equal("generic")
	// The client config on the same injector does not reach inbound: an outbound
	// call gets the client side's latency and no error.
	start := time.Now()
	assert.Error(t, in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{})).
		Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil))).Nil()
	assert.That(t, time.Since(start) >= 50*time.Millisecond).True()

	// Scope "loadtest" + plain ctx => nothing fires (scope excludes real
	// traffic), so the caller proceeds.
	prop := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	in2 := newFixture(configs{Server: Config{Rules: []Rule{{Rate: 1, Error: "generic", Scope: "loadtest"}}}}, prop)
	_, err = in2.server.NewServer().Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()

	// Scope "loadtest" + load-test ctx => injected.
	_, err = in2.server.NewServer().Fire(prop.WithLoadTest(context.Background()), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
}

// TestManager_DirectionsAreIndependent pins the core of the direction split: a
// fire on one side leaves the other untouched, including the guardrails — each
// direction counts its own MaxAffected, so a client fire that has spent its blast
// radius cannot silence a server fire.
func TestManager_DirectionsAreIndependent(t *testing.T) {
	in := newFixture(configs{
		Client: Config{Rules: []Rule{{Rate: 1, Error: "generic"}}},
		Server: Config{Rules: []Rule{{Rate: 1, Error: "reset"}}},
	}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))

	// Outbound gets the client's kind; inbound gets the server's.
	err := exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil))
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).False()
	assert.That(t, IsInjected(err)).True()

	_, srvErr := in.server.NewServer().Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, errors.Is(srvErr, syscall.ECONNRESET)).True()

	// One side off (a push that disables the client) must not touch the other.
	in.refresh(configs{
		Client: Config{},
		Server: Config{Rules: []Rule{{Rate: 1, Error: "reset"}}},
	})
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil))).Nil()
	_, srvErr = in.server.NewServer().Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(srvErr)).True()
}

// TestManager_GuardrailsPerDirection pins that the safety guardrails are counted
// PER DIRECTION: an outbound fire that has spent its MaxAffected budget cannot
// silence the inbound one, and each self-heals on its own.
func TestManager_GuardrailsPerDirection(t *testing.T) {
	in := newFixture(configs{
		Client: Config{Rules: []Rule{{Rate: 1, Error: "generic", MaxAffected: 2}}},
		Server: Config{Rules: []Rule{{Rate: 1, Error: "reset", MaxAffected: 1}}},
	}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))

	// Spend the client side's budget (2 of 3 calls are affected).
	for range 2 {
		assert.That(t, IsInjected(exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil)))).True()
	}
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{}, nil, countFn(new(int32), nil))).Nil()

	// The server side is still armed: its own counter never saw those calls.
	_, srvErr := in.server.NewServer().Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(srvErr)).True()
	// ...and it self-heals after its own single affected call.
	_, srvErr = in.server.NewServer().Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, srvErr).Nil()
}

// TestManager_PerServiceRules verifies a rule only governs the service it
// names: a non-matching service is not fired at all. First matching rule wins;
// an empty service is a catch-all.
func TestManager_PerServiceRules(t *testing.T) {
	// Only svc-a is armed.
	in := newFixture(configs{Client: Config{Rules: []Rule{
		{Service: "svc-a", Rate: 1, Error: "generic"},
	}}}, nil)
	// One executor per service: an executor is bound to one service now, so each
	// service under test needs its own. The injector is shared, because fault
	// rules are matched by service inside the gate.
	execA := in.client.WrapClientExecutor(newExec(t, "svc-a", resilience.ClientPolicy{}))
	execB := in.client.WrapClientExecutor(newExec(t, "svc-b", resilience.ClientPolicy{}))

	// svc-a matches the rule => faulted.
	assert.That(t, IsInjected(execA.Execute(context.Background(), chain.Operation{Service: "svc-a"}, nil, countFn(new(int32), nil)))).True()
	// svc-b matches no rule, global rate 0 => passes through.
	var calls int32
	assert.Error(t, execB.Execute(context.Background(), chain.Operation{Service: "svc-b"}, nil, countFn(&calls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// Catch-all rule overrides global for any service. Specific rule still wins
	// over the catch-all when listed first.
	in2 := newFixture(configs{Client: Config{Rules: []Rule{
		{Service: "svc-a", Rate: 1, Error: "timeout"},
		{Service: "", Rate: 1, Error: "generic"}, // catch-all
	}}}, nil)
	exec2A := in2.client.WrapClientExecutor(newExec(t, "svc-a", resilience.ClientPolicy{}))
	exec2C := in2.client.WrapClientExecutor(newExec(t, "svc-c", resilience.ClientPolicy{}))
	err := exec2A.Execute(context.Background(), chain.Operation{Service: "svc-a"}, nil, countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True() // svc-a => timeout kind
	// svc-c falls through to the catch-all => generic.
	err = exec2C.Execute(context.Background(), chain.Operation{Service: "svc-c"}, nil, countFn(new(int32), nil))
	assert.That(t, IsInjected(err)).True()
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).False() // generic, not timeout
}

// TestInjectedFaultIsTaggedOnFacts: an armed client fault writes the injection
// onto the call's record, so a layer above (observability, an audit, a chaos
// controller) sees a call was set on fire rather than failing on its own — the
// tag that closes the "injected fault is indistinguishable from a real failure"
// gap.
func TestInjectedFaultIsTaggedOnFacts(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "timeout"}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))
	op := chain.Operation{Service: "svc"}
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), op, rec, countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.Number(t, int(calls)).Equal(0)
	// The client adapter wrote the decision it got back onto the call's record.
	assert.That(t, rec.Fault).NotNil()
	assert.String(t, rec.Fault.Kind).Equal("timeout")
	assert.String(t, rec.Fault.Rule).Equal("svc[0]") // the catch-all rule's identity
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
	in := newFixture(configs{Client: Config{Rules: []Rule{{Rate: 1, Error: "generic", MaxAffected: 1}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{
		Retry: chain.RetryPolicy{MaxRetries: 1},
	}))
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{}, rec, countFn(&calls, nil))

	assert.Error(t, err).Nil()                   // the retry succeeded
	assert.Number(t, int(calls)).Equal(1)        // fn ran on the clean retry
	assert.Number(t, len(rec.Rounds())).Equal(2) // two attempts recorded
	assert.That(t, rec.Fault).NotNil()           // the first attempt's fire survived
	assert.String(t, rec.Fault.Kind).Equal("generic")
}

// TestFaultRuleMatchesMetadata: a fault rule whose Match names a metadata
// dimension fires only for calls that carry it — a rule can target one tenant or
// region of a service, not just the whole service.
func TestFaultRuleMatchesMetadata(t *testing.T) {
	in := newFixture(configs{Client: Config{Rules: []Rule{{
		Service:       "svc",
		MetadataMatch: map[string]string{"tenant": "acme"},
		Rate:          1,
	}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))

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
	in := newFixture(configs{Client: Config{Rules: []Rule{{
		Service:     "svc",
		ActionMatch: ActionMatcher{Exact: "insert"},
		Rate:        1,
	}}}}, nil)
	exec := in.client.WrapClientExecutor(newExec(t, "svc", resilience.ClientPolicy{}))

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
	in := newFixture(configs{Server: Config{Rules: []Rule{{
		Service:       "gin",
		ActionMatch:   ActionMatcher{Prefix: "GET /users"},
		MetadataMatch: map[string]string{"tenant": "acme"},
		Rate:          1,
	}}}}, nil)

	fires := func(tgt Target) bool {
		_, err := in.server.NewServer().Fire(context.Background(), tgt)
		return IsInjected(err)
	}

	// The named route of the named tenant fires.
	assert.That(t, fires(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme"}})).True()
	// Another route of the same server: the interface filter holds it back.
	assert.That(t, fires(Target{Service: "gin", Action: "POST /orders",
		Metadata: map[string]string{"tenant": "acme"}})).False()
	// The same route of another tenant: the metadata filter holds it back.
	assert.That(t, fires(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "other"}})).False()
}

// TestRule_BindsMatchAndAction pins that a rule's Match and Action — fault's own
// types, not chain's — bind from the flattened properties an operator writes.
// Both are reached by reflect kind rather than a registered converter, so what
// matters is the spelling: match.<key> per metadata dimension, action.exact /
// action.prefix for the interface.
func TestRule_BindsMatchAndAction(t *testing.T) {
	storage := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"rules[0].service":       "gin",
		"rules[0].action.exact":  "GET /users/{id}",
		"rules[0].match.tenant":  "acme",
		"rules[0].match.region":  "cn",
		"rules[0].rate":          "1",
		"rules[1].service":       "redis:cache",
		"rules[1].action.prefix": "GET /users",
	}))
	var c Config
	assert.Error(t, conf.Bind(storage, &c)).Nil()

	assert.Number(t, len(c.Rules)).Equal(2)
	r0 := c.Rules[0]
	assert.String(t, r0.Service).Equal("gin")
	assert.String(t, r0.ActionMatch.Exact).Equal("GET /users/{id}")
	assert.String(t, r0.ActionMatch.Prefix).Equal("")
	assert.String(t, r0.MetadataMatch["tenant"]).Equal("acme")
	assert.String(t, r0.MetadataMatch["region"]).Equal("cn")
	assert.Number(t, int(c.Rules[1].Rate)).Equal(0)

	// The bound rule governs exactly the call it names: same service, same
	// interface, matching metadata — and nothing else.
	assert.That(t, r0.matchesRule(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).True()
	assert.That(t, r0.matchesRule(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme"}})).False() // region missing
	assert.That(t, r0.matchesRule(Target{Service: "gin", Action: "POST /orders",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).False()
}
