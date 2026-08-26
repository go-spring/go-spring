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
	"testing"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// dur is a shorthand for declaring timeouts in test policies.
func dur(d int) time.Duration { return time.Duration(d) * time.Millisecond }

// enabledTimeout returns a Config whose Default policy sets a per-attempt
// timeout, the knob the center exists to centralize.
func enabledTimeout(d int) Config {
	return Config{
		Enabled: true,
		Default: resilience.PolicyConfig{AttemptTimeout: dur(d)},
	}
}

// TestConfig_FaultEmbedding guards that Config carries the centralized fault
// config as an embedded field, and that a Config built with Fault set both
// preserves it (so Center.Init can read cfg.Fault and build the injector) and
// does not disturb resilience PolicyFor. This is the structural anchor of fault
// centralization (DESIGN_CN.md §8).
func TestConfig_FaultEmbedding(t *testing.T) {
	// Zero Config: fault disabled (transparent), resilience disabled.
	var zero Config
	if zero.Fault.Enabled {
		t.Fatal("zero Config.Fault should be disabled")
	}
	c := NewCenter(zero)
	if p := c.PolicyFor("x"); p.Timeout != 0 {
		t.Fatalf("zero config PolicyFor: want no timeout, got %v", p.Timeout)
	}

	// Config carrying an armed fault: Fault is preserved on the Config (Center.Init
	// reads cfg.Fault directly to build the injector) and resilience PolicyFor still
	// resolves from Default/Rules unaffected.
	cfg := Config{
		Enabled: true,
		Default: resilience.PolicyConfig{AttemptTimeout: dur(100)},
		Fault:   fault.Config{Enabled: true, Rate: 0.5, Error: "generic"},
	}
	if !cfg.Fault.Enabled || cfg.Fault.Rate != 0.5 || cfg.Fault.Error != "generic" {
		t.Fatalf("Config.Fault not preserved: %+v", cfg.Fault)
	}
	cc := NewCenter(cfg)
	if p := cc.PolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("PolicyFor with Fault set: want timeout 100ms, got %v", p.Timeout)
	}
}

func TestPolicyFor_Default(t *testing.T) {
	c := NewCenter(enabledTimeout(100))
	if p := c.PolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("PolicyFor default: want timeout 100ms, got %v", p.Timeout)
	}
	// Unknown label also falls back to Default.
	if p := c.PolicyFor("anything:else"); p.Timeout != dur(100) {
		t.Fatalf("PolicyFor fallback: want timeout 100ms, got %v", p.Timeout)
	}
}

func TestPolicyFor_RuleReplacesDefault(t *testing.T) {
	c := NewCenter(Config{
		Enabled: true,
		Default: resilience.PolicyConfig{AttemptTimeout: dur(100), MaxRetries: 1},
		Rules: []Rule{{
			Resources:          []string{"redis:cache"},
			PolicyConfig: resilience.PolicyConfig{AttemptTimeout: dur(50)}, // no MaxRetries
		}},
	})
	p := c.PolicyFor("redis:cache")
	if p.Timeout != dur(50) {
		t.Fatalf("rule timeout: want 50ms, got %v", p.Timeout)
	}
	// A Rule fully replaces Default, so MaxRetries from Default does NOT carry
	// over — that is the documented "complete, self-contained policy" semantic.
	if p.MaxRetries != 0 {
		t.Fatalf("rule must replace not merge: want MaxRetries 0, got %d", p.MaxRetries)
	}
	// A label no Rule matches still gets Default.
	if p := c.PolicyFor("redis:other"); p.Timeout != dur(100) {
		t.Fatalf("unmatched label: want default 100ms, got %v", p.Timeout)
	}
}

