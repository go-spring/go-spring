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

package governance

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/guard"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// dur is a shorthand for declaring timeouts in test policies.
func dur(d int) time.Duration { return time.Duration(d) * time.Millisecond }

// timeoutConfig returns a Config whose client Default policy sets a per-attempt
// timeout, the knob the center exists to centralize.
func timeoutConfig(d int) Config {
	return Config{
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{Timeout: dur(d)}},
		},
	}
}

// newAuthorities returns the three module authorities a center distributes to.
// The wiring starter creates them as beans; a test creates them per center so no
// state leaks between tests. Each runs on the bundled driver — the backend is a
// startup choice, not part of the document, so a center test never selects one.
func newAuthorities() (*resilience.ClientManager,
	*loadbalance.Manager, *fault.ClientManager, *fault.ServerManager) {
	driver := resilience.NewDefaultDriver(nil)
	return resilience.NewClientManager(driver), loadbalance.NewManager(),
		fault.NewClientManager(nil), fault.NewServerManager(nil)
}

// newTestCenterWith builds a center over fresh module authorities and dispatches
// cfg into them, exactly as the wiring starter does in production.
func newTestCenterWith(cfg Config) (*Center, *resilience.ClientManager,
	*loadbalance.Manager, *fault.ClientManager, *fault.ServerManager) {
	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, lb, fc, fs, nil, NewPushSource(cfg))
	if err != nil {
		panic(err)
	}
	return c, resClient, lb, fc, fs
}

// newTestCenter is newTestCenterWith for the common case of the bundled driver.
func newTestCenter(cfg Config) *Center {
	c, _, _, _, _ := newTestCenterWith(cfg)
	return c
}

func TestPolicyFor_Default(t *testing.T) {
	c := newTestCenter(timeoutConfig(100))
	if p := c.resClient.ClientPolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("policyFor default: want timeout 100ms, got %v", p.Timeout)
	}
	// Unknown label also falls back to Default.
	if p := c.resClient.ClientPolicyFor("anything:else"); p.Timeout != dur(100) {
		t.Fatalf("policyFor fallback: want timeout 100ms, got %v", p.Timeout)
	}
}

func TestPolicyFor_RuleReplacesDefault(t *testing.T) {
	c := newTestCenter(Config{
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{Timeout: dur(100), Retry: chain.RetryPolicy{MaxRetries: 1}}},
			Rules: []ClientRule{{
				Service:      "redis:cache",
				ClientPolicy: resilience.ClientPolicy{Timeout: dur(50)}, // no MaxRetries
			}},
		},
	})
	p := c.resClient.ClientPolicyFor("redis:cache")
	if p.Timeout != dur(50) {
		t.Fatalf("rule timeout: want 50ms, got %v", p.Timeout)
	}
	// A Rule fully replaces Default, so MaxRetries from Default does NOT carry
	// over — that is the documented "complete, self-contained policy" semantic.
	if p.Retry.MaxRetries != 0 {
		t.Fatalf("rule must replace not merge: want MaxRetries 0, got %d", p.Retry.MaxRetries)
	}
	// A label no Rule matches still gets Default.
	if p := c.resClient.ClientPolicyFor("redis:other"); p.Timeout != dur(100) {
		t.Fatalf("unmatched label: want default 100ms, got %v", p.Timeout)
	}
}

func TestPolicyFor_EmptyServiceMatchesNothing(t *testing.T) {
	// A Rule with an empty Service matches nothing (use Default instead).
	c := newTestCenter(Config{
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{Timeout: dur(100)}},
			Rules:   []ClientRule{{ClientPolicy: resilience.ClientPolicy{Timeout: dur(50)}}},
		},
	})
	if p := c.resClient.ClientPolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("empty-Service rule must not match: want default 100ms, got %v", p.Timeout)
	}
}

