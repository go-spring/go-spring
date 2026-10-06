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

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// dur is a shorthand for declaring timeouts in test policies.
func dur(d int) time.Duration { return time.Duration(d) * time.Millisecond }

// enabledTimeout returns a Config whose client Default policy sets a per-attempt
// timeout, the knob the center exists to centralize.
func enabledTimeout(d int) Config {
	return Config{
		Enabled: true,
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(d)}},
		},
	}
}

// newAuthorities returns the three module authorities a center distributes to.
// The wiring starter creates them as beans; a test creates them per center so no
// state leaks between tests. drivers is the resilience driver directory the
// managers are built over — the stand-in for the container's bean collection, so
// a config naming a non-bundled backend resolves.
func newAuthorities(drivers map[string]resilience.Driver) (*resilience.Manager, *loadbalance.Manager, *fault.Injector) {
	lb, err := loadbalance.NewManager(nil) // no factories contributed
	if err != nil {
		panic(err)
	}
	return resilience.NewManager(drivers), lb, fault.NewInjector(fault.Configs{}, nil)
}

// newTestCenterWith builds a center over fresh module authorities and dispatches
// cfg into them, exactly as the wiring starter does in production.
func newTestCenterWith(cfg Config, drivers map[string]resilience.Driver) (*Center, *resilience.Manager, *loadbalance.Manager, *fault.Injector) {
	res, lb, inj := newAuthorities(drivers)
	c := NewCenter(cfg, res, lb, inj, nil, nil)
	if err := c.dispatch(cfg); err != nil {
		panic(err)
	}
	return c, res, lb, inj
}

// newTestCenter is newTestCenterWith for the common case of the bundled driver.
func newTestCenter(cfg Config) *Center {
	c, _, _, _ := newTestCenterWith(cfg, nil)
	return c
}

func TestPolicyFor_Default(t *testing.T) {
	c := newTestCenter(enabledTimeout(100))
	if p := c.clientPolicyFor("redis:cache"); p.AttemptTimeout != dur(100) {
		t.Fatalf("policyFor default: want timeout 100ms, got %v", p.AttemptTimeout)
	}
	// Unknown label also falls back to Default.
	if p := c.clientPolicyFor("anything:else"); p.AttemptTimeout != dur(100) {
		t.Fatalf("policyFor fallback: want timeout 100ms, got %v", p.AttemptTimeout)
	}
}

func TestPolicyFor_RuleReplacesDefault(t *testing.T) {
	c := newTestCenter(Config{
		Enabled: true,
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(100), MaxRetries: 1}},
			Rules: []ClientRule{{
				Service:      "redis:cache",
				ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(50)}, // no MaxRetries
			}},
		},
	})
	p := c.clientPolicyFor("redis:cache")
	if p.AttemptTimeout != dur(50) {
		t.Fatalf("rule timeout: want 50ms, got %v", p.AttemptTimeout)
	}
	// A Rule fully replaces Default, so MaxRetries from Default does NOT carry
	// over — that is the documented "complete, self-contained policy" semantic.
	if p.MaxRetries != 0 {
		t.Fatalf("rule must replace not merge: want MaxRetries 0, got %d", p.MaxRetries)
	}
	// A label no Rule matches still gets Default.
	if p := c.clientPolicyFor("redis:other"); p.AttemptTimeout != dur(100) {
		t.Fatalf("unmatched label: want default 100ms, got %v", p.AttemptTimeout)
	}
}

func TestPolicyFor_EmptyServiceMatchesNothing(t *testing.T) {
	// A Rule with an empty Service matches nothing (use Default instead).
	c := newTestCenter(Config{
		Enabled: true,
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(100)}},
			Rules:   []ClientRule{{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(50)}}},
		},
	})
	if p := c.clientPolicyFor("redis:cache"); p.AttemptTimeout != dur(100) {
		t.Fatalf("empty-Service rule must not match: want default 100ms, got %v", p.AttemptTimeout)
	}
}

