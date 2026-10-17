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

package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/stdlib/testing/assert"
)

// errFail is a plain failure used to drive the breaker in tests.
var errFail = errors.New("test: fail")

// testManager is a thin wrapper over the outbound authority for tests.
type testManager struct {
	client *ClientManager
}

func newTestManager(d Driver) *testManager {
	return &testManager{client: NewClientManager(d)}
}

func (m *testManager) Apply(s Settings) error {
	return m.client.Apply(s.Client)
}

func (m *testManager) ClientExecutorFor(system, service string) chain.Executor {
	return m.client.ClientExecutorFor(system, service)
}

func (m *testManager) ClientPolicyFor(label string) ClientPolicy {
	return m.client.ClientPolicyFor(label)
}

func (m *testManager) Subscribe(label string, cb func(ClientPolicy)) Subscription {
	return m.client.Subscribe(label, cb)
}

func (m *testManager) Resolve(op chain.Operation) CallPolicy { return m.client.Resolve(op) }

func (m *testManager) Close() error {
	return m.client.Close()
}

// armed returns a manager armed with a fixed per-label policy set, over the
// bundled driver (the driver is fixed at construction now, not part of Settings).
func armed(t *testing.T, policies map[string]ClientPolicy) *testManager {
	t.Helper()
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.Apply(Settings{
		Client: ClientSettings{Rules: clientRules(policies)},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return m
}

// clientRules renders a label -> policy map as the pushed outbound rule list.
func clientRules(policies map[string]ClientPolicy) []ClientRule {
	if len(policies) == 0 {
		return nil
	}
	out := make([]ClientRule, 0, len(policies))
	for label, p := range policies {
		out = append(out, ClientRule{Service: label, ClientPolicy: p})
	}
	return out
}

// runOnce runs fn through e and reports whether it ran.
func runOnce(t *testing.T, e chain.Executor) bool {
	t.Helper()
	ran := false
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return ran
}

// TestManagerUnarmedPanics pins the construction-order contract: an unarmed
// manager refuses to hand out an executor — a silent pass-through would mean
// governance is off without anyone knowing. Arm first ([ClientManager.Apply]), then ask.
func TestManagerUnarmedPanics(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	defer func() {
		if recover() == nil {
			t.Fatal("asking an unarmed manager for an executor must panic")
		}
	}()
	m.ClientExecutorFor("redis", "redis:cache")
}

func TestManagerEmptySettingsIsPassThrough(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.Apply(Settings{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// A document that configures nothing leaves every label on the pass-through —
	// the manager is armed (Enabled reports true), but there is no policy to
	// apply.
	if p := m.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("an empty document must resolve a zero policy, got %+v", p)
	}
	if !runOnce(t, m.ClientExecutorFor("redis", "redis:cache")) {
		t.Fatal("an empty document must still run fn")
	}
}

// TestManagerSharesOneExecutorPerLabel pins the semantic that makes the
// per-label cache load-bearing: two handles for one label must share breaker
// state, or two objects would each trip at their own threshold and effectively
// double it.
func TestManagerSharesOneExecutorPerLabel(t *testing.T) {
	m := armed(t, map[string]ClientPolicy{
		"redis:cache": {ErrorThreshold: 2, OpenDuration: time.Minute},
	})
	a := m.ClientExecutorFor("redis", "redis:cache")
	b := m.ClientExecutorFor("redis", "redis:cache")

	fail := func(e chain.Executor) {
		_ = e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return errFail })
	}
	fail(a)
	fail(b) // second failure across BOTH handles trips the shared breaker

	err := a.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	if err != chain.ErrCircuitOpen {
		t.Fatalf("breaker state must be shared across handles for one label, got %v", err)
	}
}

// TestManagerSubscribeArmsAndNotifies covers the subscription contract: immediate
// arming, notification on change, silence on an unchanged policy, and cancel.
func TestManagerSubscribeArmsAndNotifies(t *testing.T) {
	var mu sync.Mutex
	policies := map[string]ClientPolicy{"redis:cache": {Timeout: time.Second}}
	m := newTestManager(NewDefaultDriver(nil))
	// apply pushes the current "document" (the policies map) down, the way the
	// center does on every source push.
	apply := func() {
		t.Helper()
		mu.Lock()
		rules := clientRules(policies)
		mu.Unlock()
		if err := m.Apply(Settings{Client: ClientSettings{Rules: rules}}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}
	apply()

	var got []ClientPolicy
	sub := m.Subscribe("redis:cache", func(p ClientPolicy) { got = append(got, p) })
	if len(got) != 1 || got[0].Timeout != time.Second {
		t.Fatalf("Subscribe must arm immediately, got %+v", got)
	}
	if sub.ClientPolicy.Timeout != time.Second {
		t.Fatalf("Subscription.ClientPolicy = %+v, want the arming policy", sub.ClientPolicy)
	}

	// An apply that leaves this label's policy unchanged must not notify.
	apply()
	if len(got) != 1 {
		t.Fatalf("an unchanged policy must not re-notify, got %d calls", len(got))
	}

	// A change must notify with the new policy.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{Timeout: 2 * time.Second}
	mu.Unlock()
	apply()
	if len(got) != 2 || got[1].Timeout != 2*time.Second {
		t.Fatalf("a changed policy must notify, got %+v", got)
	}

	// Cancel must stop delivery and be idempotent.
	sub.Cancel()
	sub.Cancel()
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{Timeout: 3 * time.Second}
	mu.Unlock()
	apply()
	if len(got) != 2 {
		t.Fatalf("a cancelled subscription must not be notified, got %d calls", len(got))
	}
	// Cancelling the last subscriber must drop the label's entry entirely: a
	// client that rebuilds itself repeatedly must not grow the table.
	if _, ok := m.client.subs["redis:cache"]; ok {
		t.Fatal("cancelling the last subscriber must drop the label's map entry")
	}
}

// TestSubscription_ZeroIsInert pins that a Subscription is safe to hold and
// cancel even when the manager was never armed: the zero value is inert.
func TestSubscription_ZeroIsInert(t *testing.T) {
	var s Subscription
	s.Cancel()
	if !s.ClientPolicy.IsZero() {
		t.Fatal("zero Subscription must carry a zero policy")
	}
}

// countingDriver records how many executors it built.
type countingDriver struct {
	executors atomic.Int32
}

func (d *countingDriver) NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error) {
	d.executors.Add(1)
	return NewDefaultDriver(nil).NewClientExecutor(service, p)
}

// policyDriver records every (service, policy) it was asked to build, so a test can
// tell WHICH labels a push rebuilt.
type policyDriver struct {
	mu    sync.Mutex
	built []string
}

func (d *policyDriver) NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error) {
	d.mu.Lock()
	d.built = append(d.built, service+"@"+p.Timeout.String())
	d.mu.Unlock()
	return NewDefaultDriver(nil).NewClientExecutor(service, p)
}