func TestServerPolicyFor_ServerBlock(t *testing.T) {
	c := newTestCenter(Config{
		Server: ServerConfig{
			Default: guard.Policy{Timeout: dur(100)},
			Rules: []ServerRule{{
				Service: "gin::8080",
				Policy:  guard.Policy{RateLimit: 500},
			}},
		},
	})
	// A matched inbound rule fully replaces the server default.
	if a := c.GuardServer().PolicyFor("gin::8080"); a.RateLimit != 500 || a.Timeout != 0 {
		t.Fatalf("inbound rule must replace the default wholesale: %+v", a)
	}
	if a := c.GuardServer().PolicyFor("echo::9090"); a.Timeout != dur(100) {
		t.Fatalf("unmatched route: want the server default, got %+v", a)
	}
	// The two directions are separate resolutions over separate rules: the client
	// half of the same document holds nothing for these labels.
	if p := c.resClient.ClientPolicyFor("gin::8080"); !p.IsZero() {
		t.Fatalf("inbound must not leak into the outbound policy: %+v", p)
	}
}

func TestServerPolicyFor_EmptyDocumentIsZero(t *testing.T) {
	c := newTestCenter(Config{})
	if a := c.GuardServer().PolicyFor("gin::8080"); !a.IsZero() {
		t.Fatalf("an empty document must yield a zero inbound, got %+v", a)
	}
}

// TestDispatch_RejectsDuplicateServerRules is the server-side twin of the
// duplicate-label guard: an inbound list naming the same route twice is rejected
// whole, so which entry wins can never depend on list order.
func TestDispatch_RejectsDuplicateServerRules(t *testing.T) {
	c := newTestCenter(Config{
		Server: ServerConfig{
			Rules: []ServerRule{{Service: "gin::8080", Policy: guard.Policy{Timeout: dur(50)}}},
		},
	})
	bad := Config{
		Server: ServerConfig{
			Rules: []ServerRule{
				{Service: "gin::8080", Policy: guard.Policy{RateLimit: 1}},
				{Service: "gin::8080", Policy: guard.Policy{RateLimit: 999}},
			},
		},
	}
	if err := c.dispatch(context.Background(), bad); err == nil {
		t.Fatal("dispatch must reject duplicate INBOUND service labels")
	}
	if a := c.GuardServer().PolicyFor("gin::8080"); a.Timeout != dur(50) {
		t.Fatalf("rejected push must not leak into the serving snapshot: got %+v", a)
	}
}

func TestPolicyFor_EmptyDocumentIsPassThrough(t *testing.T) {
	// An empty document resolves zero policies for every label: that is what
	// "governance off" means now that there is no enabled switch.
	c := newTestCenter(Config{})
	if p := c.resClient.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("an empty document must yield zero policy, got %+v", p)
	}
	// A subscription is still registered — a later push must be able to notify
	// it — but it is armed with a ZERO policy, a transparent pass-through.
	var got resilience.ClientPolicy
	s := c.resClient.Subscribe("redis:cache", func(p resilience.ClientPolicy) { got = p })
	if !got.IsZero() || !s.ClientPolicy.IsZero() {
		t.Fatalf("an empty document must arm with a zero policy, got %+v", got)
	}
}

func TestSubscribe_ArmsImmediately(t *testing.T) {
	c := newTestCenter(timeoutConfig(100))
	var got resilience.ClientPolicy
	c.resClient.Subscribe("redis:cache", func(p resilience.ClientPolicy) { got = p })
	if got.Timeout != dur(100) {
		t.Fatalf("Subscribe should arm cb with current policy, got timeout %v", got.Timeout)
	}
}

