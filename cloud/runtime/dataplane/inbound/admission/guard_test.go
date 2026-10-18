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
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/breaker"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
)

func ctx() context.Context { return context.Background() }

var id = Identity{System: "gin", Service: "gin:127.0.0.1:8080"}

func TestZeroPolicyPassesThrough(t *testing.T) {
	g := New(id, Policy{})
	defer g.Close()

	r, done := g.Admit(ctx(), Request{Action: "GET /ping"})
	if !r.Admitted() || r.Verdict() != VerdictAdmitted {
		t.Fatalf("zero policy refused: verdict=%v err=%v", r.Verdict(), r.Err())
	}
	if r.Err() != nil {
		t.Fatalf("zero policy returned error: %v", r.Err())
	}
	done(OutcomeSuccess, nil) // no-op, must not panic
}

func TestRateLimit(t *testing.T) {
	g := New(id, Policy{RateLimit: 1, Burst: 1})
	defer g.Close()

	r, _ := g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}

	r, _ = g.Admit(ctx(), Request{})
	if r.Admitted() || r.Verdict() != VerdictRateLimited {
		t.Fatalf("second request admitted: verdict=%v", r.Verdict())
	}
	if !errors.Is(r.Err(), ErrRateLimited) {
		t.Fatalf("unexpected error: %v", r.Err())
	}
}

func TestBulkheadPairing(t *testing.T) {
	g := New(id, Policy{MaxConcurrent: 1})
	defer g.Close()

	r1, done1 := g.Admit(ctx(), Request{})
	if !r1.Admitted() {
		t.Fatalf("first request refused: %v", r1.Err())
	}

	r2, _ := g.Admit(ctx(), Request{})
	if r2.Verdict() != VerdictBulkheadFull {
		t.Fatalf("second request not refused by bulkhead: %v", r2.Verdict())
	}

	done1(OutcomeSuccess, nil)

	r3, _ := g.Admit(ctx(), Request{})
	if !r3.Admitted() {
		t.Fatalf("slot not released: %v", r3.Err())
	}
}

func TestBreakerConsecutive(t *testing.T) {
	g := New(id, Policy{ErrorThreshold: 2, OpenDuration: time.Hour})
	defer g.Close()

	for i := range 2 {
		r, done := g.Admit(ctx(), Request{})
		if !r.Admitted() {
			t.Fatalf("request %d refused before threshold: %v", i, r.Err())
		}
		done(OutcomeFailure, errors.New("boom"))
	}

	r, _ := g.Admit(ctx(), Request{})
	if r.Verdict() != VerdictCircuitOpen {
		t.Fatalf("breaker did not open: verdict=%v", r.Verdict())
	}
	if !errors.Is(r.Err(), ErrCircuitOpen) {
		t.Fatalf("unexpected error: %v", r.Err())
	}
}

func TestBreakerIgnoresUnknownOutcome(t *testing.T) {
	g := New(id, Policy{ErrorThreshold: 1, OpenDuration: time.Hour})
	defer g.Close()

	r, done := g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}
	done(OutcomeUnknown, nil)

	r, _ = g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("unknown outcome tripped the breaker: %v", r.Verdict())
	}
}

func TestDoneIsIdempotent(t *testing.T) {
	g := New(id, Policy{MaxConcurrent: 1, ErrorThreshold: 1, OpenDuration: time.Hour})
	defer g.Close()

	_, done := g.Admit(ctx(), Request{})
	done(OutcomeFailure, errors.New("boom"))
	done(OutcomeFailure, errors.New("boom")) // must not double-count or panic

	// The breaker is open after exactly one failure; a double count would not be
	// observable here, but the bulkhead release must have happened once.
	r, _ := g.Admit(ctx(), Request{})
	if r.Verdict() != VerdictCircuitOpen {
		t.Fatalf("expected open circuit, got %v", r.Verdict())
	}
}

type fakeFault struct {
	mu   sync.Mutex
	err  error
	rule string
	call int
}

func (f *fakeFault) Fire(context.Context, Request) (FaultResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.call++
	return FaultResult{Rule: f.rule, Fired: f.err != nil, Kind: "boom"}, f.err
}