func TestPolicyFor_FirstMatchingRuleWins(t *testing.T) {
	// When two Rules match the same label, the earlier one wins — so list
	// specific Rules before broad ones.
	c := NewCenter(Config{
		Enabled: true,
		Rules: []Rule{
			{Resources: []string{"redis:cache"}, PolicyConfig: resilience.PolicyConfig{AttemptTimeout: dur(10)}},
			{Resources: []string{"redis:cache"}, PolicyConfig: resilience.PolicyConfig{AttemptTimeout: dur(20)}},
		},
	})
	if p := c.PolicyFor("redis:cache"); p.Timeout != dur(10) {
		t.Fatalf("first matching rule should win: want 10ms, got %v", p.Timeout)
	}
	// A Rule with empty Resources matches nothing (use Default instead).
	c2 := NewCenter(Config{
		Enabled: true,
		Default: resilience.PolicyConfig{AttemptTimeout: dur(100)},
		Rules:   []Rule{{PolicyConfig: resilience.PolicyConfig{AttemptTimeout: dur(50)}}},
	})
	if p := c2.PolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("empty-Resources rule must not match: want default 100ms, got %v", p.Timeout)
	}
}

func TestPolicyFor_DisabledIsPassThrough(t *testing.T) {
	// Enabled=false makes the center a no-op even when Default is set.
	c := NewCenter(Config{Enabled: false, Default: resilience.PolicyConfig{AttemptTimeout: dur(100)}})
	if p := c.PolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("disabled center must yield zero policy, got %+v", p)
	}
	if c.Enabled() {
		t.Fatal("Enabled() should be false")
	}
}

func TestRegister_ArmsImmediately(t *testing.T) {
	c := NewCenter(enabledTimeout(100))
	var got resilience.Policy
	c.Register("redis:cache", func(p resilience.Policy) { got = p })
	if got.Timeout != dur(100) {
		t.Fatalf("Register should arm cb with current policy, got timeout %v", got.Timeout)
	}
}

func TestRefresh_NotifiesOnlyChangedLabels(t *testing.T) {
	c := NewCenter(Config{
		Enabled: true,
		Driver:  "default",
		Default: resilience.PolicyConfig{AttemptTimeout: dur(100)},
	})

	var redisN, gormN int
	c.Register("redis:cache", func(p resilience.Policy) { redisN++ })
	c.Register("gorm:mysql:primary", func(p resilience.Policy) { gormN++ })
	// Register arms once each.
	if redisN != 1 || gormN != 1 {
		t.Fatalf("after register: redis=%d gorm=%d, want 1/1", redisN, gormN)
	}

	// Change only redis:cache. gorm must NOT be notified (its policy unchanged).
	cfg := enabledTimeout(100)
	cfg.Rules = []Rule{{
		Resources:          []string{"redis:cache"},
		PolicyConfig: resilience.PolicyConfig{AttemptTimeout: dur(200)},
	}}
	c.Refresh(cfg)
	if redisN != 2 {
		t.Fatalf("redis must be notified on its policy change: got %d, want 2", redisN)
	}
	if gormN != 1 {
		t.Fatalf("gorm must NOT be notified when its policy is unchanged: got %d, want 1", gormN)
	}
	if p := c.PolicyFor("redis:cache"); p.Timeout != dur(200) {
		t.Fatalf("post-refresh redis policy: want 200ms, got %v", p.Timeout)
	}

	// A no-op refresh (same config) notifies nobody.
	c.Refresh(cfg)
	if redisN != 2 || gormN != 1 {
		t.Fatalf("no-op refresh must not notify: redis=%d gorm=%d, want 2/1", redisN, gormN)
	}
}

func TestRefresh_DefaultChangeFansOutToAllUnoverridden(t *testing.T) {
	c := NewCenter(enabledTimeout(100))
	var a, b int
	c.Register("redis:cache", func(p resilience.Policy) { a++ })
	c.Register("gorm:mysql:primary", func(p resilience.Policy) { b++ })
	// Both read Default; changing Default must notify both.
	c.Refresh(enabledTimeout(300))
	if a != 2 || b != 2 {
		t.Fatalf("default change should fan out to both: a=%d b=%d, want 2/2", a, b)
	}
	if p := c.PolicyFor("redis:cache"); p.Timeout != dur(300) {
		t.Fatalf("post default-change: want 300ms, got %v", p.Timeout)
	}
}