func TestServerPolicyFor_ServerBlock(t *testing.T) {
	c := newTestCenter(Config{
		Enabled: true,
		Server: ServerConfig{
			Default: resilience.ServerPolicy{AttemptTimeout: dur(100)},
			Rules: []ServerRule{{
				Service:      "gin::8080",
				ServerPolicy: resilience.ServerPolicy{RateLimit: 500},
			}},
		},
	})
	// A matched inbound rule fully replaces the server default.
	if a := c.serverPolicyFor("gin::8080"); a.RateLimit != 500 || a.AttemptTimeout != 0 {
		t.Fatalf("inbound rule must replace the default wholesale: %+v", a)
	}
	if a := c.serverPolicyFor("echo::9090"); a.AttemptTimeout != dur(100) {
		t.Fatalf("unmatched route: want the server default, got %+v", a)
	}
	// The two directions are separate resolutions over separate rules: the client
	// half of the same document holds nothing for these labels.
	if p := c.clientPolicyFor("gin::8080"); !p.IsZero() {
		t.Fatalf("inbound must not leak into the outbound policy: %+v", p)
	}
}

func TestServerPolicyFor_DisabledIsZero(t *testing.T) {
	c := newTestCenter(Config{
		Enabled: false,
		Server:  ServerConfig{Default: resilience.ServerPolicy{RateLimit: 10, MaxConcurrent: 4}},
	})
	if a := c.serverPolicyFor("gin::8080"); !a.IsZero() {
		t.Fatalf("disabled center must yield a zero inbound, got %+v", a)
	}
}

// TestDispatch_RejectsDuplicateServerRules is the server-side twin of the
// duplicate-label guard: an inbound list naming the same route twice is rejected
// whole, so which entry wins can never depend on list order.
func TestDispatch_RejectsDuplicateServerRules(t *testing.T) {
	c := newTestCenter(Config{
		Enabled: true,
		Server: ServerConfig{
			Rules: []ServerRule{{Service: "gin::8080", ServerPolicy: resilience.ServerPolicy{AttemptTimeout: dur(50)}}},
		},
	})
	bad := Config{
		Enabled: true,
		Server: ServerConfig{
			Rules: []ServerRule{
				{Service: "gin::8080", ServerPolicy: resilience.ServerPolicy{RateLimit: 1}},
				{Service: "gin::8080", ServerPolicy: resilience.ServerPolicy{RateLimit: 999}},
			},
		},
	}
	if err := c.dispatch(bad); err == nil {
		t.Fatal("dispatch must reject duplicate INBOUND service labels")
	}
	if a := c.serverPolicyFor("gin::8080"); a.AttemptTimeout != dur(50) {
		t.Fatalf("rejected push must not leak into the serving snapshot: got %+v", a)
	}
}

func TestPolicyFor_DisabledIsPassThrough(t *testing.T) {
	// Enabled=false makes the center a no-op even when the client Default is set.
	c := newTestCenter(Config{
		Enabled: false,
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(100)}},
		},
	})
	if p := c.clientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("disabled center must yield zero policy, got %+v", p)
	}
	if c.Enabled() {
		t.Fatal("enabled() should be false")
	}
	// A disabled center leaves the module unarmed, so a subscription is still
	// registered — a later enable must be able to notify it — but it is armed
	// with a ZERO policy, which is a transparent pass-through.
	var got resilience.ClientPolicy
	s := c.res.Subscribe("redis:cache", func(p resilience.ClientPolicy) { got = p })
	if !got.IsZero() || !s.ClientPolicy.IsZero() {
		t.Fatalf("disabled center must arm with a zero policy, got %+v", got)
	}
}

func TestSubscribe_ArmsImmediately(t *testing.T) {
	c := newTestCenter(enabledTimeout(100))
	var got resilience.ClientPolicy
	c.res.Subscribe("redis:cache", func(p resilience.ClientPolicy) { got = p })
	if got.AttemptTimeout != dur(100) {
		t.Fatalf("Subscribe should arm cb with current policy, got timeout %v", got.AttemptTimeout)
	}
}

// Adopt must notify only the labels whose resolved policy actually changed, so a
// localized rule edit does not churn unrelated executors — and a no-op adopt
// notifies nobody.
func TestAdopt_NotifiesOnlyChangedLabels(t *testing.T) {
	c := newTestCenter(Config{
		Enabled: true,
		Driver:  "default",
		Client: ClientConfig{
			Default: ClientDefaultPolicy{ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(100)}},
		},
	})

	var redisN, gormN int
	c.res.Subscribe("redis:cache", func(resilience.ClientPolicy) { redisN++ })
	c.res.Subscribe("gorm:mysql:primary", func(resilience.ClientPolicy) { gormN++ })
	// Subscribe arms once each.
	if redisN != 1 || gormN != 1 {
		t.Fatalf("after subscribe: redis=%d gorm=%d, want 1/1", redisN, gormN)
	}

	// Change only redis:cache. gorm must NOT be notified (its policy unchanged).
	cfg := enabledTimeout(100)
	cfg.Client.Rules = []ClientRule{{
		Service:      "redis:cache",
		ClientPolicy: resilience.ClientPolicy{AttemptTimeout: dur(200)},
	}}
	c.adopt(context.Background(), cfg)
	if redisN != 2 {
		t.Fatalf("redis must be notified on its policy change: got %d, want 2", redisN)
	}
	if gormN != 1 {
		t.Fatalf("gorm must NOT be notified when its policy is unchanged: got %d, want 1", gormN)
	}
	if p := c.clientPolicyFor("redis:cache"); p.AttemptTimeout != dur(200) {
		t.Fatalf("post-adopt redis policy: want 200ms, got %v", p.AttemptTimeout)
	}

	// A no-op adopt (same config) notifies nobody.
	c.adopt(context.Background(), cfg)
	if redisN != 2 || gormN != 1 {
		t.Fatalf("no-op adopt must not notify: redis=%d gorm=%d, want 2/1", redisN, gormN)
	}
}

