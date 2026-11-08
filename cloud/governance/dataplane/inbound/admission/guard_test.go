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

package admission

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/bulkhead"
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/inbound/chain"
)

func ctx() context.Context { return context.Background() }

var id = Identity{System: "gin", Service: "gin:127.0.0.1:8080"}

// mustNew builds a standalone Guard whose policy is expected to be valid.
func mustNew(t *testing.T, p Policy, opts ...Option) Guard {
	t.Helper()
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	return mustNewWith(t, p, factories, opts...)
}

func mustNewWith(t *testing.T, p Policy, factories *ratelimit.Factory, opts ...Option) Guard {
	t.Helper()
	g, err := New(id, p, factories, opts...)
	if err != nil {
		t.Fatalf("New(%+v): %v", p, err)
	}
	return g
}

// mustGuardFor creates the Guard for id under m, whose settings are expected to
// be valid.
func mustGuardFor(t *testing.T, m *Manager, id Identity) Guard {
	t.Helper()
	g, err := m.GuardFor(id)
	if err != nil {
		t.Fatalf("GuardFor(%q): %v", id.Service, err)
	}
	return g
}

func TestZeroPolicyPassesThrough(t *testing.T) {
	g := mustNew(t, Policy{})
	defer g.Close()

	r, done := g.Admit(ctx(), chain.Request{Action: "GET /ping"})
	if !r.Admitted() || r.Verdict() != VerdictAdmitted {
		t.Fatalf("zero policy refused: verdict=%v err=%v", r.Verdict(), r.Err())
	}
	if r.Err() != nil {
		t.Fatalf("zero policy returned error: %v", r.Err())
	}
	done() // no-op, must not panic
}

func TestRateLimit(t *testing.T) {
	g := mustNew(t, Policy{Policy: ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1}})
	defer g.Close()

	r, _ := g.Admit(ctx(), chain.Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}

	r, _ = g.Admit(ctx(), chain.Request{})
	if r.Admitted() || r.Verdict() != VerdictRateLimited {
		t.Fatalf("second request admitted: verdict=%v", r.Verdict())
	}
	if !errors.Is(r.Err(), ErrRateLimited) {
		t.Fatalf("unexpected error: %v", r.Err())
	}
}

func TestBulkheadPairing(t *testing.T) {
	g := mustNew(t, Policy{MaxConcurrent: 1})
	defer g.Close()

	r1, done1 := g.Admit(ctx(), chain.Request{})
	if !r1.Admitted() {
		t.Fatalf("first request refused: %v", r1.Err())
	}

	r2, _ := g.Admit(ctx(), chain.Request{})
	if r2.Verdict() != VerdictBulkheadFull {
		t.Fatalf("second request not refused by bulkhead: %v", r2.Verdict())
	}

	done1()

	r3, _ := g.Admit(ctx(), chain.Request{})
	if !r3.Admitted() {
		t.Fatalf("slot not released: %v", r3.Err())
	}
}

func TestDoneIsIdempotent(t *testing.T) {
	g := mustNew(t, Policy{MaxConcurrent: 1})
	defer g.Close()

	_, done := g.Admit(ctx(), chain.Request{})
	done()
	done() // must not release the slot twice or panic

	// The slot is free exactly once: a double release would have returned a
	// second slot the bulkhead never held.
	r, _ := g.Admit(ctx(), chain.Request{})
	if !r.Admitted() {
		t.Fatalf("slot not released: %v", r.Err())
	}
	r, _ = g.Admit(ctx(), chain.Request{})
	if r.Verdict() != VerdictBulkheadFull {
		t.Fatalf("expected bulkhead full, got %v", r.Verdict())
	}
}

type fakeFault struct {
	mu   sync.Mutex
	err  error
	rule string
	call int
}

func (f *fakeFault) Fire(context.Context, chain.Request) (kfault.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.call++
	fr := kfault.Result{}
	if f.err != nil {
		fr.Name, fr.Inject, fr.Error = f.rule, true, "boom"
	}
	return fr, f.err
}

