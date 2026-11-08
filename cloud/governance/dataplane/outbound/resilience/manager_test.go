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
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// testManager is a thin wrapper over the outbound authority for tests.
type testManager struct {
	mgr *Manager
}

func newTestManager() *testManager {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		panic(err)
	}
	mgr, err := NewManager(factories)
	if err != nil {
		panic(err)
	}
	return &testManager{mgr: mgr}
}

func (m *testManager) ApplyTimeoutRetry(s TimeoutRetrySettings) error {
	return m.mgr.ApplyTimeoutRetry(s)
}

func (m *testManager) ApplyBreaker(s BreakerSettings) error {
	return m.mgr.ApplyBreaker(s)
}

func (m *testManager) ApplyRateLimit(s RateLimitSettings) error {
	return m.mgr.ApplyRateLimit(s)
}

func (m *testManager) resolve(op chain.Operation) TimeoutRetryRule {
	rule, _ := m.mgr.timeoutRetrySettings().findRule(op)
	return rule
}

func (m *testManager) Close() error {
	return m.mgr.Close()
}

// limitsRules renders a label -> limits map as the pushed limits document.
func limitsRules(limits map[string]TimeoutRetry) []TimeoutRetryService {
	if len(limits) == 0 {
		return nil
	}
	out := make([]TimeoutRetryService, 0, len(limits))
	for label, p := range limits {
		out = append(out, TimeoutRetryService{
			Service: label,
			Rules:   []TimeoutRetryRule{{ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction}, TimeoutRetry: p}},
		})
	}
	return out
}