func TestAdopt_DefaultChangeFansOutToAllUnoverridden(t *testing.T) {
	c := newTestCenter(enabledTimeout(100))
	var a, b int
	c.res.Subscribe("redis:cache", func(resilience.ClientPolicy) { a++ })
	c.res.Subscribe("gorm:mysql:primary", func(resilience.ClientPolicy) { b++ })
	// Both read Default; changing Default must notify both.
	c.adopt(context.Background(), enabledTimeout(300))
	if a != 2 || b != 2 {
		t.Fatalf("default change should fan out to both: a=%d b=%d, want 2/2", a, b)
	}
	if p := c.clientPolicyFor("redis:cache"); p.AttemptTimeout != dur(300) {
		t.Fatalf("post default-change: want 300ms, got %v", p.AttemptTimeout)
	}
}

func TestDriver(t *testing.T) {
	// A configured name resolves against the injected driver directory.
	drivers := map[string]resilience.Driver{"sentinel": resilience.NewDefaultDriver(nil)}
	_, res, _, _ := newTestCenterWith(Config{Enabled: true, Driver: "sentinel"}, drivers)
	if d := res.Driver(); d != "sentinel" {
		t.Fatalf("Driver: want sentinel, got %s", d)
	}
	// Defaults to "default" when unset.
	_, res2, _, _ := newTestCenterWith(Config{Enabled: true}, nil)
	if d := res2.Driver(); d != "default" {
		t.Fatalf("Driver default: want default, got %s", d)
	}
	// An enabled config naming an uninstalled driver is a wiring error, and it
	// must not disturb the center: the config is still adopted, and services
	// fall back to a pass-through.
	c3, _, _, _ := newTestCenterWith(Config{}, nil)
	err := c3.dispatch(Config{Enabled: true, Driver: "nope"})
	if err == nil {
		t.Fatal("an uninstalled driver name must fail dispatch")
	}
	if p := c3.clientPolicyFor("x"); !p.IsZero() {
		t.Fatalf("the config must still be adopted, got %+v", p)
	}
}