func TestFaultInjection(t *testing.T) {
	f := &fakeFault{err: errors.New("injected"), rule: "svc[0]"}
	g := New(id, Policy{}, WithFault(f))
	defer g.Close()

	r, done := g.Admit(ctx(), Request{})
	if r.Verdict() != VerdictFaultInjected {
		t.Fatalf("fault not injected: %v", r.Verdict())
	}
	if r.Err() == nil || r.Fault().Rule != "svc[0]" || r.Rule() != "svc[0]" {
		t.Fatalf("fault result not reported: err=%v fault=%+v rule=%q", r.Err(), r.Fault(), r.Rule())
	}
	done(OutcomeFailure, r.Err()) // no-op, must not panic
}

func TestManagerPush(t *testing.T) {
	m := NewManager()
	defer m.Close()
	g := m.GuardFor(id)

	r, _ := g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("unarmed manager refused: %v", r.Err())
	}

	if err := m.Apply(Settings{Rules: []Rule{{Service: id.Service, Policy: Policy{MaxConcurrent: 1}}}}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	r1, done := g.Admit(ctx(), Request{})
	if !r1.Admitted() {
		t.Fatalf("first request after push refused: %v", r1.Err())
	}
	r2, _ := g.Admit(ctx(), Request{})
	if r2.Verdict() != VerdictBulkheadFull {
		t.Fatalf("push not applied: %v", r2.Verdict())
	}
	done(OutcomeSuccess, nil)
}

func TestTwoInstancesDoNotShareState(t *testing.T) {
	m := NewManager()
	defer m.Close()
	_ = m.Apply(Settings{Default: Policy{MaxConcurrent: 1}})

	a := m.GuardFor(id)
	b := m.GuardFor(id) // same label, different server instance

	ra, doneA := a.Admit(ctx(), Request{})
	if !ra.Admitted() {
		t.Fatalf("a refused: %v", ra.Err())
	}
	rb, _ := b.Admit(ctx(), Request{})
	if !rb.Admitted() {
		t.Fatalf("b shared a's bulkhead: %v", rb.Err())
	}
	// a's own second request is refused; a's slot is still held.
	ra2, _ := a.Admit(ctx(), Request{})
	if ra2.Verdict() != VerdictBulkheadFull {
		t.Fatalf("a's bulkhead not independent: %v", ra2.Verdict())
	}
	doneA(OutcomeSuccess, nil)
}

func TestDeadlineReleasesBulkhead(t *testing.T) {
	g := New(id, Policy{MaxConcurrent: 1})
	defer g.Close()

	dl, cancel := context.WithTimeout(ctx(), 20*time.Millisecond)
	defer cancel()
	r, _ := g.Admit(dl, Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}
	// Deliberately never call Done; the deadline fallback must release the slot.
	time.Sleep(50 * time.Millisecond)

	r2, _ := g.Admit(ctx(), Request{})
	if !r2.Admitted() {
		t.Fatalf("deadline fallback did not release the slot: %v", r2.Err())
	}
}

type recordingListener struct {
	mu        sync.Mutex
	from, to  []breaker.State
	services  []string
	transited int32
}

func (l *recordingListener) OnBreakerStateChange(service string, from, to breaker.State) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.services = append(l.services, service)
	l.from = append(l.from, from)
	l.to = append(l.to, to)
	atomic.AddInt32(&l.transited, 1)
}

func TestBreakerListener(t *testing.T) {
	g := New(id, Policy{ErrorThreshold: 1, OpenDuration: time.Hour})
	defer g.Close()

	l := &recordingListener{}
	g.(breaker.EventListenerSetter).SetBreakerEventListener(l)

	r, done := g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("request refused: %v", r.Err())
	}
	done(OutcomeFailure, errors.New("boom"))

	if atomic.LoadInt32(&l.transited) != 1 {
		t.Fatalf("expected one transition, got %d", l.transited)
	}
	if l.from[0] != breaker.Closed || l.to[0] != breaker.Open {
		t.Fatalf("unexpected transition %v -> %v", l.from[0], l.to[0])
	}
	if l.services[0] != id.Service {
		t.Fatalf("unexpected service %q", l.services[0])
	}
}