// Adopt must notify only the labels whose resolved policy actually changed, so a
// localized rule edit does not churn unrelated executors — and a no-op adopt
// notifies nobody.
func TestAdopt_NotifiesOnlyChangedLabels(t *testing.T) {
	c := newTestCenter(Config{
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{Timeout: dur(100)}},
		},
	})

	var redisN, gormN int
	c.resClient.Subscribe("redis:cache", func(resilience.ClientPolicy) { redisN++ })
	c.resClient.Subscribe("gorm:mysql:primary", func(resilience.ClientPolicy) { gormN++ })
	// Subscribe arms once each.
	if redisN != 1 || gormN != 1 {
		t.Fatalf("after subscribe: redis=%d gorm=%d, want 1/1", redisN, gormN)
	}

	// Change only redis:cache. gorm must NOT be notified (its policy unchanged).
	cfg := timeoutConfig(100)
	cfg.Client.Rules = []ClientRule{{
		Service:      "redis:cache",
		ClientPolicy: resilience.ClientPolicy{Timeout: dur(200)},
	}}
	c.adopt(context.Background(), cfg)
	if redisN != 2 {
		t.Fatalf("redis must be notified on its policy change: got %d, want 2", redisN)
	}
	if gormN != 1 {
		t.Fatalf("gorm must NOT be notified when its policy is unchanged: got %d, want 1", gormN)
	}
	if p := c.resClient.ClientPolicyFor("redis:cache"); p.Timeout != dur(200) {
		t.Fatalf("post-adopt redis policy: want 200ms, got %v", p.Timeout)
	}

	// A no-op adopt (same config) notifies nobody.
	c.adopt(context.Background(), cfg)
	if redisN != 2 || gormN != 1 {
		t.Fatalf("no-op adopt must not notify: redis=%d gorm=%d, want 2/1", redisN, gormN)
	}
}

func TestAdopt_DefaultChangeFansOutToAllUnoverridden(t *testing.T) {
	c := newTestCenter(timeoutConfig(100))
	var a, b int
	c.resClient.Subscribe("redis:cache", func(resilience.ClientPolicy) { a++ })
	c.resClient.Subscribe("gorm:mysql:primary", func(resilience.ClientPolicy) { b++ })
	// Both read Default; changing Default must notify both.
	c.adopt(context.Background(), timeoutConfig(300))
	if a != 2 || b != 2 {
		t.Fatalf("default change should fan out to both: a=%d b=%d, want 2/2", a, b)
	}
	if p := c.resClient.ClientPolicyFor("redis:cache"); p.Timeout != dur(300) {
		t.Fatalf("post default-change: want 300ms, got %v", p.Timeout)
	}
}

// TestSource_AdoptSwapsFaultConfig guards the single-sink property of adopt:
// a config pushed from ANY source swaps BOTH the resilience snapshot and the
// fault injector's config (the injector pointer itself is never rebuilt, which
// is what lets clients hold it for the process lifetime).
func TestSource_AdoptSwapsFaultConfig(t *testing.T) {
	c, _, _, fc, fs := newTestCenterWith(Config{})
	if !fc.Config().Inert() || !fs.Config().Inert() {
		t.Fatal("precondition: injector should start inert")
	}

	cfg := timeoutConfig(100)
	cfg.Client.Fault = fault.Config{Rules: []fault.Rule{{Rate: 0.5}}}
	cfg.Server.Fault = fault.Config{Rules: []fault.Rule{{Rate: 1, Latency: dur(5)}}}
	c.adopt(context.Background(), cfg)

	if p := c.resClient.ClientPolicyFor("x"); p.Timeout != dur(100) {
		t.Fatalf("adopt resilience side: want 100ms, got %v", p.Timeout)
	}
	if cf := fc.Config(); len(cf.Rules) != 1 || cf.Rules[0].Rate != 0.5 {
		t.Fatalf("adopt client fault side: injector config not swapped: %+v", cf)
	}
	// The two sides are pushed independently: the same adopt must have landed the
	// server-side config too, and neither may bleed into the other.
	if sf := fs.Config(); len(sf.Rules) != 1 ||
		sf.Rules[0].Rate != 1 || sf.Rules[0].Latency != dur(5) {
		t.Fatalf("adopt server fault side: injector config not swapped: %+v", sf)
	}
}

// TestCenter_ConstructorSourceBinds pins the source rule: the source handed to
// NewCenter is bound at construction (the container's contribution) — its
// snapshot arms the authorities in the same breath.
func TestCenter_ConstructorSourceBinds(t *testing.T) {
	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, lb, fc, fs, nil, NewPushSource(timeoutConfig(300)))
	if err != nil {
		t.Fatal(err)
	}
	if p := c.resClient.ClientPolicyFor("x"); p.Timeout != dur(300) {
		t.Fatalf("the constructor's source must be bound at construction: want 300ms, got %v", p.Timeout)
	}
}