// TestSource_AdoptSwapsFaultConfig guards the single-sink property of adopt:
// a config pushed from ANY source swaps BOTH the resilience snapshot and the
// fault injector's config (the injector pointer itself is never rebuilt, which
// is what lets clients hold it for the process lifetime).
func TestSource_AdoptSwapsFaultConfig(t *testing.T) {
	c, _, _, inj := newTestCenterWith(Config{}, nil)
	if inj.ClientConfig().Enabled || inj.ServerConfig().Enabled {
		t.Fatal("precondition: injector should start disabled")
	}

	cfg := enabledTimeout(100)
	cfg.Client.Fault = fault.Config{Enabled: true, Rate: 0.5}
	cfg.Server.Fault = fault.Config{Enabled: true, Rate: 1, Latency: dur(5)}
	c.adopt(context.Background(), cfg)

	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(100) {
		t.Fatalf("adopt resilience side: want 100ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.5 {
		t.Fatalf("adopt client fault side: injector config not swapped: %+v", cf)
	}
	// The two sides are pushed independently: the same adopt must have landed the
	// server-side config too, and neither may bleed into the other.
	if sf := inj.ServerConfig(); !sf.Enabled || sf.Rate != 1 || sf.Latency != dur(5) {
		t.Fatalf("adopt server fault side: injector config not swapped: %+v", sf)
	}
}

// TestSetSource_LateArm_StaleGuard covers source replacement after one is
// already bound: the new source's snapshot applies immediately, and callbacks
// from the replaced source are dropped (a source cannot always be unsubscribed,
// so stale callbacks must no-op instead of being retracted).
func TestSetSource_LateArm_StaleGuard(t *testing.T) {
	pushA := NewPushSource(enabledTimeout(100))
	c := newTestCenter(Config{})
	c.SetSource(pushA)
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(100) {
		t.Fatalf("source A snapshot: want 100ms, got %v", p.AttemptTimeout)
	}

	pushB := NewPushSource(enabledTimeout(200))
	c.SetSource(pushB)
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(200) {
		t.Fatalf("source B snapshot should replace A: want 200ms, got %v", p.AttemptTimeout)
	}

	// A's pushes are stale now; B's still drive the center.
	pushA.Push(context.Background(), enabledTimeout(999))
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(200) {
		t.Fatalf("stale source A push must be dropped: want 200ms, got %v", p.AttemptTimeout)
	}
	pushB.Push(context.Background(), enabledTimeout(300))
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(300) {
		t.Fatalf("source B push should apply: want 300ms, got %v", p.AttemptTimeout)
	}
}

// TestSetSource_NilPanics pins the contract: there is no "remove the source"
// operation, only replacement. "No source" is a start-of-life state — the
// nullable constructor parameter — settled once and never revisited; a nil
// argument to SetSource is therefore a programming error, not a mode.
func TestSetSource_NilPanics(t *testing.T) {
	c, _, _, _ := newTestCenterWith(Config{}, nil)
	defer func() {
		if recover() == nil {
			t.Fatal("SetSource(nil) should panic")
		}
	}()
	c.SetSource(nil)
}

// TestSetSource_ReplacesTheConstructorSource pins the source rule that replaced
// the old default-vs-explicit dance: the source handed to NewCenter is bound at
// construction (the container's contribution), and any later SetSource replaces
// it outright — last write wins, no arbitration.
func TestSetSource_ReplacesTheConstructorSource(t *testing.T) {
	res, lb, inj := newAuthorities(nil)
	c := NewCenter(Config{}, res, lb, inj, nil, NewPushSource(enabledTimeout(300)))
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(300) {
		t.Fatalf("the constructor's source must be bound at construction: want 300ms, got %v", p.AttemptTimeout)
	}

	c.SetSource(NewPushSource(enabledTimeout(100)))
	if p := c.clientPolicyFor("x"); p.AttemptTimeout != dur(100) {
		t.Fatalf("SetSource must replace the constructor's source: want 100ms, got %v", p.AttemptTimeout)
	}
}

// TestGoLive_DistributesSnapshot covers the starter-facing completion hook:
// GoLive distributes the CURRENT snapshot to the module authorities — including
// the fault injector — and fires OnReady.
func TestGoLive_DistributesSnapshot(t *testing.T) {
	c, _, _, inj := newTestCenterWith(Config{}, nil)

	c.SetSource(NewPushSource(Config{
		Enabled: true,
		Client:  ClientConfig{Fault: fault.Config{Enabled: true, Rate: 0.25}},
	}))
	if err := c.GoLive(); err != nil {
		t.Fatal(err)
	}

	ready := false
	c.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the center live")
	}
	if !c.Live() {
		t.Fatal("Live() should report the center armed")
	}
	if !c.Enabled() {
		t.Fatal("GoLive should keep the bound source armed")
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.25 {
		t.Fatal("GoLive should push the snapshot's client Fault into the injected injector")
	}

	// Idempotent: a second GoLive re-dispatches nothing and does not disturb the
	// armed state.
	if err := c.GoLive(); err != nil {
		t.Fatal(err)
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.25 {
		t.Fatalf("second GoLive must be a no-op, got %+v", cf)
	}
}

// selectionPool builds a pool over a one-endpoint static source — enough for the
// distribution tests, which assert the policy that reaches the pool rather than a
// routing decision. It mirrors what a client starter builds: a suspension tracker
// attached so the thresholds have somewhere to land.
func selectionPool() *loadbalance.Pool {
	src := func() ([]discovery.Endpoint, error) {
		return []discovery.Endpoint{{Addr: "10.0.0.1:8080", Healthy: true, Weight: 1}}, nil
	}
	bal := loadbalance.NewRoundRobin()
	return loadbalance.NewPool(src, bal)
}

// selectionConfig returns a Config with one rule for label naming a strategy and
// an outlier-suspension policy. It sets only the selection half — the protection
// half stays zero, which is what proves the two halves are independent.
func selectionConfig(label, balancer string, threshold int, suspendFor time.Duration) Config {
	return Config{
		Enabled: true,
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

// TestSelectionFor_AppliesCurrentThenChanges is the end-to-end control-plane test:
// the label's rule reaches the pool at bind time (before the first Pick, not only
// on the next push), and a later config change reaches it again — which is what
// makes endpoint selection a hot-reloadable knob rather than a construction-time
// constant.
func TestSelectionFor_AppliesCurrentThenChanges(t *testing.T) {
	const label = "http:user-svc"
	c := newTestCenter(selectionConfig(label, loadbalance.LeastConn, 5, 10*time.Second))
	pool := selectionPool()

	stop := c.lb.Bind(pool, label)
	defer stop()

	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, pool.Selection().OutlierThreshold).Equal(5)
	assert.That(t, pool.Selection().OutlierSuspendFor).Equal(10 * time.Second)
	// The suspension half landed on the tracker too, not just on the record.
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(5)

	// A pushed change reaches the live pool in place.
	c.adopt(context.Background(), selectionConfig(label, loadbalance.Weighted, 3, time.Minute))
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.Weighted)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(3)
}

// TestSelectionFor_UnknownBalancerKeepsLastGood pins the degrade-don't-fail rule
// at the control-plane level: a rule naming a strategy that is not registered
// leaves the last good strategy in force instead of taking the client down.
func TestSelectionFor_UnknownBalancerKeepsLastGood(t *testing.T) {
	const label = "redis:cache"
	c := newTestCenter(selectionConfig(label, loadbalance.LeastConn, 0, 0))
	pool := selectionPool()

	stop := c.lb.Bind(pool, label)
	defer stop()
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)

	c.adopt(context.Background(), selectionConfig(label, "no_such_strategy", 0, 0))
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
}

// TestSelectionFor_DisabledIsPassThrough covers the process that imports the
// starter but has governance turned off: the pool is left exactly as built — no
// override, no suspension — instead of being armed with a partial policy.
func TestSelectionFor_DisabledIsPassThrough(t *testing.T) {
	const label = "gorm:mysql:orders-db"
	cfg := selectionConfig(label, loadbalance.LeastConn, 5, time.Second)
	cfg.Enabled = false
	c := newTestCenter(cfg)
	pool := selectionPool()

	stop := c.lb.Bind(pool, label)
	defer stop()

	assert.That(t, pool.Selection().Balancer).Equal("")
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(0)
}

// TestSelectionFor_StopDetaches covers the teardown half: a pool that goes away
// must be able to detach, and detaching twice must be harmless.
func TestSelectionFor_StopDetaches(t *testing.T) {
	const label = "mongodb:orders"
	c := newTestCenter(selectionConfig(label, loadbalance.LeastConn, 0, 0))
	pool := selectionPool()

	stop := c.lb.Bind(pool, label)
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)

	stop()
	stop()

	c.adopt(context.Background(), selectionConfig(label, loadbalance.Weighted, 0, 0))
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
}