func TestFaultInjection(t *testing.T) {
	f := &fakeFault{err: errors.New("injected"), rule: "svc[0]"}
	g := mustNew(t, Policy{}, WithFault(f))
	defer g.Close()

	r, done := g.Admit(ctx(), chain.Request{})
	if r.Verdict() != VerdictFaultInjected {
		t.Fatalf("fault not injected: %v", r.Verdict())
	}
	// The fault's attribution lives in Fault(); Result.Name names the FAULT
	// rule only, so the admission policy's own rule stays empty.
	if r.Err() == nil || !r.Fault().Fired() || r.Fault().Name != "svc[0]" || r.Rule() != "" {
		t.Fatalf("fault result not reported: err=%v fault=%+v rule=%q", r.Err(), r.Fault(), r.Rule())
	}
	done() // no-op, must not panic
}

func TestManagerPush(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	g := mustGuardFor(t, m, id)

	r, _ := g.Admit(ctx(), chain.Request{})
	if !r.Admitted() {
		t.Fatalf("a manager with no document refused: %v", r.Err())
	}

	if err := m.Apply(Settings{Rules: []Rule{{Service: id.Service, Policy: Policy{MaxConcurrent: 1}}}}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	r1, done := g.Admit(ctx(), chain.Request{})
	if !r1.Admitted() {
		t.Fatalf("first request after push refused: %v", r1.Err())
	}
	r2, _ := g.Admit(ctx(), chain.Request{})
	if r2.Verdict() != VerdictBulkheadFull {
		t.Fatalf("push not applied: %v", r2.Verdict())
	}
	done()
}

func TestTwoInstancesDoNotShareState(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	_ = m.Apply(Settings{Default: Policy{MaxConcurrent: 1}})

	a := mustGuardFor(t, m, id)
	b := mustGuardFor(t, m, id) // same label, different server instance

	ra, doneA := a.Admit(ctx(), chain.Request{})
	if !ra.Admitted() {
		t.Fatalf("a refused: %v", ra.Err())
	}
	rb, _ := b.Admit(ctx(), chain.Request{})
	if !rb.Admitted() {
		t.Fatalf("b shared a's bulkhead: %v", rb.Err())
	}
	// a's own second request is refused; a's slot is still held.
	ra2, _ := a.Admit(ctx(), chain.Request{})
	if ra2.Verdict() != VerdictBulkheadFull {
		t.Fatalf("a's bulkhead not independent: %v", ra2.Verdict())
	}
	doneA()
}

func TestDeadlineReleasesBulkhead(t *testing.T) {
	g := mustNew(t, Policy{MaxConcurrent: 1})
	defer g.Close()

	dl, cancel := context.WithTimeout(ctx(), 20*time.Millisecond)
	defer cancel()
	r, _ := g.Admit(dl, chain.Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}
	// Deliberately never call Done; the deadline fallback must release the slot.
	time.Sleep(50 * time.Millisecond)

	r2, _ := g.Admit(ctx(), chain.Request{})
	if !r2.Admitted() {
		t.Fatalf("deadline fallback did not release the slot: %v", r2.Err())
	}
}

type fakeLimiter struct {
	allow bool
	err   error
	spec  ratelimit.Policy
	calls int
}

func (l *fakeLimiter) Allow(_ context.Context, spec ratelimit.Policy, _ int) (bool, error) {
	l.calls++
	l.spec = spec
	if l.err != nil {
		return false, l.err
	}
	return l.allow, nil
}

// fakeProvider builds the one fake limiter and records the scope it was built
// for.
type fakeProvider struct {
	l     *fakeLimiter
	scope string
}

func (f *fakeProvider) Algorithms() []string {
	return []string{ratelimit.TokenBucketAlgorithm}
}

func (f *fakeProvider) New(_ string, scope string) (ratelimit.Limiter, error) {
	f.scope = scope
	return f.l, nil
}

func TestApplyRejectsUnknownLimiterKind(t *testing.T) {
	f := &fakeProvider{l: &fakeLimiter{allow: true}}
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{f})
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	// A Guard is what makes the default policy actually used; an unserved rule
	// is just data and is not validated.
	g := mustGuardFor(t, m, id)
	defer g.Close()

	sliding := Settings{Default: Policy{Policy: ratelimit.Policy{RateLimit: 1, Limiter: "redis:sliding-window"}}}
	if err := m.Apply(sliding); err == nil {
		t.Fatal("a rule naming an unregistered limiter kind was accepted")
	}

	bucket := Settings{Default: Policy{Policy: ratelimit.Policy{RateLimit: 1, Limiter: "token-bucket"}}}
	if err := m.Apply(bucket); err != nil {
		t.Fatalf("token-bucket rule refused: %v", err)
	}
}