type fakeLimiter struct {
	allow bool
	err   error
	scope string
	spec  ratelimit.Spec
	calls int
}

func (l *fakeLimiter) Allow(_ context.Context, scope string, spec ratelimit.Spec, _ int) (bool, error) {
	l.calls++
	l.scope = scope
	l.spec = spec
	if l.err != nil {
		return false, l.err
	}
	return l.allow, nil
}

func TestClusterLimiter(t *testing.T) {
	l := &fakeLimiter{allow: true}
	g := New(id, Policy{RateLimit: 10, Burst: 3}, WithClusterLimiter(l))
	defer g.Close()

	r, _ := g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("first request refused: %v", r.Err())
	}
	if l.calls != 1 || l.scope != id.Service {
		t.Fatalf("limiter not charged: calls=%d scope=%q", l.calls, l.scope)
	}
	if l.spec.RateLimit != 10 || l.spec.Burst != 3 {
		t.Fatalf("limiter got wrong spec: %+v", l.spec)
	}

	l.allow = false
	r, _ = g.Admit(ctx(), Request{})
	if r.Verdict() != VerdictRateLimited {
		t.Fatalf("store rejection not surfaced: %v", r.Verdict())
	}

	// A store failure is not an inbound decision: the request proceeds.
	l.err = errors.New("store down")
	r, _ = g.Admit(ctx(), Request{})
	if !r.Admitted() {
		t.Fatalf("store failure must allow the request: %v", r.Err())
	}
}

func TestDegradeOnOpen(t *testing.T) {
	g := New(id, Policy{ErrorThreshold: 1, OpenDuration: time.Hour, DegradeOnOpen: true})
	defer g.Close()

	_, done := g.Admit(ctx(), Request{})
	done(OutcomeFailure, errors.New("boom"))

	r, _ := g.Admit(ctx(), Request{})
	if !r.Degraded() {
		t.Fatalf("expected a degraded verdict, got %v", r.Verdict())
	}
	if !errors.Is(r.Err(), ErrCircuitOpen) {
		t.Fatalf("degradation must keep the cause: %v", r.Err())
	}
}

func TestStandaloneRateLimiter(t *testing.T) {
	l := NewRateLimiter(Policy{RateLimit: 1, Burst: 1}, "svc", nil)
	if !l.Allow(ctx()) {
		t.Fatal("first request should pass")
	}
	if l.Allow(ctx()) {
		t.Fatal("second request should be limited")
	}
	if !(*RateLimiter)(nil).Allow(ctx()) {
		t.Fatal("a nil limiter should admit")
	}
}

func TestStandaloneBulkhead(t *testing.T) {
	b := NewBulkhead(1, 0)
	release, ok := b.Acquire(ctx())
	if !ok {
		t.Fatal("first acquire should succeed")
	}
	if _, ok := b.Acquire(ctx()); ok {
		t.Fatal("second acquire should be refused")
	}
	release()
	release() // idempotent
	if _, ok := b.Acquire(ctx()); !ok {
		t.Fatal("slot was not released")
	}
}

func TestStandaloneBreaker(t *testing.T) {
	b := NewBreaker(Policy{ErrorThreshold: 1, OpenDuration: time.Hour}, "svc", nil)
	if !b.Allow() {
		t.Fatal("closed breaker should allow")
	}
	b.Record(OutcomeFailure, 0)
	if b.Allow() {
		t.Fatal("breaker should be open")
	}
	if !(*Breaker)(nil).Allow() {
		t.Fatal("a nil breaker should allow")
	}
}

func TestTimeoutExposed(t *testing.T) {
	g := New(id, Policy{Timeout: 3 * time.Second})
	defer g.Close()
	r, _ := g.Admit(ctx(), Request{})
	if r.Timeout() != 3*time.Second {
		t.Fatalf("timeout = %v", r.Timeout())
	}
}
