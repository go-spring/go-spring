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
	"go-spring.org/cloud/chain"
	"sync"
)

// serverSide is the manager's inbound half: the pushed inbound settings and the
// registry of inbound executors. It has no subscriber table — nothing outside
// this package observes inbound changes.
type serverSide struct {
	// settings is the inbound half last pushed by [Manager.Apply], held whole. Its
	// zero value is the pass-through — the state before any Apply, after a Close,
	// and what a document that governs only outbound traffic looks like.
	settings ServerSettings

	// entries registers every inbound executor this side has built, keyed by the
	// route label, each with the policy it was built from — the inbound twin of
	// [clientSide.entries], with the same change semantics.
	entries sync.Map // label -> *serverEntry

	// buildMu serializes the build path, exactly as [clientSide.buildMu] does.
	buildMu sync.Mutex
}

// serverEntry is [clientEntry] for the inbound side.
type serverEntry struct {
	exec   chain.Executor
	policy ServerPolicy
}

// policyLocked is [clientSide.policyLocked] for the inbound side: the first Rule
// whose Service equals the label, else the default. Callers hold the manager
// lock.
func (s *serverSide) policyLocked(label string) ServerPolicy {
	for _, r := range s.settings.Rules {
		if r.Service == label {
			return r.ServerPolicy
		}
	}
	return s.settings.Default
}

// backing is [clientSide.backing] for the inbound side: the same lazy build and
// registration, over the inbound model and the inbound driver method.
func (s *serverSide) backing(m *Manager, label, system string, listener BreakerEventListener) chain.Executor {
	if v, ok := s.entries.Load(label); ok {
		return v.(*serverEntry).exec
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if v, ok := s.entries.Load(label); ok { // built while we waited
		return v.(*serverEntry).exec
	}
	return s.buildLocked(m, label, system, listener)
}

// buildLocked is [clientSide.buildLocked] for the inbound side, with the same
// re-read rule and for the same reason: an inbound executor built against a
// superseded model must not be published, because the sweep that would have
// dropped it has already run.
func (s *serverSide) buildLocked(m *Manager, label, system string, listener BreakerEventListener) chain.Executor {
	_ = system // the system label is the observer's concern; this engine keys on label alone
	for {
		m.mu.Lock()
		active := m.applied
		policy := s.policyLocked(label)
		m.mu.Unlock()
		e := chain.Executor(noopServerExecutor{})
		if active {
			if built, err := m.driver.NewServerExecutor(label, policy); err == nil && built != nil {
				e = built
			}
		}
		m.mu.Lock()
		activeNow := m.applied
		current := s.policyLocked(label)
		m.mu.Unlock()
		if activeNow != active || current != policy {
			continue
		}
		if listener != nil {
			if setter, ok := e.(BreakerEventListenerSetter); ok {
				setter.SetBreakerEventListener(listener)
			}
		}
		s.entries.Store(label, &serverEntry{exec: e, policy: policy})
		return e
	}
}

// evictChangedLocked is [clientSide.evictChangedLocked] for the inbound side.
func (s *serverSide) evictChangedLocked() {
	s.entries.Range(func(key, value any) bool {
		if v := value.(*serverEntry); v.policy != s.policyLocked(key.(string)) {
			s.entries.Delete(key)
		}
		return true
	})
}

// ServerExecutorFor returns the inbound executor the given system's inbound route
// should run under. It is the inbound counterpart of [Manager.ClientExecutorFor]:
// pass the owning system (e.g. "gin", "grpc") and the route label, and install the
// result in the inbound middleware chain.
//
// It mirrors ClientExecutorFor in every respect that matters — lazily bound
// backing, per-label registry, a policy change taking effect through
// [Manager.Apply] — but it
// resolves the label in the inbound side, so it reads the inbound policy and
// never the outbound one. The two directions never read each other's config, which
// is what lets an operator tune inbound without touching outbound call
// protection.
func (m *Manager) ServerExecutorFor(system, service string) chain.Executor {
	return &managedServerExecutor{mgr: m, system: system, label: service}
}

// ServerPolicyFor returns the inbound policy label currently resolves to, or a
// zero policy on an unarmed manager. It is the inbound twin of
// [Manager.ClientPolicyFor]: callers that only need to READ the policy — an
// adapter mapping it onto its own knobs, a test — use this instead of building an
// executor they will not use.
func (m *Manager) ServerPolicyFor(label string) ServerPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.server.policyLocked(label)
}

// drainServerCache empties the inbound registry and returns what was in it.
func drainServerCache(c *sync.Map) []chain.Executor {
	var out []chain.Executor
	c.Range(func(key, value any) bool {
		out = append(out, value.(*serverEntry).exec)
		c.Delete(key)
		return true
	})
	return out
}

// managedServerExecutor is the inbound counterpart of [managedClientExecutor]:
// [Manager.ServerExecutorFor] returns one, and each Execute resolves the backing
// inbound executor through the inbound registry.
type managedServerExecutor struct {
	mgr    *Manager
	system string
	label  string

	// listener is the observe wrapper's inbound breaker listener, forwarded to the
	// backing executor when it is built; see [managedClientExecutor.listener].
	listener BreakerEventListener
}

// SetBreakerEventListener satisfies [BreakerEventListenerSetter] for the observe
// wrapper composed around this handle.
func (e *managedServerExecutor) SetBreakerEventListener(l BreakerEventListener) {
	e.listener = l
}

func (e *managedServerExecutor) Execute(ctx context.Context, call chain.Operation, fn func(context.Context) error) error {
	return e.mgr.server.backing(e.mgr, e.label, e.system, e.listener).Execute(ctx, call, fn)
}

func (e *managedServerExecutor) Close() error { return nil }

// noopServerExecutor is the inbound counterpart of [noopClientExecutor]: the
// pass-through a route resolves to when inbound is unarmed, so the middleware
// chain is installed unconditionally.
type noopServerExecutor struct{}

func (noopServerExecutor) Execute(ctx context.Context, call chain.Operation, fn func(context.Context) error) error {
	return fn(ctx)
}

func (noopServerExecutor) Close() error { return nil }
