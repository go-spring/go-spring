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

package registrycore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// fakeRegistrar records every lifecycle call so tests can assert what the
// single registryServer drove, without a registry center. fail maps an
// operation ("register", "deregister", "update-weight") to an injected
// failure.
type fakeRegistrar struct {
	mu     sync.Mutex
	events []string
	fail   map[string]error
}

func (f *fakeRegistrar) record(op string) error {
	if err := f.fail[op]; err != nil {
		return err
	}
	f.mu.Lock()
	f.events = append(f.events, op)
	f.mu.Unlock()
	return nil
}

func (f *fakeRegistrar) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...)
}

func (f *fakeRegistrar) has(op string) bool {
	for _, e := range f.snapshot() {
		if e == op {
			return true
		}
	}
	return false
}

func (f *fakeRegistrar) reset() {
	f.mu.Lock()
	f.events = nil
	f.mu.Unlock()
}

func (f *fakeRegistrar) Register(ctx context.Context, inst discovery.Instance) error {
	return f.record("register")
}

func (f *fakeRegistrar) Deregister(ctx context.Context, inst discovery.Instance) error {
	return f.record("deregister")
}

func (f *fakeRegistrar) UpdateWeight(ctx context.Context, inst discovery.Instance, weight int) error {
	return f.record("update-weight")
}

// compile-time contract: the fake stands in for a real backend registrar.
var _ discovery.Registrar = (*fakeRegistrar)(nil)

// regA / regB are the two fake backend registrars the test container collects.
// The same instances back every test in this file; each test resets them, and
// the beans are only instantiated when a test's injections cite them.
var (
	regA = &fakeRegistrar{}
	regB = &fakeRegistrar{}
)

func init() {
	// Two named fake registrar beans exported as discovery.Registrar: the
	// container's slice collection the Server bean autowires is exactly the
	// mechanism under test (one bean per configured center, across backends).
	gs.Provide(func() *fakeRegistrar { return regA }).
		Name("fake-a").Export(gs.As[discovery.Registrar]())
	gs.Provide(func() *fakeRegistrar { return regB }).
		Name("fake-b").Export(gs.As[discovery.Registrar]())
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
// registrar receives the register (once the app is ready) and — after the
// container's shutdown — the deregister, in both cases across ALL centers.
func TestServerLifecycle(t *testing.T) {
	regA.reset()
	regB.reset()

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.registry.service-name", "orders")
		app.Property("spring.registry.addr", "10.0.0.5:8080")
	}).RunTest(t, func(s *struct {
		Server *Server `autowire:"registryServer"`
	}) {
		if s.Server == nil {
			t.Fatal("registryServer bean must exist when ${spring.registry.service-name} is set")
		}
		if len(s.Server.Registrars) != 2 {
			t.Fatalf("the server must collect every registrar bean, want 2 got %d", len(s.Server.Registrars))
		}

		// Registration runs after the ready signal, which races this callback;
		// poll for it rather than assuming ordering.
		waitFor(t, func() bool { return regA.has("register") && regB.has("register") },
			"the app-ready register never reached both centers")
	})

	// Shutdown ran PreStop: both centers saw the deregister (the lossless-drain
	// sequence) even though neither was explicitly driven by the test body.
	for _, r := range []*fakeRegistrar{regA, regB} {
		if !r.has("deregister") {
			t.Fatalf("shutdown must deregister from every center, events=%v", r.snapshot())
		}
	}
}

// TestServerNotTriggeredWithoutServiceName pins the OnProperty guard: with no
// ${spring.registry.service-name} the server bean does not exist — pure
// consumers register nothing anywhere, and the registrar beans simply sit
// uninstantiated.
func TestServerNotTriggeredWithoutServiceName(t *testing.T) {
	regA.reset()

	gs.Web(false).RunTest(t, func(s *struct {
		Server *Server `autowire:"registryServer"`
	}) {
		if s.Server != nil {
			t.Fatal("registryServer must not exist without ${spring.registry.service-name}")
		}
	})
	if regA.has("register") {
		t.Fatal("no registrar may be driven without the registration intent signal")
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
		inst:       discovery.Instance{ServiceName: "orders", Addr: "10.0.0.5:8080"},
		Registrars: []discovery.Registrar{regA, regB},
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
	s.Registrars = []discovery.Registrar{regA}
	err := s.UpdateWeight(context.Background(), 1)
	assert.Error(t, err).Matches("not registered yet")
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
		inst:       discovery.Instance{ServiceName: "orders", Addr: "10.0.0.5:8080"},
		Registrars: []discovery.Registrar{regA, regB},
	}
	s.deregister(context.Background())

	if !regB.has("deregister") {
		t.Fatal("the sweep must continue past a failing center")
	}
}
