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

// counters.go caches one rate-limit counter per (kind, scope) and bounds that
// cache. Counters themselves are CREATED BY THE CONTAINER'S FACTORY, the same
// path every other caller uses, so the gateway owns only their lifetime — not
// which algorithm a kind names.
//
// The cache is needed because a budget is keyed by route, or route|client-ip:
// an unbounded key space that must evict idle entries. A shared store
// contributed as a limiter factory covers every replica instead and is reached
// through the same factory.

package StarterGateway

import (
	"sync"
	"time"

	"go-spring.org/cloud/governance/capability/ratelimit"
)

// maxScopes bounds how many keys the cache tracks before it starts evicting
// idle ones; the gateway keys budgets by things possibly unbounded (client
// IPs), so without a cap those keys would grow the map for the process's life.
const (
	maxScopes = 4096
	idleScope = 10 * time.Minute
)

// scopeStore caches the counter bound to a (kind, scope) pair. A changed rule
// reconfigures the cached counter in place, so an entry survives a policy push;
// only an idle entry is dropped.
type scopeStore struct {
	factory *ratelimit.Factory

	mu    sync.Mutex
	byKey map[string]*scopeCounter
}

type scopeCounter struct {
	limiter ratelimit.Limiter
	used    time.Time
}

func newScopeStore(factory *ratelimit.Factory) *scopeStore {
	return &scopeStore{factory: factory, byKey: map[string]*scopeCounter{}}
}

// limiter returns the counter for (kind, scope), building it through the
// factory on first use.
func (s *scopeStore) limiter(kind string, scope string) (ratelimit.Limiter, error) {
	key := kind + "\x00" + scope
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.byKey[key]; ok {
		c.used = now
		return c.limiter, nil
	}
	l, err := s.factory.NewLimiter(kind, scope)
	if err != nil {
		return nil, err
	}
	s.byKey[key] = &scopeCounter{limiter: l, used: now}
	if len(s.byKey) > maxScopes {
		s.evict(now)
	}
	return l, nil
}

// evict drops the keys that have gone idle, and then — if the cache is
// still over its cap, which only an unbounded key space can cause — arbitrary
// ones until it is under. Re-creating an evicted key starts a fresh counter.
func (s *scopeStore) evict(now time.Time) {
	for k, c := range s.byKey {
		if now.Sub(c.used) > idleScope {
			delete(s.byKey, k)
		}
	}
	for k := range s.byKey {
		if len(s.byKey) <= maxScopes {
			return
		}
		delete(s.byKey, k)
	}
}
