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
	"context"
	"strings"
	"testing"
)

// staticRegistrar records every Register/Deregister call for assertions.
type staticRegistrar struct {
	registered   []Instance
	deregistered []Instance
}

func (r *staticRegistrar) Register(_ context.Context, inst Instance) error {
	r.registered = append(r.registered, inst)
	return nil
}

func (r *staticRegistrar) Deregister(_ context.Context, inst Instance) error {
	r.deregistered = append(r.deregistered, inst)
	return nil
}

func TestRegisterRegistrarAndGet(t *testing.T) {
	r := &staticRegistrar{}
	RegisterRegistrar("test-registrar", r)

	got, err := GetRegistrar("test-registrar")
	if err != nil {
		t.Fatalf("GetRegistrar: %v", err)
	}
	if got != Registrar(r) {
		t.Fatalf("GetRegistrar returned %v, want the exact registrar registered", got)
	}
}

func TestGetRegistrarNotFoundListsRegistered(t *testing.T) {
	// Register a sentinel so the diagnostic has a concrete name to list.
	RegisterRegistrar("test-registrar-notfound-sentinel", &staticRegistrar{})

	_, err := GetRegistrar("does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a missing registrar, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does-not-exist") {
		t.Fatalf("error should name the requested registrar: %v", err)
	}
	if !strings.Contains(msg, "test-registrar-notfound-sentinel") {
		t.Fatalf("error should list registered registrars to make typos obvious: %v", err)
	}
}

func TestRegisterRegistrarPanics(t *testing.T) {
	if !panics("empty name", func() { RegisterRegistrar("", &staticRegistrar{}) }) {
		t.Error("registering with an empty name should panic")
	}
	if !panics("nil Registrar", func() { RegisterRegistrar("test-registrar-nil", nil) }) {
		t.Error("registering a nil Registrar should panic")
	}
	RegisterRegistrar("test-registrar-dup", &staticRegistrar{})
	if !panics("already registered", func() {
		RegisterRegistrar("test-registrar-dup", &staticRegistrar{})
	}) {
		t.Error("registering a duplicate name should panic")
	}
}

// TestRegisterDeregisterRoundTrip exercises the contract's round-trip: Register,
// Deregister, then Deregister again — the last must be a safe no-op, since
// Deregister is documented as callable for an instance that is not registered.
func TestRegisterDeregisterRoundTrip(t *testing.T) {
	r := &staticRegistrar{}
	reg := Instance{ServiceName: "svc", ID: "i-1", Addr: "10.0.0.1:80"}

	if err := r.Register(context.Background(), reg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := r.Deregister(context.Background(), reg); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if err := r.Deregister(context.Background(), reg); err != nil {
		t.Fatalf("idempotent Deregister (instance not registered): %v", err)
	}
	if len(r.registered) != 1 || len(r.deregistered) != 2 {
		t.Fatalf("recorded calls = registered %d / deregistered %d, want 1 / 2",
			len(r.registered), len(r.deregistered))
	}
}

// TestReRegisterIsTheUpdate verifies the documented idempotent-refresh rule:
// there is no Update method — re-Registering with the same ID refreshes the
// live entry. The graceful-shutdown sequence Register(serving) →
// re-Register(Disabled=true) to drain → Deregister uses only that.
func TestReRegisterIsTheUpdate(t *testing.T) {
	r := &staticRegistrar{}
	inst := Instance{ServiceName: "svc", ID: "i-1", Addr: "10.0.0.1:80", Scheme: "tls"}

	if err := r.Register(context.Background(), inst); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Announce drain by re-registering the SAME identity with Disabled flipped on.
	drain := inst
	drain.Disabled = true
	if err := r.Register(context.Background(), drain); err != nil {
		t.Fatalf("re-Register (drain): %v", err)
	}

	if err := r.Deregister(context.Background(), inst); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	// Two Register calls (serve + drain refresh), one Deregister.
	if len(r.registered) != 2 || len(r.deregistered) != 1 {
		t.Fatalf("call counts = register %d / deregister %d, want 2/1",
			len(r.registered), len(r.deregistered))
	}
	if !r.registered[1].Disabled {
		t.Fatal("the drain re-Register should have set Disabled = true on the refreshed record")
	}
	if r.registered[1].ID != inst.ID {
		t.Fatalf("drain re-Register changed identity: got ID %q, want %q", r.registered[1].ID, inst.ID)
	}
}
