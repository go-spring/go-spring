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
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"go-spring.org/cloud/governance/capability/matcher"

	kfault "go-spring.org/cloud/governance/capability/fault"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/stdlib/testing/assert"
)

// configs is the test-side convenience for setting both directions at once. The
// production API arms each direction with its own manager's Refresh; an empty
// slice is inert, so a test names only the side it means to arm.
type configs struct {
	Client []trule
	Server []trule
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
	in.client.Refresh(ClientConfig{Rules: clientRules(c.Client)})
	in.server.Refresh(serverfault.ServerConfig{Rules: serverRules(c.Server)})
}

// passThrough is the chain below the injector when a test asserts fault behavior
// alone: the fire happens, then the real call runs. What a fire does to retry,
// breaker and timeout is pinned by the resilience package's own tests, which
// compose this package inward.
type passThrough struct{}

func (passThrough) Execute(ctx context.Context, _ chain.Operation, rec *chain.Recorder, fn chain.RoundFunc) error {
	return fn(ctx, rec.LastRound())
}

func (passThrough) Close() error { return nil }

// loopRound mimics the governance retry loop AROUND the injector: it opens the
// round the injector's per-attempt fire lands on and seals the measured facts
// when the round returns. In the real stack the loop is above the fault layer;
// tests that assert fault behavior alone reproduce just this much of it.
type loopRound struct{ inner chain.Executor }

func (l loopRound) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn chain.RoundFunc) error {
	rd := rec.NewRound()
	start := time.Now()
	err := l.inner.Execute(ctx, op, rec, fn)
	rd.Start = start
	rd.Duration = time.Since(start)
	rd.Status = chain.ClassifyStatus(err)
	rd.ErrorType = chain.RoundErrorType(err)
	rd.Error = err
	return err
}

func (l loopRound) Close() error { return l.inner.Close() }

// countFn returns an operation fn that increments calls and returns a fixed
// error (or nil) so tests can assert how many times the real fn ran.
func countFn(counter *int32, err error) func(context.Context, *chain.Round) error {
	return func(context.Context, *chain.Round) error {
		atomic.AddInt32(counter, 1)
		return err
	}
}

// TestManager_ZeroConfigIsTransparent: an inert config (no rate, no latency)
// => fn runs, no injection.
func TestManager_ZeroConfigIsTransparent(t *testing.T) {
	in := newFixture(configs{Client: []trule{}}) // inert
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_RuleAttribution: the decision names the rule the fire came from —
// the rule's configured Name — so a platform can join its
// own experiment record with the call. With rules-only there is no "top-level
// knobs" fallback: no match means no fire.
func TestManager_RuleAttribution(t *testing.T) {
	// A matched rule: named by its identity.
	in := newFixture(configs{Client: []trule{
		{Name: "guarded-svc", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}})
	f := decidedAt(t, in, kfault.Target{Service: "svc"})
	assert.That(t, f.Fired()).True()
	assert.That(t, f.Name).Equal("guarded-svc")

	// No rule matches: nothing fires, and the decision names nothing.
	in2 := newFixture(configs{Client: []trule{
		{Name: "other-rule", Service: "other", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}})
	f2 := decidedAt(t, in2, kfault.Target{Service: "svc"})
	assert.That(t, f2.Fired()).False()
	assert.That(t, f2.Fired()).False()
}

// decidedAt runs one gated call and returns what the injector decided about it.
// No round is needed: the decision is the return value now, which is the point
// of it being one.
func decidedAt(t *testing.T, in *fixture, tgt kfault.Target) kfault.Result {
	t.Helper()
	d, _ := in.client.Fire(context.Background(), tgt)
	return d
}

// TestManager_RateZeroAlwaysSucceeds: Rate 0 => never inject.
func TestManager_RateZeroAlwaysSucceeds(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 0, Class: kfault.ClassTransient, Error: "generic"}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	var calls int32
	for range 10 {
		err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
		assert.Error(t, err).Nil()
	}
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(10))
}

// TestManager_KindsSurfaceAsFamiliarErrors: the typed kinds wrap a real error
// so errors.Is matches the underlying sentinel (and the observe outcome mapping
// classifies the call the same way a real timeout/reset would).
func TestManager_KindsSurfaceAsFamiliarErrors(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, os.ErrDeadlineExceeded)).True()
	assert.That(t, kfault.IsInjected(err)).True()

	in.refresh(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "reset"}}})
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
}