func TestClusterLimiter(t *testing.T) {
	l := &fakeLimiter{allow: true}
	f := &fakeProvider{l: l}
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{f})
	if err != nil {
		t.Fatal(err)
	}
	g := mustNewWith(t, Policy{Policy: ratelimit.Policy{RateLimit: 10, Burst: 3, Limiter: "token-bucket"}}, factories)
	defer g.Close()

	r, _ := g.Admit(ctx(), chain.Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}
	if l.calls != 1 || f.scope != id.Service {
		t.Fatalf("limiter not charged: calls=%d scope=%q", l.calls, f.scope)
	}
	if l.spec.RateLimit != 10 || l.spec.Burst != 3 {
		t.Fatalf("limiter got wrong spec: %+v", l.spec)
	}

	l.allow = false
	r, _ = g.Admit(ctx(), chain.Request{})
	if r.Verdict() != VerdictRateLimited {
		t.Fatalf("store rejection not surfaced: %v", r.Verdict())
	}

	// A store failure is not an inbound decision: the request proceeds.
	l.err = errors.New("store down")
	r, _ = g.Admit(ctx(), chain.Request{})
	if !r.Admitted() {
		t.Fatalf("store failure must allow the request: %v", r.Err())
	}
}

// TestRateStateSurvivesPolicyChange pins the shared model on the inbound side:
// a policy change updates the counter's rule without refilling it.
func TestRateStateSurvivesPolicyChange(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()

	id := Identity{System: "sys", Service: "svc-rate-survives"}
	if err := m.Apply(Settings{Default: Policy{Policy: ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := mustGuardFor(t, m, id)
	defer g.Close()

	if r, _ := g.Admit(ctx(), chain.Request{}); !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}
	if r, _ := g.Admit(ctx(), chain.Request{}); r.Admitted() {
		t.Fatal("second request should be rate limited")
	}

	// Raising the burst must not refill the spent token.
	if err := m.Apply(Settings{Default: Policy{Policy: ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 10}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if r, _ := g.Admit(ctx(), chain.Request{}); r.Admitted() {
		t.Fatal("policy change refilled the rate counter")
	}
}

func TestStandaloneBulkhead(t *testing.T) {
	b := mustBulkhead(t, 1)
	release, ok := b.Acquire(ctx())
	if !ok {
		t.Fatal("first acquire should succeed")
	}
	if _, ok := b.Acquire(ctx()); ok {
		t.Fatal("second acquire should be refused")
	}
	release()
	if _, ok := b.Acquire(ctx()); !ok {
		t.Fatal("slot was not released")
	}
}

func TestTimeoutExposed(t *testing.T) {
	g := mustNew(t, Policy{Timeout: 3 * time.Second})
	defer g.Close()
	r, _ := g.Admit(ctx(), chain.Request{})
	if r.Timeout() != 3*time.Second {
		t.Fatalf("timeout = %v", r.Timeout())
	}
}

// TestNegativeMaxConcurrentRejected pins the bulkhead's constructor invariant:
// the caller guarantees maxConcurrent >= 0, and an invalid policy is rejected
// before a Bulkhead is built.
func TestNegativeMaxConcurrentRejected(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(id, Policy{MaxConcurrent: -1}, factories); err == nil {
		t.Fatal("New accepted a negative max-concurrent")
	}
}

// mustBulkhead builds a bulkhead, failing the test if its input is rejected.
func mustBulkhead(t *testing.T, maxConcurrent int, opts ...bulkhead.Option) *bulkhead.Bulkhead {
	t.Helper()
	b, err := bulkhead.New(maxConcurrent, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
