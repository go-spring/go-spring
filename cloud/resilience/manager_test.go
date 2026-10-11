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
	"go-spring.org/cloud/chain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// errFail is a plain failure used to drive the breaker in tests.
var errFail = errors.New("test: fail")

// armed returns a manager armed with a fixed per-label policy set, over the
// bundled driver (the driver is fixed at construction now, not part of Settings).
func armed(t *testing.T, policies map[string]ClientPolicy) *Manager {
	t.Helper()
	m := NewManager(nil)
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

// serverRules renders a label -> policy map as the pushed inbound rule list.
func serverRules(policies map[string]ServerPolicy) []ServerRule {
	if len(policies) == 0 {
		return nil
	}
	out := make([]ServerRule, 0, len(policies))
	for label, p := range policies {
		out = append(out, ServerRule{Service: label, ServerPolicy: p})
	}
	return out
}

// runOnce runs fn through e and reports whether it ran.
func runOnce(t *testing.T, e chain.Executor) bool {
	t.Helper()
	ran := false
	err := e.Execute(context.Background(), chain.Operation{}, func(context.Context) error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return ran
}

func TestManagerUnarmedIsPassThrough(t *testing.T) {
	m := NewManager(nil)
	if !runOnce(t, m.ClientExecutorFor("redis", "redis:cache")) {
		t.Fatal("unarmed manager must run fn")
	}
	if p := m.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("unarmed manager must resolve a zero policy, got %+v", p)
	}
}

func TestManagerEmptySettingsIsPassThrough(t *testing.T) {
	m := NewManager(nil)
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
		_ = e.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return errFail })
	}
	fail(a)
	fail(b) // second failure across BOTH handles trips the shared breaker

	err := a.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil })
	if err != chain.ErrCircuitOpen {
		t.Fatalf("breaker state must be shared across handles for one label, got %v", err)
	}
}

// TestManagerSubscribeArmsAndNotifies covers the subscription contract: immediate
// arming, notification on change, silence on an unchanged policy, and cancel.
func TestManagerSubscribeArmsAndNotifies(t *testing.T) {
	var mu sync.Mutex
	policies := map[string]ClientPolicy{"redis:cache": {AttemptTimeout: time.Second}}
	m := NewManager(nil)
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
	if len(got) != 1 || got[0].AttemptTimeout != time.Second {
		t.Fatalf("Subscribe must arm immediately, got %+v", got)
	}
	if sub.ClientPolicy.AttemptTimeout != time.Second {
		t.Fatalf("Subscription.ClientPolicy = %+v, want the arming policy", sub.ClientPolicy)
	}

	// An apply that leaves this label's policy unchanged must not notify.
	apply()
	if len(got) != 1 {
		t.Fatalf("an unchanged policy must not re-notify, got %d calls", len(got))
	}

	// A change must notify with the new policy.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 2 * time.Second}
	mu.Unlock()
	apply()
	if len(got) != 2 || got[1].AttemptTimeout != 2*time.Second {
		t.Fatalf("a changed policy must notify, got %+v", got)
	}

	// Cancel must stop delivery and be idempotent.
	sub.Cancel()
	sub.Cancel()
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 3 * time.Second}
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

// TestManagerServerLaneIsSeparate pins that the two directions are separate
// lanes over separate models: ServerExecutorFor builds through Driver.NewServerExecutor
// (not NewClientExecutor), reads the inbound resolver, and a client-side policy change
// never touches the inbound model.
func TestManagerServerLaneIsSeparate(t *testing.T) {
	drv := &countingDriver{}
	m := NewManager(drv)

	var mu sync.Mutex
	policies := map[string]ClientPolicy{"redis:cache": {AttemptTimeout: time.Second}}
	serverPolicies := map[string]ServerPolicy{"gin::8080": {RateLimit: 7}}
	if err := m.Apply(Settings{
		Client: ClientSettings{Rules: clientRules(policies)},
		Server: ServerSettings{Rules: serverRules(serverPolicies)},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	runOnce(t, m.ClientExecutorFor("redis", "redis:cache"))
	if err := m.ServerExecutorFor("gin", "gin::8080").Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("inbound Execute: %v", err)
	}

	if got := drv.executors.Load(); got != 1 {
		t.Fatalf("want one outbound build, got %d", got)
	}
	if got := drv.serverPolicies.Load(); got != 1 {
		t.Fatalf("ServerExecutorFor must build through NewServerExecutor, got %d inbound builds", got)
	}
	if got := drv.lastServerPolicy.Load().(ServerPolicy).RateLimit; got != 7 {
		t.Fatalf("ServerExecutorFor must resolve the ADMISSION resolver, got rate limit %v", got)
	}

	// A later outbound-only change leaves the inbound model untouched.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 2 * time.Second}
	mu.Unlock()
	if err := m.Apply(Settings{
		Client: ClientSettings{Rules: clientRules(policies)},
		Server: ServerSettings{Rules: serverRules(serverPolicies)},
	}); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if got := drv.serverPolicies.Load(); got != 1 {
		t.Fatalf("an outbound-only change must not rebuild inbound inbound, got %d builds", got)
	}
}

// countingDriver records how many executors it built per direction, and what
// inbound model it was last handed.
type countingDriver struct {
	executors        atomic.Int32
	serverPolicies   atomic.Int32
	lastServerPolicy atomic.Value // ServerPolicy
}

func (d *countingDriver) NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error) {
	d.executors.Add(1)
	return NewDefaultDriver(nil).NewClientExecutor(service, p)
}