// TestManager_HotSwap: pushing a new config at runtime via Refresh takes
// effect on the next operation (the Dync-driven path starters use) — here,
// arming a fire and then disarming it by pushing an inert config.
func TestManager_HotSwap(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.That(t, kfault.IsInjected(err)).True()

	in.refresh(configs{Client: []trule{}}) // push an inert config: fire off
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestRefreshReachesExistingExecutor pins the publish model: an executor created
// while the manager is inert must start firing after a later push, without ever
// asking the manager on the call path. The authority publishes one atomic kernel
// pointer; every call loads whatever generation is current.
func TestRefreshReachesExistingExecutor(t *testing.T) {
	in := newFixture(configs{Client: []trule{}}) // unarmed
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).Nil() // inert: the real fn ran
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// Arm AFTER the executor exists.
	in.client.Refresh(ClientConfig{Rules: clientRules([]trule{{Name: "arm", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}})})
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
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

	in.server.Refresh(serverfault.ServerConfig{Rules: serverRules([]trule{{Name: "arm", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"}})})
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
		r := trule{Name: "traffic", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}
		if class != "" {
			r.MetadataMatch = map[string]matcher.StringMatch{"traffic": {Exact: class}}
		}
		in := newFixture(configs{Client: []trule{r}})
		return loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	}
	op := func(class string) chain.Operation {
		return chain.Operation{Service: "svc", Request: chain.Request{
			Metadata: map[string]string{"traffic": class},
		}}
	}

	// No traffic condition: both real and load-test traffic get faulted.
	all := mk("")
	assert.That(t, kfault.IsInjected(all.Execute(context.Background(), op("real"), new(chain.Recorder), countFn(new(int32), nil)))).True()
	assert.That(t, kfault.IsInjected(all.Execute(context.Background(), op("loadtest"), new(chain.Recorder), countFn(new(int32), nil)))).True()

	// match.traffic.exact=real: real traffic faulted, load-test passes through.
	real := mk("real")
	assert.That(t, kfault.IsInjected(real.Execute(context.Background(), op("real"), new(chain.Recorder), countFn(new(int32), nil)))).True()
	var ltCalls int32
	assert.Error(t, real.Execute(context.Background(), op("loadtest"), new(chain.Recorder), countFn(&ltCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&ltCalls)).Equal(int32(1)) // fn ran, no fault

	// match.traffic.exact=loadtest: load-test faulted, real passes through.
	lt := mk("loadtest")
	assert.That(t, kfault.IsInjected(lt.Execute(context.Background(), op("loadtest"), new(chain.Recorder), countFn(new(int32), nil)))).True()
	var realCalls int32
	assert.Error(t, lt.Execute(context.Background(), op("real"), new(chain.Recorder), countFn(&realCalls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&realCalls)).Equal(int32(1))
}

// TestManager_MaxAffectedCapsBlastRadius verifies MaxAffected stops injecting
// after the configured count of affected calls.
func TestManager_MaxAffectedCapsBlastRadius(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxAffected: 3}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// First three calls are faulted.
	for range 3 {
		assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil)))).True()
	}
	// Fourth onward: guardrail tripped, call passes through untouched.
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_MaxDurationAutoOff verifies MaxDuration turns the fire off after
// the window elapses (a forgotten fault self-heals).
func TestManager_MaxDurationAutoOff(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxDuration: 40 * time.Millisecond}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// Within the window: faulted.
	assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil)))).True()
	// After the window: passes through.
	time.Sleep(60 * time.Millisecond)
	var calls int32
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(&calls, nil))
	assert.Error(t, err).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))
}

// TestManager_NoGuardrailsUnchanged confirms the default (no guardrails) keeps
// the original behavior: every Rate-1 call faults, indefinitely.
func TestManager_NoGuardrailsUnchanged(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	for range 10 {
		assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil)))).True()
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
		Client: []trule{{Name: "fire", Service: "svc", Rate: 0, Class: kfault.ClassTransient, Error: kfault.ErrorGeneric, Latency: 50 * time.Millisecond}},
		Server: []trule{{Name: "fire", Service: "svc", Rate: 1, Status: 503}},
	})
	d, err := in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, d.Status).Equal(503)
	// The client config on the same injector does not reach inbound: an outbound
	// call gets the client side's latency and no error.
	start := time.Now()
	assert.Error(t, loopRound{inner: in.client.WrapClientExecutor(passThrough{})}.
		Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))).Nil()
	assert.That(t, time.Since(start) >= 50*time.Millisecond).True()

	// match.traffic.exact=loadtest + real call => nothing fires, so the caller proceeds.
	in2 := newFixture(configs{Server: []trule{{
		Name: "loadtest-only", Service: "svc", Rate: 1, Status: 503,
		MetadataMatch: map[string]matcher.StringMatch{"traffic": {Exact: "loadtest"}},
	}}})
	srv := in2.server.NewServer()
	_, err = srv.Fire(context.Background(), kfault.Target{Service: "svc", Metadata: map[string]string{"traffic": "real"}})
	assert.Error(t, err).Nil()

	// match.traffic.exact=loadtest + load-test call => injected.
	_, err = srv.Fire(context.Background(), kfault.Target{Service: "svc", Metadata: map[string]string{"traffic": "loadtest"}})
	assert.That(t, kfault.IsInjected(err)).True()
}