func (d *policyDriver) snapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.built...)
}

// TestManagerApplyPublishesForPulling pins both halves of a push under the pull
// design. Apply builds nothing and touches no executor: it publishes a new
// generation. Each executor then follows it on its own next call — the object the
// caller holds is the same one throughout, and what moves is the policy it
// resolves.
func TestManagerApplyPublishesForPulling(t *testing.T) {
	d := &policyDriver{}
	m := newTestManager(d)

	var mu sync.Mutex
	policies := map[string]ClientPolicy{
		"redis:cache": {Timeout: time.Second},
		"redis:queue": {Timeout: time.Second},
	}
	// apply pushes the current "document" (the policies map) down.
	apply := func() {
		t.Helper()
		mu.Lock()
		rules := clientRules(policies)
		mu.Unlock()
		if err := m.Apply(Settings{Client: ClientSettings{Rules: rules}}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	apply()
	cache := m.ClientExecutorFor("redis", "redis:cache")
	queue := m.ClientExecutorFor("redis", "redis:queue")
	if got := d.snapshot(); len(got) != 2 {
		t.Fatalf("want both labels built, got %v", got)
	}
	if got := timeoutOf(t, cache); got != time.Second {
		t.Fatalf("cache must run the policy its label resolves, got %v", got)
	}

	// Move ONE label's policy.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{Timeout: 2 * time.Second}
	mu.Unlock()
	apply()
	if got := d.snapshot(); len(got) != 2 {
		t.Fatalf("a push must not build eagerly, got %v", got)
	}

	// Both labels still resolve through the SAME executor objects — the shell the
	// caller holds is never replaced — and the moved label now runs its new
	// policy, resolved per call.
	if again := m.ClientExecutorFor("redis", "redis:cache"); again != cache {
		t.Fatal("the shell must keep its identity across a push")
	}
	if got := timeoutOf(t, cache); got != 2*time.Second {
		t.Fatalf("the moved label must follow the push on its next call, got %v", got)
	}
	if got := timeoutOf(t, queue); got != time.Second {
		t.Fatalf("the untouched label must keep its policy, got %v", got)
	}
}

// timeoutOf runs one call through e and reports the policy the executor recorded
// for it, which is how a test observes what a label resolves to without reaching
// into the executor.
func timeoutOf(t *testing.T, e chain.Executor) time.Duration {
	t.Helper()
	rec := &chain.Recorder{}
	err := e.Execute(context.Background(), chain.Operation{}, rec, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return rec.Timeout
}

// TestManagerGenerationIsContentDerived pins the generation contract at the
// manager, which is what keeps a config push from resetting protection state it
// did not change.
//
// Two halves. RE-APPLYING AN IDENTICAL DOCUMENT MOVES NOTHING: the generation is
// derived from content, so a file watcher firing twice, or a poll that changed
// nothing, leaves every label exactly where it was. And a change to a rule that is
// NOT the first one naming a service still moves that label — the old
// per-label comparison looked only at the first matching rule and would have
// missed it entirely, leaving the second rule's calls running the old policy
// for ever.
//
// A label's generation is one number, so every call of that label moves together;
// the granularity the design offers is per LABEL, and a change to one service
// leaves every other service alone.
func TestManagerGenerationIsContentDerived(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))

	doc := func(get, set time.Duration) Settings {
		return Settings{Client: ClientSettings{Rules: []ClientRule{
			{Service: "svc", Action: chain.ActionMatch{Exact: "get"}, ClientPolicy: ClientPolicy{Timeout: get}},
			{Service: "svc", Action: chain.ActionMatch{Exact: "set"}, ClientPolicy: ClientPolicy{Timeout: set}},
			{Service: "other", ClientPolicy: ClientPolicy{Timeout: time.Second}},
		}}}
	}
	genOf := func(label string) uint64 {
		return m.Resolve(chain.Operation{Service: label}).Gen
	}

	if err := m.Apply(doc(time.Second, 2*time.Second)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	svc, other := genOf("svc"), genOf("other")

	// The same document again is not a change.
	if err := m.Apply(doc(time.Second, 2*time.Second)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assert.Number(t, genOf("svc")).Equal(svc)
	assert.Number(t, genOf("other")).Equal(other)

	// Moving the SECOND rule naming svc moves svc — its calls now run under a
	// document the materialized state was not built from — and leaves "other"
	// alone.
	if err := m.Apply(doc(time.Second, 3*time.Second)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assert.That(t, genOf("svc") != svc).True()
	assert.Number(t, genOf("other")).Equal(other)
}

// TestManagerPolicyMatch pins the matching this package now owns — it used to
// live in the center's document walk: an exact label match wins, an unmatched
// label falls to Default, a rule with an empty Service never captures a real
// label (it is not a catch-all), and the first matching Rule wins.
func TestManagerPolicyMatch(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.Apply(Settings{
		Client: ClientSettings{
			Default: ClientPolicy{Timeout: time.Second},
			Rules: []ClientRule{
				{Service: "", ClientPolicy: ClientPolicy{Timeout: 9 * time.Second}}, // matches no real label
				{Service: "redis:cache", ClientPolicy: ClientPolicy{Timeout: 2 * time.Second}},
				{Service: "redis:cache", ClientPolicy: ClientPolicy{Timeout: 3 * time.Second}}, // shadowed
			},
		},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := m.ClientPolicyFor("redis:cache").Timeout; got != 2*time.Second {
		t.Fatalf("an exact match must win: got %v", got)
	}
	if got := m.ClientPolicyFor("redis:queue").Timeout; got != time.Second {
		t.Fatalf("an unmatched label must fall to Default: got %v", got)
	}
}

// TestManagerMatchesRuleByAction proves a rule can be selected by the call's
// Action: a rate limit armed for one interface must not touch another interface
// of the same service — the interface-level governance the service label alone
// cannot express.
func TestManagerMatchesRuleByAction(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.Apply(Settings{Client: ClientSettings{Rules: []ClientRule{
		{Service: "svc", Action: chain.ActionMatch{Exact: "limited"}, ClientPolicy: ClientPolicy{RateLimit: 1, Burst: 1}},
	}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	e := m.ClientExecutorFor("svc", "svc")
	noop := func(context.Context) error { return nil }

	limited := chain.Operation{Request: chain.Request{Action: "limited"}}
	other := chain.Operation{Request: chain.Request{Action: "other"}}

	if err := e.Execute(context.Background(), limited, nil, noop); err != nil {
		t.Fatalf("first limited call: %v", err)
	}
	if err := e.Execute(context.Background(), limited, nil, noop); !errors.Is(err, chain.ErrRateLimited) {
		t.Fatalf("second limited call = %v, want ErrRateLimited", err)
	}
	// A different action of the SAME service has its own (default, unmetered)
	// policy — the armed rule must not touch it.
	for range 3 {
		if err := e.Execute(context.Background(), other, nil, noop); err != nil {
			t.Fatalf("other action must be untouched: %v", err)
		}
	}
}
