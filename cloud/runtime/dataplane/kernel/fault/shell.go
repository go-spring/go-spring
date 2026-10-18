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
	"sync/atomic"

	"go-spring.org/cloud/runtime/dataplane/kernel/traffic"
)

// Manager is the direction-neutral fault authority: it owns one document,
// builds the kernel from it, and pushes the new kernel into every shell
// registered with it.
//
// The direction packages (outbound/fault and inbound/fault) embed one Manager
// each and expose it under their own authority type, so a client starter can
// only reach the outbound object and a server starter only the inbound one.
// This type exists so both directions share the bookkeeping without sharing an
// API: direction isolation is a property of the two wrappers, not of the
// document or the injection sequence.
type Manager struct {
	dir  direction
	prop traffic.Propagator
}

// NewManager returns an unarmed authority: it carries no rules, so it injects
// nothing until a document is pushed.
//
// prop is the load-test convention this manager hands to its shells when a
// rule's Scope distinguishes synthetic from real traffic.
func NewManager(prop traffic.Propagator) *Manager {
	return &Manager{dir: newDirection(), prop: prop}
}

// Refresh adopts c as this direction's document, with the reconciliation
// guarantees described on [buildCore]. It is safe to call concurrently with a
// gated call; each call observes the kernel current at its own snapshot, so a
// toggle takes effect on the next operation.
func (m *Manager) Refresh(c Config) { m.dir.refresh(c) }

// Config returns the document this authority currently runs (the zero Config
// when nothing has been pushed, which is a no-op).
func (m *Manager) Config() Config { return m.dir.config() }

// NewShell creates and registers a shell. A direction wrapper calls this once
// per consumer and closes the shell when the consumer goes away.
func (m *Manager) NewShell() *Shell {
	s := &Shell{dir: &m.dir, prop: m.prop}
	m.dir.register(s)
	return s
}

// Shell is one caller's handle on its direction's current kernel. Apart from
// the atomic kernel pointer — which [Manager.Refresh] pushes into — it is
// immutable, so the call path never consults the manager.
type Shell struct {
	dir  *direction
	prop traffic.Propagator
	core atomic.Pointer[core]
}

// setCore is the direction's push entry point; it is the only write to the
// kernel pointer.
func (s *Shell) setCore(k *core) { s.core.Store(k) }

// Snapshot captures the kernel current at this moment, so a call that evaluates
// several attempts can hold one document for all of them (see
// [go-spring.org/cloud/runtime/dataplane/outbound/fault]'s retry loop).
func (s *Shell) Snapshot() Snapshot {
	return Snapshot{core: s.core.Load(), prop: s.prop}
}

// Fire decides whether one call should be set on fire, loading the current
// kernel once and applying the configured latency.
func (s *Shell) Fire(ctx context.Context, t Target) (Decision, error) {
	return s.Snapshot().Decide(ctx, t)
}

// Close unregisters the shell from its manager. The kernel itself is shared and
// immutable, so nothing else is released — but a closed shell must stop
// receiving pushes, and the manager's registry must not retain it.
func (s *Shell) Close() { s.dir.unregister(s) }

// Snapshot is one call's immutable view of a direction's kernel: the document
// current when the call started. Every attempt of that call evaluates against
// the same Snapshot, which is what makes a retry's outcome comparable to the
// first try; a push landing mid-call takes effect on the next call.
type Snapshot struct {
	core *core
	prop traffic.Propagator
}

// Decide runs one attempt's decide-and-delay sequence. It does NOT call the
// operation: a nil error means the caller should run the real fn, anything else
// means it should return that error instead (an injected failure, or the
// context error when the injected latency was cancelled mid-sleep).
func (s Snapshot) Decide(ctx context.Context, t Target) (Decision, error) {
	return evaluate(s.core, s.prop, t, ctx)
}