// TestManager_DirectionsAreIndependent pins the core of the direction split: a
// fire on one side leaves the other untouched, including the guardrails — each
// direction counts its own MaxAffected, so a client fire that has spent its blast
// radius cannot silence a server fire.
func TestManager_DirectionsAreIndependent(t *testing.T) {
	in := newFixture(configs{
		Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}},
		Server: []trule{{Name: "fire", Service: "svc", Rate: 1, Status: 503}},
	})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// Outbound gets the client's kind; inbound gets the server's response.
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).False()
	assert.That(t, kfault.IsInjected(err)).True()

	srvRes, srvErr := in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(srvErr)).True()
	assert.Number(t, srvRes.Status).Equal(503)

	// One side off (a push that disables the client) must not touch the other.
	in.refresh(configs{
		Client: []trule{},
		Server: []trule{{Name: "fire", Service: "svc", Rate: 1, Status: 503}},
	})
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))).Nil()
	_, srvErr = in.server.NewServer().Fire(context.Background(), kfault.Target{Service: "svc"})
	assert.That(t, kfault.IsInjected(srvErr)).True()
}

// TestManager_GuardrailsPerDirection pins that the safety guardrails are counted
// PER DIRECTION: an outbound fire that has spent its MaxAffected budget cannot
// silence the inbound one, and each self-heals on its own.
func TestManager_GuardrailsPerDirection(t *testing.T) {
	in := newFixture(configs{
		Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic", MaxAffected: 2}},
		Server: []trule{{Name: "fire", Service: "svc", Rate: 1, Status: 503, MaxAffected: 1}},
	})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// Spend the client side's budget (2 of 3 calls are affected).
	for range 2 {
		assert.That(t, kfault.IsInjected(exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil)))).True()
	}
	assert.Error(t, exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder), countFn(new(int32), nil))).Nil()

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
	in := newFixture(configs{Client: []trule{
		{Name: "svc-a-rule", Service: "svc-a", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}})
	// One executor per service: an executor is bound to one service now, so each
	// service under test needs its own. The injector is shared, because fault
	// rules are matched by service inside the gate.
	execA := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	execB := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// svc-a matches the rule => faulted.
	assert.That(t, kfault.IsInjected(execA.Execute(context.Background(), chain.Operation{Service: "svc-a"}, new(chain.Recorder), countFn(new(int32), nil)))).True()
	// svc-b matches no rule, global rate 0 => passes through.
	var calls int32
	assert.Error(t, execB.Execute(context.Background(), chain.Operation{Service: "svc-b"}, new(chain.Recorder), countFn(&calls, nil))).Nil()
	assert.That(t, atomic.LoadInt32(&calls)).Equal(int32(1))

	// A second rule names a different service; each service meets its own rule.
	in2 := newFixture(configs{Client: []trule{
		{Name: "svc-a-timeout", Service: "svc-a", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"},
		{Name: "svc-c-generic", Service: "svc-c", Rate: 1, Class: kfault.ClassTransient, Error: "generic"},
	}})
	exec2A := loopRound{inner: in2.client.WrapClientExecutor(passThrough{})}
	exec2C := loopRound{inner: in2.client.WrapClientExecutor(passThrough{})}
	err := exec2A.Execute(context.Background(), chain.Operation{Service: "svc-a"}, new(chain.Recorder), countFn(new(int32), nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, errors.Is(err, os.ErrDeadlineExceeded)).True() // svc-a => timeout kind
	// svc-c meets its own rule => generic.
	err = exec2C.Execute(context.Background(), chain.Operation{Service: "svc-c"}, new(chain.Recorder), countFn(new(int32), nil))
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, errors.Is(err, os.ErrDeadlineExceeded)).False() // generic, not timeout
}

// TestInjectedFaultIsTaggedOnFacts: an armed client fault writes the injection
// onto the call's record, so a layer above (observability, an audit, a chaos
// controller) sees a call was set on fire rather than failing on its own — the
// tag that closes the "injected fault is indistinguishable from a real failure"
// gap.
func TestInjectedFaultIsTaggedOnFacts(t *testing.T) {
	in := newFixture(configs{Client: []trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "timeout"}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	op := chain.Operation{Service: "svc"}
	rec := &chain.Recorder{}
	var calls int32
	err := exec.Execute(context.Background(), op, rec, countFn(&calls, nil))
	assert.Error(t, err).NotNil()
	assert.Number(t, int(calls)).Equal(0)
	// The client adapter wrote the decision it got back onto the call's record.
	last := rec.Rounds[len(rec.Rounds)-1]
	assert.That(t, last.Fault).NotNil()
	assert.String(t, last.Fault.Error).Equal("timeout")
	assert.That(t, last.Fault.Fired()).True() // the rule's configured identity
	assert.String(t, last.Fault.Name).Equal("fire")
}

// TestFaultRuleMatchesMetadata: a fault rule whose Match names a metadata
// dimension fires only for calls that carry it — a rule can target one tenant or
// region of a service, not just the whole service.
func TestFaultRuleMatchesMetadata(t *testing.T) {
	in := newFixture(configs{Client: []trule{{
		Name:          "acme-only",
		Service:       "svc",
		MetadataMatch: map[string]matcher.StringMatch{"tenant": {Exact: "acme"}},
		Rate:          1,
	}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// A call tagged tenant=acme is faulted.
	var a int32
	opA := chain.Operation{Service: "svc", Request: chain.Request{Metadata: map[string]string{"tenant": "acme"}}}
	assert.Error(t, exec.Execute(context.Background(), opA, new(chain.Recorder), countFn(&a, nil))).NotNil()
	assert.Number(t, int(a)).Equal(0)

	// Another tenant falls through to the (inert) default: untouched.
	var b int32
	opB := chain.Operation{Service: "svc", Request: chain.Request{Metadata: map[string]string{"tenant": "other"}}}
	assert.Error(t, exec.Execute(context.Background(), opB, new(chain.Recorder), countFn(&b, nil))).Nil()
	assert.Number(t, int(b)).Equal(1)
}

// TestFaultRuleMatchesAction: a fault rule whose Action names an interface fires
// only for calls of that interface — one endpoint can be set on fire without
// touching another.
func TestFaultRuleMatchesAction(t *testing.T) {
	in := newFixture(configs{Client: []trule{{
		Name:        "insert-only",
		Service:     "svc",
		ActionMatch: matcher.ActionMatch{Exact: "insert"},
		Rate:        1,
	}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}

	// The named interface is faulted.
	var a int32
	opA := chain.Operation{Service: "svc", Request: chain.Request{Action: "insert"}}
	assert.Error(t, exec.Execute(context.Background(), opA, new(chain.Recorder), countFn(&a, nil))).NotNil()
	assert.Number(t, int(a)).Equal(0)

	// A different interface falls through to the (inert) default: untouched.
	var b int32
	opB := chain.Operation{Service: "svc", Request: chain.Request{Action: "select"}}
	assert.Error(t, exec.Execute(context.Background(), opB, new(chain.Recorder), countFn(&b, nil))).Nil()
	assert.Number(t, int(b)).Equal(1)
}

// TestFireServer_RuleMatchesActionAndMetadata pins the capability the server
// seam did not have while it handed the injector a zero chain.Operation: an
// inbound rule can name ONE interface of the server and one slice of its
// traffic, instead of only being able to fire the whole entrance.
func TestFireServer_RuleMatchesActionAndMetadata(t *testing.T) {
	in := newFixture(configs{Server: []trule{{
		Name:          "users-acme",
		Service:       "gin",
		ActionMatch:   matcher.ActionMatch{Prefix: "GET /users"},
		MetadataMatch: map[string]matcher.StringMatch{"tenant": {Exact: "acme"}},
		Rate:          1,
		Status:        400,
	}}})

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

// TestInjectedRunIDOnRecord: the document's run id rides through the decision to
// the call record, so this fire can be grouped with the other replicas' fires
// from the same drill.
func TestInjectedRunIDOnRecord(t *testing.T) {
	in := newFixture(configs{})
	in.client.Refresh(ClientConfig{
		RunID: "drill-9",
		Rules: clientRules([]trule{{Name: "fire", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}),
	})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	rec := &chain.Recorder{}
	_ = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(new(int32), nil))
	last := rec.Rounds[len(rec.Rounds)-1]
	assert.That(t, last.Fault).NotNil()
	assert.String(t, last.Fault.RunID).Equal("drill-9")
}

// TestManager_StatusRidesDecisionAndError: a declared protocol status rides both
// the call record (telemetry) and the injected error (for an adapter to
// translate).
func TestManager_StatusRidesDecisionAndError(t *testing.T) {
	in := newFixture(configs{Client: []trule{{
		Name: "fire", Service: "svc", Rate: 1, Status: 503,
	}}})
	exec := loopRound{inner: in.client.WrapClientExecutor(passThrough{})}
	rec := &chain.Recorder{}
	err := exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec, countFn(new(int32), nil))

	assert.Number(t, rec.Rounds[len(rec.Rounds)-1].Fault.Status).Equal(503)
	s, ok := kfault.StatusOf(err)
	assert.That(t, ok).True()
	assert.Number(t, s).Equal(503)
}