// TestGoLive_ArmsSelection covers the wiring this whole feature hangs on: GoLive
// distributes the snapshot into the selection manager, so a client starter that
// only binds its pool — without naming cloud/governance — gets its policy. It
// also pins the label contract: one rule drives both the protection executor and
// endpoint selection, because both are addressed by the same service label.
func TestGoLive_ArmsSelection(t *testing.T) {
	const label = "grpc:client"
	c, _, lb, _ := newTestCenterWith(Config{}, nil)

	c.SetSource(NewPushSource(selectionConfig(label, loadbalance.P2C, 4, 2*time.Second)))
	if err := c.GoLive(); err != nil {
		t.Fatal(err)
	}

	pool := selectionPool()
	stop := lb.Bind(pool, label)
	defer stop()

	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.P2C)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(4)
}

// TestDiscovery_HandsOutTheDirectory pins the center's fourth hole: it carries
// the discovery directory and returns exactly the manager it was built with, so
// a client reaches discovery through the same injection point as the policy
// authorities.
func TestDiscovery_HandsOutTheDirectory(t *testing.T) {
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "10.0.0.1:2379"})
	disc := discovery.NewManager(map[string]discovery.Discovery{"etcd.main": backend})

	res, lb, inj := newAuthorities(nil)
	c := NewCenter(Config{}, res, lb, inj, disc, nil)
	assert.That(t, c.Discovery()).Same(disc)

	got, ok := c.Discovery().Get("etcd.main")
	assert.That(t, ok).True()
	assert.That(t, got).Same(backend)
}
