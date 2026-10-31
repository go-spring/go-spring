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

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// errFail is a plain failure used to drive the breaker in tests.
var errFail = errors.New("test: fail")

// testManager is a thin wrapper over the outbound authority for tests.
type testManager struct {
	mgr *Manager
}

func newTestManager(d Driver) *testManager {
	return &testManager{mgr: NewManager(d)}
}

func (m *testManager) ApplyLimits(s LimitsSettings) error {
	return m.mgr.ApplyLimits(s)
}

func (m *testManager) ApplyBreaker(s BreakerSettings) error {
	return m.mgr.ApplyBreaker(s)
}

func (m *testManager) ApplyRateLimit(s RateLimitSettings) error {
	return m.mgr.ApplyRateLimit(s)
}

func (m *testManager) ExecutorFor(service string) chain.Executor {
	return m.mgr.ExecutorFor(service)
}

func (m *testManager) LimitsFor(label string) Limits {
	return m.mgr.LimitsFor(label)
}

func (m *testManager) SubscribeLimits(label string, cb func(Limits)) Subscription {
	return m.mgr.SubscribeLimits(label, cb)
}

func (m *testManager) Resolve(op chain.Operation) CallPolicy { return m.mgr.Resolve(op) }

func (m *testManager) Close() error {
	return m.mgr.Close()
}

// armedLimits returns a manager whose limits are fixed per label, over the
// bundled driver (the driver is fixed at construction now, not part of settings).
func armedLimits(t *testing.T, limits map[string]Limits) *testManager {
	t.Helper()
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.ApplyLimits(LimitsSettings{Rules: limitsRules(limits)}); err != nil {
		t.Fatalf("ApplyLimits: %v", err)
	}
	return m
}

// armedBreakers returns a manager whose breakers are fixed per label.
func armedBreakers(t *testing.T, breakers map[string]BreakerPolicy) *testManager {
	t.Helper()
	m := newTestManager(NewDefaultDriver(nil))
	rules := make([]BreakerRule, 0, len(breakers))
	for label, p := range breakers {
		rules = append(rules, BreakerRule{Service: label, Action: chain.ActionMatch{Exact: ServiceScopeAction}, BreakerPolicy: p})
	}
	if err := m.ApplyBreaker(BreakerSettings{Rules: rules}); err != nil {
		t.Fatalf("ApplyBreaker: %v", err)
	}
	return m
}

