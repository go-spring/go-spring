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
	"go-spring.org/cloud/observability"
	"sync"
)

// serverSide is the manager's inbound half: the resolver for inbound policies
// and the registry of inbound executors. It has no subscriber table — nothing
// outside this package observes inbound changes.
type serverSide struct {
	// resolve maps a route label to the inbound policy it runs under, or nil
	// when this side is unarmed (a disabled center, or a center that configures no
	// inbound), in which case every route falls to the pass-through.
	resolve func(label string) ServerPolicy

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

// policyLocked is [clientSide.policyLocked] for the inbound side.
func (s *serverSide) policyLocked(label string) ServerPolicy {
	if s.resolve == nil {
		return ServerPolicy{}
	}
	return s.resolve(label)
}

// backing is [clientSide.backing] for the inbound side: the same lazy build and
// registration, over the inbound model and the inbound driver method.
func (s *serverSide) backing(m *Manager, label, system string) chain.Executor {
	if v, ok := s.entries.Load(label); ok {
		return v.(*serverEntry).exec
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if v, ok := s.entries.Load(label); ok { // built while we waited
		return v.(*serverEntry).exec
	}
	return s.buildLocked(m, label, system)
}

// buildLocked is [clientSide.buildLocked] for the inbound side, with the same
// re-read rule and for the same reason: an inbound executor built against a
// superseded model must not be published, because the sweep that would have
// dropped it has already run.
func (s *serverSide) buildLocked(m *Manager, label, system string) chain.Executor {
	for {
		m.mu.Lock()
		resolve := s.resolve
		m.mu.Unlock()
		policy := ServerPolicy{}
		if resolve != nil {
			policy = resolve(label)
		}
		e := chain.Executor(noopServerExecutor{})
		if resolve != nil {
			if d, err := m.driverFor(m.Driver()); err == nil && d != nil {
				if built, err := d.NewServerExecutor(label, policy); err == nil && built != nil {
					e = built
				}
			}
		}
		m.mu.Lock()
		armed := s.resolve != nil
		current := s.policyLocked(label)
		m.mu.Unlock()
		if armed != (resolve != nil) || current != policy {
			continue
		}
		registered := observability.WrapServerExecutor(e, system, label)
		s.entries.Store(label, &serverEntry{exec: registered, policy: policy})
		return registered
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
}

func (e *managedServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return e.mgr.server.backing(e.mgr, e.label, e.system).Execute(ctx, fn)
}

func (e *managedServerExecutor) Close() error { return nil }

// noopServerExecutor is the inbound counterpart of [noopClientExecutor]: the
// pass-through a route resolves to when inbound is unarmed, so the middleware
// chain is installed unconditionally.
type noopServerExecutor struct{}

func (noopServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (noopServerExecutor) Close() error { return nil }
