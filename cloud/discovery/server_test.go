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

package discovery

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// fakeRegistry records every lifecycle call so tests can assert what the
// single discoveryServer drove, without a discovery center. fail maps an
// operation ("register", "deregister", "update-weight") to an injected
// failure.
type fakeRegistry struct {
	mu     sync.Mutex
	events []string
	ctxs   map[string]context.Context // the context each op was last called with
	fail   map[string]error
}

func (f *fakeRegistry) record(op string, ctx context.Context) error {
	f.mu.Lock()
	if f.ctxs == nil {
		f.ctxs = map[string]context.Context{}
	}
	f.ctxs[op] = ctx
	f.mu.Unlock()

	if err := f.fail[op]; err != nil {
		return err
	}
	f.mu.Lock()
	f.events = append(f.events, op)
	f.mu.Unlock()
	return nil
}

// ctxFor returns the context the named operation was last called with, so a
// test can assert what rode on it.
func (f *fakeRegistry) ctxFor(op string) context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ctxs[op]
}

func (f *fakeRegistry) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...)
}

func (f *fakeRegistry) has(op string) bool {
	for _, e := range f.snapshot() {
		if e == op {
			return true
		}
	}
	return false
}

func (f *fakeRegistry) reset() {
	f.mu.Lock()
	f.events = nil
	f.ctxs = nil
	f.mu.Unlock()
}

func (f *fakeRegistry) Register(ctx context.Context, inst Instance) error {
	return f.record("register", ctx)
}

func (f *fakeRegistry) Deregister(ctx context.Context, inst Instance) error {
	return f.record("deregister", ctx)
}

func (f *fakeRegistry) UpdateWeight(ctx context.Context, inst Instance, weight int) error {
	return f.record("update-weight", ctx)
}

// compile-time contract: the fake stands in for a real backend registry.
var _ Registry = (*fakeRegistry)(nil)

// regA / regB are the two fake backend registrys the test container collects.
// The same instances back every test in this file; each test resets them, and
// the beans are only instantiated when a test's injections cite them.
var (
	regA = &fakeRegistry{}
	regB = &fakeRegistry{}
)

func init() {
	// Two named fake discovery beans exported as Registry: the container's
	// slice collection the Server bean autowires is exactly the mechanism under
	// test (one bean per configured center, across backends).
	gs.Provide(func() *fakeRegistry { return regA }).
		Name("fake-a").Export(gs.As[Registry]())
	gs.Provide(func() *fakeRegistry { return regB }).
		Name("fake-b").Export(gs.As[Registry]())
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestServerLifecycle pins the whole publication lifecycle through a real gs
// container: the OnProperty condition activates the server, every collected
// registry receives the register (once the app is ready) and — after the
// container's shutdown — the deregister, in both cases across ALL centers.
func TestServerLifecycle(t *testing.T) {
	regA.reset()
	regB.reset()

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.discovery.service-name", "orders")
		app.Property("spring.discovery.addr", "10.0.0.5:8080")
	}).RunTest(t, func(s *struct {
		Server *Server `autowire:"discoveryServer"`
	}) {
		if s.Server == nil {
			t.Fatal("discoveryServer bean must exist when ${spring.discovery.service-name} is set")
		}
		if len(s.Server.Registries) != 2 {
			t.Fatalf("the server must collect every discovery bean, want 2 got %d", len(s.Server.Registries))
		}

		// Registration runs after the ready signal, which races this callback;
		// poll for it rather than assuming ordering.
		waitFor(t, func() bool { return regA.has("register") && regB.has("register") },
			"the app-ready register never reached both centers")
	})

	// Shutdown ran PreStop: both centers saw the deregister (the lossless-drain
	// sequence) even though neither was explicitly driven by the test body.
	for _, r := range []*fakeRegistry{regA, regB} {
		if !r.has("deregister") {
			t.Fatalf("shutdown must deregister from every center, events=%v", r.snapshot())
		}
	}
}

// TestServerNotTriggeredWithoutServiceName pins the OnProperty guard: with no
// ${spring.discovery.service-name} the server bean does not exist — pure
// consumers register nothing anywhere, and the discovery beans simply sit
// uninstantiated.
func TestServerNotTriggeredWithoutServiceName(t *testing.T) {
	regA.reset()

	gs.Web(false).RunTest(t, func(s *struct {
		Server *Server `autowire:"discoveryServer"`
	}) {
		if s.Server != nil {
			t.Fatal("discoveryServer must not exist without ${spring.discovery.service-name}")
		}
	})
	if regA.has("register") {
		t.Fatal("no registry may be driven without the registration intent signal")
	}
}