func TestDriver(t *testing.T) {
	c := NewCenter(Config{Enabled: true, Driver: "sentinel"})
	if d := c.Driver(); d != "sentinel" {
		t.Fatalf("Driver: want sentinel, got %s", d)
	}
	// Defaults to "default" when unset.
	c2 := NewCenter(Config{Enabled: true})
	if d := c2.Driver(); d != "default" {
		t.Fatalf("Driver default: want default, got %s", d)
	}
}

// TestSource_PushSourceDrivesCenter covers the custom-source end-to-end path:
// a PushSource installed via the SetSource facade arms the center from its
// snapshot and later pushes fan out to registered subscribers — the same
// fan-out the default source drives on the plain-${govern} path.
func TestSource_PushSourceDrivesCenter(t *testing.T) {
	defer Reset()

	src := NewPushSource(enabledTimeout(100))
	SetSource(src)
	if !Enabled() {
		t.Fatal("PushSource snapshot should arm the center")
	}
	if p := PolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("snapshot policy: want 100ms, got %v", p.Timeout)
	}

	var got resilience.Policy
	Register("redis:cache", func(p resilience.Policy) { got = p })
	src.Push(enabledTimeout(200))
	if got.Timeout != dur(200) {
		t.Fatalf("push should fan out: want 200ms, got %v", got.Timeout)
	}
	if p := PolicyFor("redis:cache"); p.Timeout != dur(200) {
		t.Fatalf("post-push policy: want 200ms, got %v", p.Timeout)
	}
}

// TestSource_AdoptSwapsFaultConfig guards the single-sink property of adopt:
// a config pushed from ANY source swaps both the resilience snapshot and the
// fault injector's config (the injector pointer itself is never rebuilt).
func TestSource_AdoptSwapsFaultConfig(t *testing.T) {
	c := NewCenter(Config{})
	c.injector = fault.NewInjector(fault.Config{})
	if c.injector.Config().Enabled {
		t.Fatal("precondition: injector should start disabled")
	}

	cfg := enabledTimeout(100)
	cfg.Fault = fault.Config{Enabled: true, Rate: 0.5}
	c.adopt(cfg)

	if p := c.PolicyFor("x"); p.Timeout != dur(100) {
		t.Fatalf("adopt resilience side: want 100ms, got %v", p.Timeout)
	}
	if !c.injector.Config().Enabled || c.injector.Config().Rate != 0.5 {
		t.Fatalf("adopt fault side: injector config not swapped: %+v", c.injector.Config())
	}

	// adopt with a nil injector (pre-Init center) must not panic.
	c2 := NewCenter(Config{})
	c2.adopt(enabledTimeout(50))
	if p := c2.PolicyFor("x"); p.Timeout != dur(50) {
		t.Fatalf("adopt without injector: want 50ms, got %v", p.Timeout)
	}
}

// TestSetSource_LateArm_StaleGuard covers source replacement after one is
// already bound: the new source's snapshot applies immediately, and callbacks
// from the replaced source are dropped (gs.Dync.OnChanged cannot unsubscribe,
// so stale callbacks must no-op instead of being retracted).
func TestSetSource_LateArm_StaleGuard(t *testing.T) {
	pushA := NewPushSource(enabledTimeout(100))
	c := NewCenter(Config{})
	c.setSource(pushA)
	if p := c.PolicyFor("x"); p.Timeout != dur(100) {
		t.Fatalf("source A snapshot: want 100ms, got %v", p.Timeout)
	}

	pushB := NewPushSource(enabledTimeout(200))
	c.setSource(pushB)
	if p := c.PolicyFor("x"); p.Timeout != dur(200) {
		t.Fatalf("source B snapshot should replace A: want 200ms, got %v", p.Timeout)
	}

	// A's pushes are stale now; B's still drive the center.
	pushA.Push(enabledTimeout(999))
	if p := c.PolicyFor("x"); p.Timeout != dur(200) {
		t.Fatalf("stale source A push must be dropped: want 200ms, got %v", p.Timeout)
	}
	pushB.Push(enabledTimeout(300))
	if p := c.PolicyFor("x"); p.Timeout != dur(300) {
		t.Fatalf("source B push should apply: want 300ms, got %v", p.Timeout)
	}
}

