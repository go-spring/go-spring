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

package fault

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/resilience"
)

// noopExec is a minimal inner executor for testing WrapExecutor composition: it
// runs fn once and records that it did.
type noopExec struct{ ran int }

func (n *noopExec) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	n.ran++
	return fn(ctx)
}
func (n *noopExec) Close() error                      { return nil }
func (n *noopExec) Refresh(_ resilience.Policy) error { return nil }

// resetInjector clears the process-wide injector so tests are independent.
func resetInjector() { RegisterInjector(nil) }

func TestInjectorFor_RegisterRoundTrip(t *testing.T) {
	t.Cleanup(resetInjector)

	if got := InjectorFor(); got != nil {
		t.Fatalf("InjectorFor before register: want nil, got %v", got)
	}
	inj := NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"})
	RegisterInjector(inj)
	if got := InjectorFor(); got != inj {
		t.Fatalf("InjectorFor after register: want %p, got %p", inj, got)
	}
	RegisterInjector(nil)
	if got := InjectorFor(); got != nil {
		t.Fatalf("InjectorFor after nil register: want nil, got %v", got)
	}
}

// TestWrapExecutor_LazyGlobalInjector is the regression test for the ordering
// fix: a starter wires fault.WrapExecutor(inner) in its
// Init, BEFORE starter-govern has registered the injector (Runner time). The
// captured injector is nil, so the fault layer must resolve InjectorFor() lazily
// on each Execute — otherwise fault never applies.
func TestWrapExecutor_LazyGlobalInjector(t *testing.T) {
	t.Cleanup(resetInjector)

	inner := &noopExec{}
	// Simulate wiring at Init time: InjectorFor() is nil (not registered yet).
	exec := WrapExecutor(inner)
	if exec == nil {
		t.Fatal("WrapExecutor(inner) returned nil; want a lazy faultExecutor")
	}

	// Before an injector is registered, Execute is a transparent pass-through.
	ctx := context.Background()
	want := errors.New("boom")
	if err := exec.Execute(ctx, "redis", func(context.Context) error { return want }); err != want {
		t.Fatalf("Execute with no injector: want %v passthrough, got %v", want, err)
	}

	// Now starter-govern registers an always-inject injector at Runner time.
	RegisterInjector(NewInjector(Config{Enabled: true, Rate: 1, Error: "generic"}))
	err := exec.Execute(ctx, "redis", func(context.Context) error {
		t.Fatal("fn should not run under a 100% inject fault")
		return nil
	})
	if err == nil || !IsInjected(err) {
		t.Fatalf("Execute after injector registered: want injected error, got %v", err)
	}
}

// TestWrapExecutor_ExplicitInjector is the explicit path tests/cloud-loadtest
// use: a hand-built injector passed directly still injects.
func TestWrapExecutor_ExplicitInjector(t *testing.T) {
	t.Cleanup(resetInjector)
	inner := &noopExec{}
	inj := NewInjector(Config{Enabled: true, Rate: 1, Error: "reset"})
	exec := WrapExecutorWith(inner, inj)
	err := exec.Execute(context.Background(), "redis", func(context.Context) error {
		t.Fatal("fn should not run")
		return nil
	})
	if err == nil || !IsInjected(err) {
		t.Fatalf("Execute with explicit injector: want injected error, got %v", err)
	}
}