// TestCenter_ConstructsArmed covers the starter-facing contract: construction
// distributes the source's snapshot to the module authorities — including the
// fault injector.
func TestCenter_ConstructsArmed(t *testing.T) {
	resClient, lb, fc, fs := newAuthorities()
	_, err := NewCenter(resClient, lb, fc, fs, nil, NewPushSource(Config{
		Client: ClientConfig{Fault: fault.Config{Rules: []fault.Rule{{Rate: 0.25}}}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cf := fc.Config(); len(cf.Rules) != 1 || cf.Rules[0].Rate != 0.25 {
		t.Fatal("construction should push the snapshot's client Fault into the injected injector")
	}
}

// selectionPool builds a pool over a one-endpoint static source — enough for the
// distribution tests, which assert the policy that reaches the pool rather than a
// routing decision. It mirrors what a client starter builds: a suspension tracker
// attached so the thresholds have somewhere to land.
// selectionChain builds the endpoint-selection chain a client starter would: an
// endpoint source registered under the label with the center's loadbalance
// authority, so the rule reaching that label reaches this chain.
func selectionChain(m *loadbalance.Manager, label string) *loadbalance.Chain {
	src := func() ([]discovery.Endpoint, error) {
		return []discovery.Endpoint{{Addr: "10.0.0.1:8080", Healthy: true, Weight: 1}}, nil
	}
	return m.Chain(context.Background(), src, label)
}

// selectionConfig returns a Config with one rule for label naming a strategy and
// an outlier-suspension policy. It sets only the selection half — the protection
// half stays zero, which is what proves the two halves are independent.
func selectionConfig(label, balancer string, threshold int, suspendFor time.Duration) Config {
	return Config{
		Client: ClientConfig{
			Rules: []ClientRule{{
				Service: label,
				Selection: loadbalance.Selection{
					Balancer:          balancer,
					OutlierThreshold:  threshold,
					OutlierSuspendFor: suspendFor,
				},
			}},
		},
	}
}

// TestSelection_AppliesCurrentThenChanges is the end-to-end control-plane test:
// the label's rule reaches the chain at registration time (before the first Pick,
// not only on the next push), and a later config change rebuilds it — which is
// what makes endpoint selection a hot-reloadable knob rather than a
// construction-time constant.
func TestSelection_AppliesCurrentThenChanges(t *testing.T) {
	const label = "http:user-svc"
	c := newTestCenter(selectionConfig(label, loadbalance.LeastConn, 5, 10*time.Second))
	chain := selectionChain(c.lb, label)

	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, chain.Selection().OutlierThreshold).Equal(5)
	assert.That(t, chain.Selection().OutlierSuspendFor).Equal(10 * time.Second)
	// The suspension half landed on the tracker too, not just on the record.
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(5)

	// A pushed change rebuilds the chain, and the new generation carries it.
	c.adopt(context.Background(), selectionConfig(label, loadbalance.Weighted, 3, time.Minute))
	assert.Number(t, chain.Generation()).Equal(uint64(2))
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.Weighted)
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(3)
}

// TestCenter_Validate pins the dry-run contract: [Center.Validate] answers the
// same question dispatch's pre-check does — could this document take effect? —
// while touching nothing. A caller uses it to reject a bad document before it is
// ever pushed.
func TestCenter_Validate(t *testing.T) {
	c := newTestCenter(Config{})

	if err := c.Validate(Config{}); err != nil {
		t.Fatalf("an empty document configures nothing, so it must validate: %v", err)
	}
	if err := c.Validate(timeoutConfig(300)); err != nil {
		t.Fatalf("a well-formed document must validate: %v", err)
	}

	// Each rejection dispatch would also make, reported here without a push.
	dup := Config{Client: ClientConfig{Rules: []ClientRule{
		{Service: "redis:cache"}, {Service: "redis:cache"},
	}}}
	assert.Error(t, c.Validate(dup)).Matches(`duplicate rule for service "redis:cache"`)
	assert.Error(t, c.Validate(selectionConfig("redis:cache", "no_such_strategy", 0, 0))).
		Matches("no strategy registered")

	// A pure read: none of the calls above may have armed or configured anything.
	if p := c.resClient.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("Validate must not push a resilience policy: %+v", p)
	}
	if !c.injClient.Config().Inert() {
		t.Fatal("Validate must not touch the fault injector")
	}
}

// TestSelection_UnknownBalancerRejectsDocument pins the control-plane rule for a
// strategy name that resolves against nothing: the document is rejected WHOLE,
// exactly like a bad driver name — the chain keeps its last good generation (as
// before), AND the rest of the rejected document must not land either.
//
// The per-chain degrade in loadbalance.Manager.Apply is still there for callers
// that drive the manager directly; going through the center, a bad name is caught
// in the pre-check and never reaches a chain at all.
func TestSelection_UnknownBalancerRejectsDocument(t *testing.T) {
	const label = "redis:cache"
	good := selectionConfig(label, loadbalance.LeastConn, 5, 0)
	good.Client.Rules[0].Timeout = dur(100)
	c := newTestCenter(good)
	chain := selectionChain(c.lb, label)
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.LeastConn)

	// A document that is bad in ONE place (the strategy name) but would otherwise
	// change both halves: the selection AND the protection policy.
	bad := selectionConfig(label, "no_such_strategy", 9, 0)
	bad.Client.Rules[0].Timeout = dur(777)
	if err := c.dispatch(context.Background(), bad); err == nil {
		t.Fatal("an unregistered strategy name must fail dispatch")
	}

	// The chain's last good generation is untouched, and the protection half of
	// the rejected document did not sneak in beside it.
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, chain.Selection().OutlierThreshold).Equal(5)
	if p := c.resClient.ClientPolicyFor(label); p.Timeout != dur(100) {
		t.Fatalf("a rejected document must not land its other half either: got %v", p.Timeout)
	}
}

