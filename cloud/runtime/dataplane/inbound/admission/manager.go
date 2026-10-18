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
	"reflect"
	"sync"
)

// Manager is the inbound configuration authority: it holds the pushed
// [Settings] and the registry of Guards built for it, and pushes each Guard its
// own resolved policy when the document moves.
//
// It is deliberately NOT a state registry: a Guard owns its stages and their
// state, and the Manager never resolves a request or shares one Guard's breaker
// with another. Two Guards created for the same label are two independent
// instances — that is the point, not an accident.
//
// Safe for concurrent use.
type Manager struct {
	mu       sync.Mutex
	settings Settings
	guards   map[*server]struct{}
	closed   bool

	// opts are the process-wide construction options (a cluster limiter, a
	// fault stage); each Guard starts from them and a caller may add more.
	opts options
}

// NewManager returns an unarmed inbound manager. Every Guard it creates before
// the first Apply resolves the zero [Settings], i.e. the pass-through.
//
// opts apply to every Guard this manager creates; a caller can add per-instance
// options through [Manager.GuardFor].
func NewManager(opts ...Option) *Manager {
	o := options{}
	for _, f := range opts {
		f(&o)
	}
	return &Manager{guards: make(map[*server]struct{}), opts: o}
}

// Apply adopts s as the inbound document and pushes each registered Guard its
// resolved policy. A document identical to the current one is a no-op: a
// redundant push does not reset a live breaker.
func (m *Manager) Apply(s Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || reflect.DeepEqual(m.settings, s) {
		return nil
	}
	m.settings = s
	for g := range m.guards {
		p, rule := resolvePolicy(s, g.id.Service)
		g.apply(p, rule)
	}
	return nil
}

// GuardFor creates the Guard for one server instance and registers it for
// future pushes. The Guard snapshots the current settings at creation, so a
// Guard wired after Apply still starts under the live document.
func (m *Manager) GuardFor(id Identity, opts ...Option) Guard {
	m.mu.Lock()
	defer m.mu.Unlock()
	o := m.opts
	for _, f := range opts {
		f(&o)
	}
	p, rule := resolvePolicy(m.settings, id.Service)
	s := &server{id: id, opts: o, mgr: m}
	s.st = buildState(p, rule, id.Service, nil, o.limiter)
	if m.closed {
		s.closed = true
	} else {
		m.guards[s] = struct{}{}
	}
	return s
}

// Close closes every registered Guard and empties the registry. It does not
// reset the Settings: a manager that is closed is done.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	guards := make([]*server, 0, len(m.guards))
	for g := range m.guards {
		guards = append(guards, g)
	}
	m.guards = make(map[*server]struct{})
	m.mu.Unlock()

	for _, g := range guards {
		g.closeLocal()
	}
	return nil
}

// PolicyFor returns the policy the label currently resolves to: the first
// matching Rule, else Default. It is the read-side counterpart of the push, for
// a caller that needs the policy without building a Guard (an adapter mapping
// knobs, a test).
func (m *Manager) PolicyFor(label string) Policy {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, _ := resolvePolicy(m.settings, label)
	return p
}

// unregister removes a Guard that closed itself.
func (m *Manager) unregister(s *server) {
	m.mu.Lock()
	delete(m.guards, s)
	m.mu.Unlock()
}

// resolvePolicy is the inbound matching rule: the first Rule whose Service
// equals the instance label, else the default. It returns the matched rule's
// label so a Result can name it.
func resolvePolicy(s Settings, service string) (Policy, string) {
	for _, r := range s.Rules {
		if r.Service == service {
			return r.Policy, r.Service
		}
	}
	return s.Default, ""
}
