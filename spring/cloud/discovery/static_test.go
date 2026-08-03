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

// staticDiscovery is an in-memory backend used only by tests in this package.
// It is the reference implementation of the package's watch contract: the
// current snapshot is delivered as the FIRST result on a Watch channel, later
// topology changes are pushed by Update, and the channel closes when the watch
// context is cancelled. It also satisfies the optional Catalog via a fixed name
// list.
type staticDiscovery struct {
	mu       sync.Mutex
	eps      map[string][]Endpoint
	names    []string
	watchers map[string][]chan WatchResult
}

func newStaticDiscovery(names ...string) *staticDiscovery {
	return &staticDiscovery{
		eps:      map[string][]Endpoint{},
		names:    names,
		watchers: map[string][]chan WatchResult{},
	}
}

// set replaces the endpoint snapshot stored for name.
func (s *staticDiscovery) set(name string, eps ...Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = eps
}

func (s *staticDiscovery) Resolve(_ context.Context, name string, opts ...Option) ([]Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return FilterByScheme(append([]Endpoint(nil), s.eps[name]...), NewQuery("", opts...).Scheme), nil
}

func (s *staticDiscovery) Watch(ctx context.Context, name string, opts ...Option) (<-chan WatchResult, error) {
	scheme := NewQuery("", opts...).Scheme
	s.mu.Lock()
	eps := FilterByScheme(append([]Endpoint(nil), s.eps[name]...), scheme)
	s.mu.Unlock()

	ch := make(chan WatchResult, 8)
	ch <- WatchResult{Endpoints: eps} // seed: the current snapshot, delivered first

	// Register only after the seed is queued, so a racing Update cannot push a
	// change ahead of the seed on this channel.
	s.mu.Lock()
	s.watchers[name] = append(s.watchers[name], ch)
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		// Remove before close, under the same lock Update sends under, so Update
		// can never send to a channel that is about to be (or already is) closed.
		s.mu.Lock()
		s.watchers[name] = removeChan(s.watchers[name], ch)
		s.mu.Unlock()
		close(ch)
	}()
	return ch, nil
}

// Update replaces the snapshot for name and pushes the new full set to every
// live watcher of name. It is how tests simulate a topology change.
func (s *staticDiscovery) Update(name string, eps ...Endpoint) {
	snap := append([]Endpoint(nil), eps...)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = snap
	for _, ch := range s.watchers[name] {
		select {
		case ch <- WatchResult{Endpoints: append([]Endpoint(nil), snap...)}:
		default:
			// Buffer full: drop rather than block the producer. Tests keep the
			// buffer small and consume promptly, so this is just a safety net.
		}
	}
}

func (s *staticDiscovery) Services(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.names...), nil
}

// removeChan returns list without ch (channel identity compared by address).
func removeChan(list []chan WatchResult, ch chan WatchResult) []chan WatchResult {
	for i, c := range list {
		if c == ch {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}