// TestSelection_NoRuleIsPassThrough covers the process that imports the
// starter but configures no rule for the label: the chain is built ungoverned —
// no override, no suspension — instead of being armed with a partial
// policy. An unmatched label resolving the (zero) Default IS "governance off".
func TestSelection_NoRuleIsPassThrough(t *testing.T) {
	const label = "gorm:mysql:orders-db"
	// A document that configures some OTHER label, none for this one.
	cfg := selectionConfig("some:other-db", loadbalance.LeastConn, 5, time.Second)
	c := newTestCenter(cfg)
	chain := selectionChain(c.lb, label)

	assert.That(t, chain.Selection().Balancer).Equal("")
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(0)
}

// TestCenter_ConstructsArmedSelection covers the wiring this whole feature hangs
// on: construction distributes the snapshot into the selection manager, so a
// client starter that only binds its pool — without naming cloud/governance —
// gets its policy. It also pins the label contract: one rule drives both the
// protection executor and endpoint selection, because both are addressed by the
// same service label.
func TestCenter_ConstructsArmedSelection(t *testing.T) {
	const label = "grpc:client"
	resClient, lb, fc, fs := newAuthorities()
	if _, err := NewCenter(resClient, lb, fc, fs, nil,
		NewPushSource(selectionConfig(label, loadbalance.P2C, 4, 2*time.Second))); err != nil {
		t.Fatal(err)
	}

	chain := selectionChain(lb, label)

	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.P2C)
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(4)
}

// TestDiscovery_HandsOutTheDirectory pins the center's fourth hole: it carries
// the discovery directory and returns exactly the manager it was built with, so
// a client reaches discovery through the same injection point as the policy
// authorities.
func TestDiscovery_HandsOutTheDirectory(t *testing.T) {
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "10.0.0.1:2379"})
	disc := discovery.NewManager(map[string]discovery.Discovery{"etcd.main": backend})

	resClient, lb, fc, fs := newAuthorities()
	c, err := NewCenter(resClient, lb, fc, fs, disc, NewPushSource(Config{}))
	if err != nil {
		t.Fatal(err)
	}
	assert.That(t, c.Discovery()).Same(disc)

	got, ok := c.Discovery().Get("etcd.main")
	assert.That(t, ok).True()
	assert.That(t, got).Same(backend)
}