// limitsRules renders a label -> limits map as the pushed limits rule list.
func limitsRules(limits map[string]Limits) []LimitsRule {
	if len(limits) == 0 {
		return nil
	}
	out := make([]LimitsRule, 0, len(limits))
	for label, p := range limits {
		out = append(out, LimitsRule{Service: label, Action: chain.ActionMatch{Exact: ServiceScopeAction}, Limits: p})
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

// TestManagerBuildsBeforeAnySettings pins the construction contract: an executor
// can be built before any settings arrive, because construction reads no
// configuration. Until a document is applied it is a transparent pass-through.
func TestManagerBuildsBeforeAnySettings(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	e := m.ExecutorFor("redis:cache")
	if e == nil {
		t.Fatal("executor construction must not depend on configuration")
	}
	err := e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("an unconfigured executor must pass through, got %v", err)
	}
}

func TestManagerEmptySettingsIsPassThrough(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	// A manager that has never been armed configures nothing, so every label
	// resolves zero limits and the executor is the pass-through.
	if p := m.LimitsFor("redis:cache"); !p.IsZero() {
		t.Fatalf("an empty manager must resolve zero limits, got %+v", p)
	}
	if !runOnce(t, m.ExecutorFor("redis:cache")) {
		t.Fatal("an empty document must still run fn")
	}
}

// TestManagerSharesOneExecutorPerLabel pins the semantic that makes the
// per-label cache load-bearing: two handles for one label must share breaker
// state, or two objects would each trip at their own threshold and effectively
// double it.
func TestManagerSharesOneExecutorPerLabel(t *testing.T) {
	m := armedBreakers(t, map[string]BreakerPolicy{
		"redis:cache": {ErrorThreshold: 2, OpenDuration: time.Minute},
	})
	a := m.ExecutorFor("redis:cache")
	b := m.ExecutorFor("redis:cache")

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
	policies := map[string]Limits{"redis:cache": {Timeout: time.Second}}
	m := newTestManager(NewDefaultDriver(nil))
	// apply pushes the current "document" (the policies map) down, the way the
	// center does on every source push.
	apply := func() {
		t.Helper()
		mu.Lock()
		rules := limitsRules(policies)
		mu.Unlock()
		if err := m.ApplyLimits(LimitsSettings{Rules: rules}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}
	apply()

	var got []Limits
	sub := m.SubscribeLimits("redis:cache", func(p Limits) { got = append(got, p) })
	if len(got) != 1 || got[0].Timeout != time.Second {
		t.Fatalf("Subscribe must arm immediately, got %+v", got)
	}
	if sub.Limits.Timeout != time.Second {
		t.Fatalf("Subscription.Policy = %+v, want the arming policy", sub.Limits)
	}

	// An apply that leaves this label's policy unchanged must not notify.
	apply()
	if len(got) != 1 {
		t.Fatalf("an unchanged policy must not re-notify, got %d calls", len(got))
	}

	// A change must notify with the new policy.
	mu.Lock()
	policies["redis:cache"] = Limits{Timeout: 2 * time.Second}
	mu.Unlock()
	apply()
	if len(got) != 2 || got[1].Timeout != 2*time.Second {
		t.Fatalf("a changed policy must notify, got %+v", got)
	}

	// Cancel must stop delivery and be idempotent.
	sub.Cancel()
	sub.Cancel()
	mu.Lock()
	policies["redis:cache"] = Limits{Timeout: 3 * time.Second}
	mu.Unlock()
	apply()
	if len(got) != 2 {
		t.Fatalf("a cancelled subscription must not be notified, got %d calls", len(got))
	}
	// Cancelling the last subscriber must drop the label's entry entirely: a
	// client that rebuilds itself repeatedly must not grow the table.
	if _, ok := m.mgr.subs["redis:cache"]; ok {
		t.Fatal("cancelling the last subscriber must drop the label's map entry")
	}
}

// TestSubscription_ZeroIsInert pins that a Subscription is safe to hold and
// cancel even when the manager was never armed: the zero value is inert.
func TestSubscription_ZeroIsInert(t *testing.T) {
	var s Subscription
	s.Cancel()
	if !s.Limits.IsZero() {
		t.Fatal("zero Subscription must carry a zero policy")
	}
}

// countingDriver records how many executors it built.
type countingDriver struct {
	executors atomic.Int32
}

func (d *countingDriver) NewExecutor(label string, m *Manager) (chain.Executor, error) {
	d.executors.Add(1)
	return NewDefaultDriver(nil).NewExecutor(label, m)
}

// TestManagerApplyPublishesForPulling pins both halves of a push under the pull
// design. Apply builds nothing and touches no executor: it publishes a new
// generation. Each executor then follows it on its own next call — the object the
// caller holds is the same one throughout, and what moves is the policy it
// resolves.
func TestManagerApplyPublishesForPulling(t *testing.T) {
	d := &countingDriver{}
	m := newTestManager(d)

	var mu sync.Mutex
	policies := map[string]Limits{
		"redis:cache": {Timeout: time.Second},
		"redis:queue": {Timeout: time.Second},
	}
	// apply pushes the current "document" (the policies map) down.
	apply := func() {
		t.Helper()
		mu.Lock()
		rules := limitsRules(policies)
		mu.Unlock()
		if err := m.ApplyLimits(LimitsSettings{Rules: rules}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	apply()
	cache := m.ExecutorFor("redis:cache")
	queue := m.ExecutorFor("redis:queue")
	if got := d.executors.Load(); got != 2 {
		t.Fatalf("want both labels built, got %d", got)
	}
	if got := timeoutOf(t, cache); got != time.Second {
		t.Fatalf("cache must run the policy its label resolves, got %v", got)
	}

	// Move ONE label's policy.
	mu.Lock()
	policies["redis:cache"] = Limits{Timeout: 2 * time.Second}
	mu.Unlock()
	apply()
	if got := d.executors.Load(); got != 2 {
		t.Fatalf("a push must not build eagerly, got %d", got)
	}

	// Both labels still resolve through the SAME executor objects — the shell the
	// caller holds is never replaced — and the moved label now runs its new
	// policy, resolved per call.
	if again := m.ExecutorFor("redis:cache"); again != cache {
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

	doc := func(get, set time.Duration) LimitsSettings {
		return LimitsSettings{Rules: []LimitsRule{
			{Service: "svc", Action: chain.ActionMatch{Exact: "get"}, Limits: Limits{Timeout: get}},
			{Service: "svc", Action: chain.ActionMatch{Exact: "set"}, Limits: Limits{Timeout: set}},
			{Service: "other", Action: chain.ActionMatch{Exact: ServiceScopeAction}, Limits: Limits{Timeout: time.Second}},
		}}
	}
	genOf := func(label string) uint64 {
		return m.Resolve(chain.Operation{Service: label}).Gen
	}

	if err := m.ApplyLimits(doc(time.Second, 2*time.Second)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	svc, other := genOf("svc"), genOf("other")

	// The same document again is not a change.
	if err := m.ApplyLimits(doc(time.Second, 2*time.Second)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assert.Number(t, genOf("svc")).Equal(svc)
	assert.Number(t, genOf("other")).Equal(other)

	// Moving the SECOND rule naming svc moves svc — its calls now run under a
	// document the materialized state was not built from — and leaves "other"
	// alone.
	if err := m.ApplyLimits(doc(time.Second, 3*time.Second)); err != nil {
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
	if err := m.ApplyLimits(LimitsSettings{
		Default: Limits{Timeout: time.Second},
		Rules: []LimitsRule{
			{Service: "redis:cache", Action: chain.ActionMatch{Exact: ServiceScopeAction}, Limits: Limits{Timeout: 2 * time.Second}},
		},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := m.LimitsFor("redis:cache").Timeout; got != 2*time.Second {
		t.Fatalf("an exact match must win: got %v", got)
	}
	if got := m.LimitsFor("redis:queue").Timeout; got != time.Second {
		t.Fatalf("an unmatched label must fall to Default: got %v", got)
	}
}

// TestManagerRejectsDuplicateRules pins the rule-list contract: two entries that
// select the SAME calls make the winner depend on list order, so the document is
// rejected.
func TestManagerRejectsDuplicateRules(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	err := m.ApplyLimits(LimitsSettings{Rules: []LimitsRule{
		{Service: "redis:cache", Limits: Limits{Timeout: time.Second}},
		{Service: "redis:cache", Action: chain.ActionMatch{Exact: ServiceScopeAction}, Limits: Limits{Timeout: 2 * time.Second}},
	}})
	if err == nil {
		t.Fatal("a duplicate rule must be rejected")
	}

	// A rule without a service names nothing and is rejected, not silent.
	if err := m.ApplyLimits(LimitsSettings{Rules: []LimitsRule{{Limits: Limits{Timeout: time.Second}}}}); err == nil {
		t.Fatal("a rule without a service must be rejected")
	}
}

// TestMostSpecificRuleWins pins the matching order: the narrowest selector wins
// regardless of its position in the list — action exact > action prefix (longer
// first) > selector size — so a broad rule listed before a narrow one never
// shadows it.
func TestMostSpecificRuleWins(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.ApplyLimits(LimitsSettings{Rules: []LimitsRule{
		{Service: "svc", Action: chain.ActionMatch{Exact: "GET /users/1"}, Limits: Limits{Timeout: 3 * time.Second}},
		{Service: "svc", Action: chain.ActionMatch{Exact: ServiceScopeAction}, Limits: Limits{Timeout: time.Second}},
		{Service: "svc", Action: chain.ActionMatch{Prefix: "GET /users"}, Limits: Limits{Timeout: 2 * time.Second}},
		{Service: "svc", Action: chain.ActionMatch{Prefix: "GET /users"}, Match: matcher.ExactMap{"tenant": "a"}, Limits: Limits{Timeout: 4 * time.Second}},
	}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	resolve := func(action string, metadata map[string]string) time.Duration {
		return m.Resolve(chain.Operation{
			Service: "svc",
			Request: chain.Request{Action: action, Metadata: metadata},
		}).Limits.Timeout
	}
	assert.That(t, resolve("GET /users/1", nil)).Equal(3 * time.Second)                              // exact beats prefix
	assert.That(t, resolve("GET /users/2", nil)).Equal(2 * time.Second)                              // prefix beats the service default
	assert.That(t, resolve("GET /users/2", map[string]string{"tenant": "a"})).Equal(4 * time.Second) // larger selector
	assert.That(t, resolve("POST /x", nil)).Equal(time.Second)                                       // service default
}

// TestAmbiguousRulesRejected pins the other half: two rules of equal specificity
// that can match the same call are a document error, not a list-order hint.
func TestAmbiguousRulesRejected(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	err := m.ApplyLimits(LimitsSettings{Rules: []LimitsRule{
		{Service: "svc", Action: chain.ActionMatch{Prefix: "GET /"}, Match: matcher.ExactMap{"tenant": "a"}, Limits: Limits{Timeout: time.Second}},
		{Service: "svc", Action: chain.ActionMatch{Prefix: "GET /"}, Match: matcher.ExactMap{"region": "b"}, Limits: Limits{Timeout: 2 * time.Second}},
	}})
	if err == nil {
		t.Fatal("equal-specificity selectors that can co-match must be rejected")
	}
}

// TestManagerMatchesRuleByAction proves a rule can be selected by the call's
// Action: a rate limit armed for one interface must not touch another interface
// of the same service — the interface-level governance the service label alone
// cannot express.
func TestManagerMatchesRuleByAction(t *testing.T) {
	m := newTestManager(NewDefaultDriver(nil))
	if err := m.ApplyRateLimit(RateLimitSettings{Rules: []RateLimitRule{
		{Service: "svc", Action: chain.ActionMatch{Exact: "limited"}, RateLimitPolicy: RateLimitPolicy{RateLimit: 1, Burst: 1}},
	}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	e := m.ExecutorFor("svc")
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

// TestApplyRejectsInvalidPolicy verifies a document that cannot materialize a
// stage is rejected whole, instead of publishing a generation that silently
// drops it.
func TestApplyRejectsInvalidPolicy(t *testing.T) {
	m := NewManager(NewDefaultDriver(nil))
	if err := m.ApplyBreaker(BreakerSettings{Default: BreakerPolicy{BreakerStrategy: "bogus"}}); err == nil {
		t.Fatal("Apply accepted an unknown breaker strategy")
	}
}