// TestSetSource_NilPanics pins the contract: there is no "remove the source"
// operation, only replacement (removal would resurrect the wiring default
// half-armed, which the guard machinery deliberately cannot do).
func TestSetSource_NilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("SetSource(nil) should panic")
		}
	}()
	SetSource(nil)
}

// TestBindDefault_RespectsExplicitSource pins the priority rule: BindDefault
// installs its source only when none is bound — an earlier explicit SetSource
// always wins, which is what lets a pre-wiring SetSource take over the default.
func TestBindDefault_RespectsExplicitSource(t *testing.T) {
	c := NewCenter(Config{})
	explicit := NewPushSource(enabledTimeout(100))
	c.setSource(explicit)

	c.bindDefault(NewPushSource(enabledTimeout(999))) // must be ignored
	if p := c.PolicyFor("x"); p.Timeout != dur(100) {
		t.Fatalf("BindDefault must not override an explicit source: want 100ms, got %v", p.Timeout)
	}

	// With nothing bound, BindDefault takes effect.
	c2 := NewCenter(Config{})
	c2.bindDefault(NewPushSource(enabledTimeout(200)))
	if p := c2.PolicyFor("x"); p.Timeout != dur(200) {
		t.Fatalf("BindDefault should bind when nothing is bound: want 200ms, got %v", p.Timeout)
	}
}

// TestGoLive_BuildsInjectorFromSnapshot covers the starter-facing completion
// hook: GoLive builds the fault injector from the CURRENT snapshot, registers
// the seams, and fires OnReady. (The seams are replace-safe atomics; Reset()
// restores the live flag afterwards.)
func TestGoLive_BuildsInjectorFromSnapshot(t *testing.T) {
	defer Reset()

	SetSource(NewPushSource(Config{Enabled: true, Fault: fault.Config{Enabled: true, Rate: 0.25}}))
	GoLive()

	ready := false
	OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the authority live")
	}
	if !Enabled() {
		t.Fatal("GoLive should keep the bound source armed")
	}
	if in := fault.InjectorFor(); in == nil || !in.Config().Enabled || in.Config().Rate != 0.25 {
		t.Fatal("GoLive should register the injector built from the snapshot's Fault")
	}

	// Idempotent: a second GoLive does not rebuild the injector.
	GoLive()
}

// TestPushSource_Concurrent runs Push/Snapshot/Subscribe concurrently; run
// with -race, this guards the lock discipline of the ready-made source.
func TestPushSource_Concurrent(t *testing.T) {
	p := NewPushSource(Config{})
	done := make(chan struct{})
	go func() { defer close(done); p.Subscribe(func(Config) {}) }()
	for i := range 100 {
		p.Push(enabledTimeout(i))
		_ = p.Snapshot()
	}
	<-done
	p.Push(enabledTimeout(1))
}

// TestDestroy_ClosesCloseableSource pins Destroy's optional-close contract:
// a source implementing Close is closed; one without Close is left alone.
func TestDestroy_ClosesCloseableSource(t *testing.T) {
	// With a closeable source.
	c := NewCenter(Config{})
	src := newCloseableSource(Config{})
	c.setSource(src)
	if err := c.Destroy(); err != nil {
		t.Fatal(err)
	}
	if !src.closed {
		t.Fatal("Destroy should close a closeable source")
	}

	// With a plain PushSource (no Close method): Destroy is a no-op.
	c2 := NewCenter(Config{})
	c2.setSource(NewPushSource(Config{}))
	if err := c2.Destroy(); err != nil {
		t.Fatal(err)
	}
}

// closeableSource is a Source that also implements Close, to exercise
// Destroy's type assertion.
type closeableSource struct {
	Push   *PushSource
	closed bool
}

func newCloseableSource(cfg Config) *closeableSource {
	return &closeableSource{Push: NewPushSource(cfg)}
}

func (s *closeableSource) Snapshot() Config          { return s.Push.Snapshot() }
func (s *closeableSource) Subscribe(cb func(Config)) { s.Push.Subscribe(cb) }
func (s *closeableSource) Close() error              { s.closed = true; return nil }