// runOnce runs fn through e and reports whether it ran.
func runOnce(t *testing.T, e chain.Executor) bool {
	t.Helper()
	ran := false
	err := e.Execute(context.Background(), chain.Operation{}, new(chain.Recorder), func(context.Context) error {
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
	m := newTestManager()
	e := m.mgr.Wrap("redis:cache", nil)
	if e == nil {
		t.Fatal("executor construction must not depend on configuration")
	}
	err := e.Execute(context.Background(), chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("an unconfigured executor must pass through, got %v", err)
	}
}

func TestManagerEmptySettingsIsPassThrough(t *testing.T) {
	m := newTestManager()
	// A manager that has never been armed configures nothing, so every label
	// resolves zero limits and the executor is the pass-through.
	if p := m.resolve(chain.Operation{Service: "redis:cache"}); p.Timeout != 0 {
		t.Fatalf("an empty manager must resolve zero limits, got %+v", p)
	}
	if !runOnce(t, m.mgr.Wrap("redis:cache", nil)) {
		t.Fatal("an empty document must still run fn")
	}
}

// TestManagerApplyPublishesForPulling pins the limits half of a push. TimeoutRetry
// carry no materialized state, so Apply only publishes the new document; each
// executor resolves it on its own next call — the object the caller holds keeps
// serving, and what moves is the policy it resolves.
func TestManagerApplyPublishesForPulling(t *testing.T) {
	m := newTestManager()

	var mu sync.Mutex
	policies := map[string]TimeoutRetry{
		"redis:cache": {Timeout: time.Second},
		"redis:queue": {Timeout: time.Second},
	}
	// apply pushes the current "document" (the policies map) down.
	apply := func() {
		t.Helper()
		mu.Lock()
		rules := limitsRules(policies)
		mu.Unlock()
		if err := m.ApplyTimeoutRetry(TimeoutRetrySettings{Services: rules}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	apply()
	cache := m.mgr.Wrap("redis:cache", nil)
	queue := m.mgr.Wrap("redis:queue", nil)
	if got := timeoutOf(t, cache, "redis:cache"); got != time.Second {
		t.Fatalf("cache must run the policy its label resolves, got %v", got)
	}

	// Move ONE label's policy.
	mu.Lock()
	policies["redis:cache"] = TimeoutRetry{Timeout: 2 * time.Second}
	mu.Unlock()
	apply()

	// The handle the caller already holds follows the push: the policy is
	// resolved through the label's shared engine on the next call, not carried by
	// the node.
	if got := timeoutOf(t, cache, "redis:cache"); got != 2*time.Second {
		t.Fatalf("the moved label must follow the push on its next call, got %v", got)
	}
	if got := timeoutOf(t, queue, "redis:queue"); got != time.Second {
		t.Fatalf("the untouched label must keep its policy, got %v", got)
	}
}

// timeoutOf runs one call through e and reports the policy the executor recorded
// for it, which is how a test observes what a label resolves to without reaching
// into the executor.
func timeoutOf(t *testing.T, e chain.Executor, service string) time.Duration {
	t.Helper()
	rec := &chain.Recorder{}
	err := e.Execute(context.Background(), chain.Operation{Service: service}, rec, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return rec.Timeout
}

// TestManagerPushIsContentDerivedAndPerLabel pins the push contract at the
// manager, which is what keeps a config push from resetting protection state it
// did not change.
//
// Two halves. RE-APPLYING AN IDENTICAL DOCUMENT MOVES NOTHING: the comparison is
// by content, so a file watcher firing twice, or a poll that changed nothing,
// leaves every label's breaker table exactly where it was. And a change to a
// rule that is NOT the first one naming a service still moves that label — the
// fingerprint covers every rule naming it, not just the first match.
//
// The granularity is per LABEL: a change to one service leaves every other
// service alone.
func TestManagerPushIsContentDerivedAndPerLabel(t *testing.T) {
	m := newTestManager()

	doc := func(get, set int) BreakerSettings {
		return BreakerSettings{Services: []BreakerService{
			{Service: "svc", Rules: []BreakerRule{
				{ActionMatch: matcher.ActionMatch{Exact: "get"}, BreakerPolicy: BreakerPolicy{ErrorThreshold: get}},
				{ActionMatch: matcher.ActionMatch{Exact: "set"}, BreakerPolicy: BreakerPolicy{ErrorThreshold: set}},
			}},
			{Service: "other", Rules: []BreakerRule{
				{ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction}, BreakerPolicy: BreakerPolicy{ErrorThreshold: 5}},
			}},
		}}
	}
	apply := func(get, set int) {
		t.Helper()
		if err := m.ApplyBreaker(doc(get, set)); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}
	slotOf := func(e *Executor) *breakerSlot { return e.breaker.Load() }

	apply(2, 3)
	svc := m.mgr.Wrap("svc", nil)
	other := m.mgr.Wrap("other", nil)
	svcSlot, otherSlot := slotOf(svc), slotOf(other)

	// The same document again is not a change.
	apply(2, 3)
	assert.That(t, slotOf(svc) == svcSlot).True()
	assert.That(t, slotOf(other) == otherSlot).True()

	// Moving the SECOND rule naming svc replaces svc's table — its calls now run
	// under a document the materialized state was not built from — and leaves
	// "other" alone.
	apply(2, 4)
	assert.That(t, slotOf(svc) != svcSlot).True()
	assert.That(t, slotOf(other) == otherSlot).True()
}

// TestManagerPolicyMatch pins the matching this package now owns — it used to
// live in the center's document walk: an exact label match wins, a rule with an
// empty Service never captures a real label (it is not a catch-all), and an
// unmatched label resolves to no policy at all.
func TestManagerPolicyMatch(t *testing.T) {
	m := newTestManager()
	if err := m.ApplyTimeoutRetry(TimeoutRetrySettings{
		Services: []TimeoutRetryService{{
			Service: "redis:cache",
			Rules: []TimeoutRetryRule{{
				ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
				TimeoutRetry: TimeoutRetry{Timeout: 2 * time.Second},
			}},
		}},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := m.resolve(chain.Operation{Service: "redis:cache"}).Timeout; got != 2*time.Second {
		t.Fatalf("an exact match must win: got %v", got)
	}
	if got := m.resolve(chain.Operation{Service: "redis:queue"}).Timeout; got != 0 {
		t.Fatalf("an unmatched label must have no policy: got %v", got)
	}
}

// TestDuplicateRulesFirstWins pins that a repeated selector is legal: rules are
// ordered, so the later copy is simply unreachable. Validation belongs to the
// source boundary (conf.Bind runs it); Apply trusts the document it is handed.
func TestDuplicateRulesFirstWins(t *testing.T) {
	cfg := TimeoutRetrySettings{Services: []TimeoutRetryService{{
		Service: "redis:cache",
		Rules: []TimeoutRetryRule{
			{ActionMatch: matcher.ActionMatch{Exact: "GET"}, TimeoutRetry: TimeoutRetry{Timeout: time.Second}},
			{ActionMatch: matcher.ActionMatch{Exact: "GET"}, TimeoutRetry: TimeoutRetry{Timeout: 2 * time.Second}},
		},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a duplicate rule is order-shadowed, not invalid: %v", err)
	}
	m := newTestManager()
	if err := m.ApplyTimeoutRetry(cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := m.resolve(chain.Operation{Service: "redis:cache", Request: chain.Request{Action: "GET"}}).Timeout; got != time.Second {
		t.Fatalf("the first copy must win: got %v", got)
	}
}

// TestMostSpecificRuleWins pins the matching order: the narrowest selector wins
// regardless of its position in the list — action exact > action prefix (longer
// first) > selector size — so a broad rule listed before a narrow one never
// shadows it.
func TestMostSpecificRuleWins(t *testing.T) {
	m := newTestManager()
	if err := m.ApplyTimeoutRetry(TimeoutRetrySettings{Services: []TimeoutRetryService{{
		Service: "svc",
		Rules: []TimeoutRetryRule{
			{ActionMatch: matcher.ActionMatch{Exact: "GET /users/1"}, TimeoutRetry: TimeoutRetry{Timeout: 3 * time.Second}},
			{ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction}, TimeoutRetry: TimeoutRetry{Timeout: time.Second}},
			{ActionMatch: matcher.ActionMatch{Prefix: "GET /users"}, TimeoutRetry: TimeoutRetry{Timeout: 2 * time.Second}},
			{ActionMatch: matcher.ActionMatch{Prefix: "GET /users"}, MetadataMatch: map[string]matcher.StringMatch{"tenant": {Exact: "a"}}, TimeoutRetry: TimeoutRetry{Timeout: 4 * time.Second}},
		},
	}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	resolve := func(action string, metadata map[string]string) time.Duration {
		return m.resolve(chain.Operation{
			Service: "svc",
			Request: chain.Request{Action: action, Metadata: metadata},
		}).Timeout
	}
	assert.That(t, resolve("GET /users/1", nil)).Equal(3 * time.Second)                              // exact beats prefix
	assert.That(t, resolve("GET /users/2", nil)).Equal(2 * time.Second)                              // prefix beats the service default
	assert.That(t, resolve("GET /users/2", map[string]string{"tenant": "a"})).Equal(4 * time.Second) // larger selector
	assert.That(t, resolve("POST /x", nil)).Equal(time.Second)                                       // service default
}

// TestOverlappingRulesOrderDecides pins that equal-specificity selectors which
// can co-match are legal: list order breaks the tie, and Validate accepts them —
// avoiding such a document is the configuration platform's job.
func TestOverlappingRulesOrderDecides(t *testing.T) {
	cfg := TimeoutRetrySettings{Services: []TimeoutRetryService{{
		Service: "svc",
		Rules: []TimeoutRetryRule{
			{ActionMatch: matcher.ActionMatch{Prefix: "GET /"}, MetadataMatch: map[string]matcher.StringMatch{"tenant": {Exact: "a"}}, TimeoutRetry: TimeoutRetry{Timeout: time.Second}},
			{ActionMatch: matcher.ActionMatch{Prefix: "GET /"}, MetadataMatch: map[string]matcher.StringMatch{"region": {Exact: "b"}}, TimeoutRetry: TimeoutRetry{Timeout: 2 * time.Second}},
		},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("overlapping rules are order-decided, not invalid: %v", err)
	}
	m := newTestManager()
	if err := m.ApplyTimeoutRetry(cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	both := m.resolve(chain.Operation{Service: "svc", Request: chain.Request{
		Action:   "GET /x",
		Metadata: map[string]string{"tenant": "a", "region": "b"},
	}})
	assert.That(t, both.Timeout).Equal(time.Second)
	second := m.resolve(chain.Operation{Service: "svc", Request: chain.Request{
		Action:   "GET /x",
		Metadata: map[string]string{"region": "b"},
	}})
	assert.That(t, second.Timeout).Equal(2 * time.Second)
}

// TestManagerMatchesRuleByAction proves a rule can be selected by the call's
// Action: a rate limit armed for one interface must not touch another interface
// of the same service — the interface-level governance the service label alone
// cannot express.
func TestManagerMatchesRuleByAction(t *testing.T) {
	m := newTestManager()
	if err := m.ApplyRateLimit(RateLimitSettings{Services: []RateLimitService{{
		Service: "svc",
		Rules: []RateLimitRule{{
			ActionMatch: matcher.ActionMatch{Exact: "limited"},
			Policy:      ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1},
		}},
	}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	e := m.mgr.Wrap("svc", nil)
	noop := func(context.Context) error { return nil }

	limited := chain.Operation{Service: "svc", Request: chain.Request{Action: "limited"}}
	other := chain.Operation{Service: "svc", Request: chain.Request{Action: "other"}}

	if err := e.Execute(context.Background(), limited, new(chain.Recorder), noop); err != nil {
		t.Fatalf("first limited call: %v", err)
	}
	if err := e.Execute(context.Background(), limited, new(chain.Recorder), noop); !errors.Is(err, chain.ErrRateLimited) {
		t.Fatalf("second limited call = %v, want ErrRateLimited", err)
	}
	// A different action of the SAME service has no matching rule, so it is
	// unmetered — the armed rule must not touch it.
	for range 3 {
		if err := e.Execute(context.Background(), other, new(chain.Recorder), noop); err != nil {
			t.Fatalf("other action must be untouched: %v", err)
		}
	}
}

// TestValidateRejectsInvalidPolicy verifies a document that cannot materialize a
// stage is rejected at the boundary, instead of reaching the manager.
func TestValidateRejectsInvalidPolicy(t *testing.T) {
	if err := (BreakerSettings{Services: []BreakerService{{
		Service: "svc",
		Rules: []BreakerRule{{
			ActionMatch:   matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			BreakerPolicy: BreakerPolicy{BreakerStrategy: "bogus"},
		}},
	}}}).Validate(); err == nil {
		t.Fatal("ValidateBreaker accepted an unknown strategy")
	}
}
