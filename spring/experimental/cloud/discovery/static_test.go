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
	"sync"
)

// staticDiscovery is a minimal in-memory Discovery used by tests. It serves a
// fixed set of endpoints per name and can push updated snapshots to live
// watchers via Update.
type staticDiscovery struct {
	mu       sync.Mutex
	eps      map[string][]Endpoint
	watchers map[string][]*staticWatcher
}

func newStaticDiscovery() *staticDiscovery {
	return &staticDiscovery{
		eps:      map[string][]Endpoint{},
		watchers: map[string][]*staticWatcher{},
	}
}

func (s *staticDiscovery) set(name string, eps ...Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = eps
}

func (s *staticDiscovery) Resolve(_ context.Context, name string) ([]Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eps[name], nil
}

func (s *staticDiscovery) Watch(_ context.Context, name string) (Watcher, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := &staticWatcher{ch: make(chan []Endpoint, 8), done: make(chan struct{})}
	s.watchers[name] = append(s.watchers[name], w)
	return w, nil
}

// Update replaces the snapshot for name and notifies live watchers.
func (s *staticDiscovery) Update(name string, eps ...Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = eps
	for _, w := range s.watchers[name] {
		select {
		case w.ch <- eps:
		case <-w.done:
		}
	}
}

type staticWatcher struct {
	ch       chan []Endpoint
	done     chan struct{}
	stopOnce sync.Once
}

func (w *staticWatcher) Next() ([]Endpoint, error) {
	select {
	case eps := <-w.ch:
		return eps, nil
	case <-w.done:
		return nil, context.Canceled
	}
}

func (w *staticWatcher) Stop() error {
	w.stopOnce.Do(func() { close(w.done) })
	return nil
}
