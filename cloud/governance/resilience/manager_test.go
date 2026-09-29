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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// errFail is a plain failure used to drive the breaker in tests.
var errFail = errors.New("test: fail")

// armed returns a manager armed with a resolver backed by fixedPolicies.
func armed(t *testing.T, policies map[string]ClientPolicy, driver string) *Manager {
	t.Helper()
	m := NewManager()
	if err := m.Apply(Settings{
		Enabled:             true,
		Driver:              driver,
		ResolveClientPolicy: func(label string) ClientPolicy { return policies[label] },
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return m
}

// runOnce runs fn through e and reports whether it ran.
func runOnce(t *testing.T, e ClientExecutor) bool {
	t.Helper()
	ran := false
	err := e.Execute(context.Background(), func(context.Context) error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return ran
}

func TestManagerUnarmedIsPassThrough(t *testing.T) {
	m := NewManager()
	if !runOnce(t, m.ClientExecutorFor("redis", "redis:cache")) {
		t.Fatal("unarmed manager must run fn")
	}
	if p := m.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("unarmed manager must resolve a zero policy, got %+v", p)
	}
	if d := m.Driver(); d != DefaultDriverName {
		t.Fatalf("Driver() = %q, want %q", d, DefaultDriverName)
	}
}

func TestManagerDisabledIsPassThrough(t *testing.T) {
	m := NewManager()
	if err := m.Apply(Settings{
		Enabled:             false,
		ResolveClientPolicy: func(string) ClientPolicy { return ClientPolicy{AttemptTimeout: time.Second} },
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// A disabled settings must not arm the resolver: the policy that the
	// resolver would have returned must not take effect.
	if p := m.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("disabled manager must resolve a zero policy, got %+v", p)
	}
	if !runOnce(t, m.ClientExecutorFor("redis", "redis:cache")) {
		t.Fatal("disabled manager must run fn")
	}
}

func TestManagerUnknownDriverFailsApply(t *testing.T) {
	m := NewManager()
	m.SetDrivers(map[string]Driver{"sentinel": NewDefaultDriver(nil)})
	err := m.Apply(Settings{Enabled: true, Driver: "nope", ResolveClientPolicy: func(string) ClientPolicy { return ClientPolicy{} }})
	if err == nil {
		t.Fatal("an enabled settings naming an uninstalled driver must fail Apply")
	}
	// The error names what IS available, so a typo in the configured backend is
	// diagnosable from the startup failure alone.
	if !strings.Contains(err.Error(), `no driver named "nope" (available: [default sentinel])`) {
		t.Fatalf("unknown-driver error must list the available backends, got: %v", err)
	}
	// A disabled settings is never checked: it builds no executor.
	if err := m.Apply(Settings{Enabled: false, Driver: "nope"}); err != nil {
		t.Fatalf("disabled Apply must not validate the driver: %v", err)
	}
}

// TestManagerSharesOneExecutorPerLabel pins the semantic that makes the
// per-label cache load-bearing: two handles for one label must share breaker
// state, or two objects would each trip at their own threshold and effectively
// double it.
func TestManagerSharesOneExecutorPerLabel(t *testing.T) {
	m := armed(t, map[string]ClientPolicy{
		"redis:cache": {ErrorThreshold: 2, OpenDuration: time.Minute},
	}, "")
	a := m.ClientExecutorFor("redis", "redis:cache")
	b := m.ClientExecutorFor("redis", "redis:cache")

	fail := func(e ClientExecutor) {
		_ = e.Execute(context.Background(), func(context.Context) error { return errFail })
	}
	fail(a)
	fail(b) // second failure across BOTH handles trips the shared breaker

	err := a.Execute(context.Background(), func(context.Context) error { return nil })
	if err != ErrCircuitOpen {
		t.Fatalf("breaker state must be shared across handles for one label, got %v", err)
	}
}

// TestManagerSubscribeArmsAndNotifies covers the subscription contract: immediate
// arming, notification on change, silence on an unchanged policy, and cancel.
func TestManagerSubscribeArmsAndNotifies(t *testing.T) {
	var mu sync.Mutex
	policies := map[string]ClientPolicy{"redis:cache": {AttemptTimeout: time.Second}}
	resolve := func(label string) ClientPolicy {
		mu.Lock()
		defer mu.Unlock()
		return policies[label]
	}
	m := NewManager()
	if err := m.Apply(Settings{Enabled: true, ResolveClientPolicy: resolve}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var got []ClientPolicy
	sub := m.Subscribe("redis:cache", func(p ClientPolicy) { got = append(got, p) })
	if len(got) != 1 || got[0].AttemptTimeout != time.Second {
		t.Fatalf("Subscribe must arm immediately, got %+v", got)
	}
	if sub.ClientPolicy.AttemptTimeout != time.Second {
		t.Fatalf("Subscription.ClientPolicy = %+v, want the arming policy", sub.ClientPolicy)
	}

	// An apply that leaves this label's policy unchanged must not notify.
	if err := m.Apply(Settings{Enabled: true, ResolveClientPolicy: resolve}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("an unchanged policy must not re-notify, got %d calls", len(got))
	}

	// A change must notify with the new policy.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 2 * time.Second}
	mu.Unlock()
	if err := m.Apply(Settings{Enabled: true, ResolveClientPolicy: resolve}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(got) != 2 || got[1].AttemptTimeout != 2*time.Second {
		t.Fatalf("a changed policy must notify, got %+v", got)
	}

	// Cancel must stop delivery and be idempotent.
	sub.Cancel()
	sub.Cancel()
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 3 * time.Second}
	mu.Unlock()
	if err := m.Apply(Settings{Enabled: true, ResolveClientPolicy: resolve}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
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

// TestManagerApplyRebuildsOnDriverSwitch pins that adopting a different backend
// drops the memoized executors: they were built by the previous driver and must
// not outlive it.
func TestManagerApplyRebuildsOnDriverSwitch(t *testing.T) {
	m := NewManager()
	var mu sync.Mutex
	var built []string
	m.SetDrivers(map[string]Driver{
		"a": namedDriver{name: "a", built: &built, mu: &mu},
		"b": namedDriver{name: "b", built: &built, mu: &mu},
	})
	resolve := func(string) ClientPolicy { return ClientPolicy{AttemptTimeout: time.Second} }

	if err := m.Apply(Settings{Enabled: true, Driver: "a", ResolveClientPolicy: resolve}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	runOnce(t, m.ClientExecutorFor("redis", "redis:cache"))
	if len(built) != 1 {
		t.Fatalf("expected one executor build, got %v", built)
	}
	if err := m.Apply(Settings{Enabled: true, Driver: "b", ResolveClientPolicy: resolve}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(built) != 1 {
		t.Fatalf("applying must not build eagerly, got %v", built)
	}
	runOnce(t, m.ClientExecutorFor("redis", "redis:cache"))
	if len(built) != 2 || built[1] != "b" {
		t.Fatalf("a driver switch must rebuild via the new backend, got %v", built)
	}
}

// namedDriver records the name of every driver asked to build an executor.
// TestManagerServerLaneIsSeparate pins that the two directions are separate
// lanes over separate models: ServerExecutorFor builds through Driver.NewServerExecutor
// (not NewClientExecutor), reads the admission resolver, and a client-side policy change
// never touches the inbound model.
func TestManagerServerLaneIsSeparate(t *testing.T) {
	m := NewManager()
	m.SetDrivers(map[string]Driver{"counting": &countingDriver{}})

	var mu sync.Mutex
	policies := map[string]ClientPolicy{"redis:cache": {AttemptTimeout: time.Second}}
	serverPolicies := map[string]ServerPolicy{"gin::8080": {RateLimit: 7}}
	if err := m.Apply(Settings{
		Enabled: true,
		Driver:  "counting",
		ResolveClientPolicy: func(label string) ClientPolicy {
			mu.Lock()
			defer mu.Unlock()
			return policies[label]
		},
		ResolveServerPolicy: func(label string) ServerPolicy {
			mu.Lock()
			defer mu.Unlock()
			return serverPolicies[label]
		},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	runOnce(t, m.ClientExecutorFor("redis", "redis:cache"))
	if err := m.ServerExecutorFor("gin", "gin::8080").Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("admission Execute: %v", err)
	}

	drv := m.drivers["counting"].(*countingDriver)
	if got := drv.executors.Load(); got != 1 {
		t.Fatalf("want one outbound build, got %d", got)
	}
	if got := drv.serverPolicies.Load(); got != 1 {
		t.Fatalf("ServerExecutorFor must build through NewServerExecutor, got %d admission builds", got)
	}
	if got := drv.lastServerPolicy.Load().(ServerPolicy).RateLimit; got != 7 {
		t.Fatalf("ServerExecutorFor must resolve the ADMISSION resolver, got rate limit %v", got)
	}

	// A later outbound-only change leaves the inbound model untouched.
	mu.Lock()
	policies["redis:cache"] = ClientPolicy{AttemptTimeout: 2 * time.Second}
	mu.Unlock()
	if err := m.Apply(Settings{
		Enabled:             true,
		Driver:              "counting",
		ResolveClientPolicy: func(label string) ClientPolicy { mu.Lock(); defer mu.Unlock(); return policies[label] },
		ResolveServerPolicy: func(label string) ServerPolicy { mu.Lock(); defer mu.Unlock(); return serverPolicies[label] },
	}); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if got := drv.serverPolicies.Load(); got != 1 {
		t.Fatalf("an outbound-only change must not rebuild inbound admission, got %d builds", got)
	}
}

// countingDriver records how many executors it built per direction, and what
// admission model it was last handed.
type countingDriver struct {
	executors        atomic.Int32
	serverPolicies   atomic.Int32
	lastServerPolicy atomic.Value // ServerPolicy
}

func (d *countingDriver) NewClientExecutor(service string, p ClientPolicy) (ClientExecutor, error) {
	d.executors.Add(1)
	return NewDefaultDriver(nil).NewClientExecutor(service, p)
}

func (d *countingDriver) NewServerExecutor(service string, a ServerPolicy) (ServerExecutor, error) {
	d.serverPolicies.Add(1)
	d.lastServerPolicy.Store(a)
	return NewDefaultDriver(nil).NewServerExecutor(service, a)
}

type namedDriver struct {
	name  string
	built *[]string
	mu    *sync.Mutex
}

func (d namedDriver) NewClientExecutor(service string, p ClientPolicy) (ClientExecutor, error) {
	d.mu.Lock()
	*d.built = append(*d.built, d.name)
	d.mu.Unlock()
	return NewDefaultDriver(nil).NewClientExecutor(service, p)
}

func (d namedDriver) NewServerExecutor(service string, a ServerPolicy) (ServerExecutor, error) {
	d.mu.Lock()
	*d.built = append(*d.built, d.name)
	d.mu.Unlock()
	return NewDefaultDriver(nil).NewServerExecutor(service, a)
}