func (d *countingDriver) NewServerExecutor(service string, a ServerPolicy) (chain.Executor, error) {
	d.serverPolicies.Add(1)
	d.lastServerPolicy.Store(a)
	return NewDefaultDriver(nil).NewServerExecutor(service, a)
}

// policyDriver records every (service, policy) it was asked to build, so a test can
// tell WHICH labels a push rebuilt.
type policyDriver struct {
	mu    sync.Mutex
	built []string
}

func (d *policyDriver) NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error) {
	d.mu.Lock()
	d.built = append(d.built, service+"@"+p.AttemptTimeout.String())
	d.mu.Unlock()
	return NewDefaultDriver(nil).NewClientExecutor(service, p)
}

func (d *policyDriver) NewServerExecutor(service string, p ServerPolicy) (chain.Executor, error) {
	return NewDefaultDriver(nil).NewServerExecutor(service, p)
}

func (d *policyDriver) snapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.built...)
}

// TestManagerApplyEvictsOnlyChangedLabels pins both halves of a push: a label whose
// policy moved is rebuilt under the new one, and a label whose policy did NOT move
// keeps the executor it already had — with its breaker and rate-limit state —
// instead of being churned by an unrelated change.
func TestManagerApplyEvictsOnlyChangedLabels(t *testing.T) {
	d := &policyDriver{}
	m := NewManager(d)

	var mu sync.Mutex
	policies := map[string]ClientPolicy{
		"redis:cache": {AttemptTimeout: time.Second},
		"redis:queue": {AttemptTimeout: time.Second},
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
	runOnce(t, m.ClientExecutorFor("redis", "redis:cache"))
	runOnce(t, m.ClientExecutorFor("redis", "redis:queue"))
	if got := d.snapshot(); len(got) != 2 {
		t.Fatalf("want both labels built, got %v", got)
	}

	// Move ONE label's policy.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 2 * time.Second}
	mu.Unlock()
	apply()
	if got := d.snapshot(); len(got) != 2 {
		t.Fatalf("a push must not build eagerly, got %v", got)
	}

	// The unchanged label keeps its executor.
	runOnce(t, m.ClientExecutorFor("redis", "redis:queue"))
	if got := d.snapshot(); len(got) != 2 {
		t.Fatalf("the unchanged label must keep its executor, got %v", got)
	}

	// The changed one is rebuilt, under the policy the push moved it to.
	runOnce(t, m.ClientExecutorFor("redis", "redis:cache"))
	got := d.snapshot()
	if len(got) != 3 || got[2] != "redis:cache@2s" {
		t.Fatalf("the changed label must be rebuilt under the new policy, got %v", got)
	}
}

// TestManagerPolicyMatch pins the matching this package now owns — it used to
// live in the center's document walk: an exact label match wins, an unmatched
// label falls to Default, a rule with an empty Service never captures a real
// label (it is not a catch-all), and the first matching Rule wins. Each
// direction matches over its own block.
func TestManagerPolicyMatch(t *testing.T) {
	m := NewManager(nil)
	if err := m.Apply(Settings{
		Client: ClientSettings{
			Default: ClientPolicy{AttemptTimeout: time.Second},
			Rules: []ClientRule{
				{Service: "", ClientPolicy: ClientPolicy{AttemptTimeout: 9 * time.Second}}, // matches no real label
				{Service: "redis:cache", ClientPolicy: ClientPolicy{AttemptTimeout: 2 * time.Second}},
				{Service: "redis:cache", ClientPolicy: ClientPolicy{AttemptTimeout: 3 * time.Second}}, // shadowed
			},
		},
		Server: ServerSettings{
			Default: ServerPolicy{RateLimit: 5},
			Rules:   []ServerRule{{Service: "gin::8080", ServerPolicy: ServerPolicy{RateLimit: 7}}},
		},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := m.ClientPolicyFor("redis:cache").AttemptTimeout; got != 2*time.Second {
		t.Fatalf("an exact match must win: got %v", got)
	}
	if got := m.ClientPolicyFor("redis:queue").AttemptTimeout; got != time.Second {
		t.Fatalf("an unmatched label must fall to Default: got %v", got)
	}
	if got := m.ServerPolicyFor("gin::8080").RateLimit; got != 7 {
		t.Fatalf("an inbound exact match must win: got %v", got)
	}
	if got := m.ServerPolicyFor("gin::9090").RateLimit; got != 5 {
		t.Fatalf("an unmatched route must fall to Default: got %v", got)
	}
}