// TestUpdateWeightBroadcast covers the runtime API against the live server:
// every center re-advertises, and the first failure aborts the sweep — later
// centers must NOT get a weight the failed one never saw.
func TestUpdateWeightBroadcast(t *testing.T) {
	regA.reset()
	regB.reset()
	regB.fail = map[string]error{"update-weight": errors.New("center down")}
	defer func() { regB.fail = nil }()

	s := &Server{
		inst:       Instance{ServiceName: "orders", Addr: "10.0.0.5:8080"},
		Registries: []Registry{regA, regB},
	}

	// Broadcast order follows the slice: regA succeeds, regB fails.
	if err := s.UpdateWeight(context.Background(), 0); err == nil {
		t.Fatal("a failing center must fail the weight update")
	}
	if !regA.has("update-weight") {
		t.Fatal("the healthy center must still have received the update")
	}
}

// TestUpdateWeightBeforeRegister pins the guard: the runtime API is callable
// only after Run has published the instance.
func TestUpdateWeightBeforeRegister(t *testing.T) {
	s := NewServer()
	s.Registries = []Registry{regA}
	err := s.UpdateWeight(context.Background(), 1)
	assert.Error(t, err).Matches("not registered yet")
}

// TestUpdateWeightInheritsTheCallerPath proves the runtime API is a step inside
// the caller's path, not a path of its own: it hands the caller's identity
// through and mints none of its own, so a caller that already carries a
// trace_id (a management endpoint's span) does not end up with two.
func TestUpdateWeightInheritsTheCallerPath(t *testing.T) {
	regA.reset()
	s := &Server{
		inst:       Instance{ServiceName: "orders", Addr: "10.0.0.5:8080"},
		Registries: []Registry{regA},
	}

	caller := log.WithFields(context.Background(), log.String("trace_id", "caller-id"))
	assert.Error(t, s.UpdateWeight(caller, 1)).Nil()

	got := renderFields(log.CarriedFields(regA.ctxFor("update-weight")))
	assert.String(t, got).Contains("trace_id=caller-id")
	assert.String(t, got).Contains("service=orders") // the instance identity rides along
	assert.Number(t, strings.Count(got, "trace_id=")).Equal(1)
}

// TestDeregisterMintsItsOwnRoot proves the drain is a path of its own: it roots
// itself rather than hanging off the shutdown context, so it carries its own
// trace_id instead of inheriting the caller's.
func TestDeregisterMintsItsOwnRoot(t *testing.T) {
	regA.reset()
	s := &Server{
		inst:       Instance{ServiceName: "orders", Addr: "10.0.0.5:8080"},
		Registries: []Registry{regA},
	}

	caller := log.WithFields(context.Background(), log.String("trace_id", "caller-id"))
	s.PreStop(caller)

	got := renderFields(log.CarriedFields(regA.ctxFor("deregister")))
	assert.String(t, got).Contains("trace_id=")
	assert.String(t, got).Contains("service=orders")
	assert.That(t, !strings.Contains(got, "caller-id")).True("the drain mints its own id, it does not inherit the caller's")
}

// renderFields renders fields the way a log line does, so a test can read a
// field's value through the public surface instead of the Field internals.
func renderFields(fields []log.Field) string {
	var buf bytes.Buffer
	log.EncodeFields(log.NewTextEncoder(&buf, " "), fields)
	return buf.String()
}

// TestDeregisterContinuesPastFailure pins the shutdown contract: one center
// failing its deregister must not stop the others — every center gets the
// drain call, and the failure itself surfaces in the log (Warn), not as an
// aborted sweep.
func TestDeregisterContinuesPastFailure(t *testing.T) {
	regA.reset()
	regB.reset()
	regA.fail = map[string]error{"deregister": errors.New("center down")}
	defer func() { regA.fail = nil }()

	s := &Server{
		inst:       Instance{ServiceName: "orders", Addr: "10.0.0.5:8080"},
		Registries: []Registry{regA, regB},
	}
	s.deregister()

	if !regB.has("deregister") {
		t.Fatal("the sweep must continue past a failing center")
	}
}
