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

import "go-spring.org/stdlib/ordered"

// Manager is the process's directory of named [Discovery] backends — the object
// form of the container's discovery beans. Callers ask for "the backend this
// entry cites" by name instead of receiving the whole map, so the lookup and
// its miss live in one place rather than being re-written by every client
// starter.
//
// There is one Manager per process. The container builds it from every
// discovery.Discovery bean, and the runtime composition root holds it and hands
// it out as one of its authorities ([governance.Runtime.Discovery]), so a client
// reaches discovery through the same injection point it reaches resilience,
// fault and loadbalance. The control plane neither holds nor dispatches it: no
// governance rule applies.
//
// The Manager is immutable once built: [NewManager] captures the map as-is and
// the caller must not mutate it afterwards. That contract is what makes reads
// lock-free and safe for concurrent use.
type Manager struct {
	backends map[string]Discovery
}

// NewManager returns a Manager over backends, keyed by bean name (the name a
// client cites, e.g. "etcd.main"). A nil map is a valid "no backend configured"
// manager: every lookup misses. The map is captured as-is, so the caller must
// not mutate it after this call.
func NewManager(backends map[string]Discovery) *Manager {
	return &Manager{backends: backends}
}

// Get returns the backend registered under name. ok is false when no backend
// carries that name — including the empty name and a Manager over a nil map.
func (m *Manager) Get(name string) (Discovery, bool) {
	d, ok := m.backends[name]
	return d, ok
}

// Names returns the configured backend names in sorted order — for
// diagnostics, and for a caller that must report which names it could have
// cited. The result is a new slice; the Manager's map is never exposed.
func (m *Manager) Names() []string {
	return ordered.MapKeys(m.backends)
}
