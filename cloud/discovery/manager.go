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

import "sort"

// Manager is the process's directory of named [Discovery] backends — the object
// form of the container's discovery beans. A caller that needs "the backend this
// entry cites" asks for it by label instead of receiving the whole map, so the
// label -> backend lookup and its miss live in one place rather than being
// re-written by every client starter.
//
// There is one Manager per process: the container builds it from every
// discovery.Discovery bean, and the governance center holds it and hands it out
// as one of its authorities ([governance.Center.Discovery]), so a client injects
// the center and reaches discovery the same way it reaches resilience, fault and
// loadbalance.
//
// Immutable once built — the container assembles it at wiring time and nothing
// mutates it afterwards — so it is safe for concurrent use without locking.
type Manager struct {
	backends map[string]Discovery
}

// NewManager returns a Manager over backends, keyed by bean name (the label a
// client cites, e.g. "etcd.main"). A nil map is a valid "no backend configured"
// manager: every lookup misses. The map is captured as-is.
func NewManager(backends map[string]Discovery) *Manager {
	return &Manager{backends: backends}
}

// Get returns the backend registered under label and whether one exists.
// Nil-receiver safe, so a container-less caller holding a nil Manager reads a
// miss rather than a false hit.
func (m *Manager) Get(label string) (Discovery, bool) {
	if m == nil {
		return nil, false
	}
	d, ok := m.backends[label]
	return d, ok
}

// Labels returns every configured backend label in sorted order — for
// diagnostics, and for a caller that must report which labels it could have
// cited. Nil-receiver safe.
func (m *Manager) Labels() []string {
	if m == nil || len(m.backends) == 0 {
		return nil
	}
	out := make([]string, 0, len(m.backends))
	for l := range m.backends {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}
