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

package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SessionStore persists sessions keyed by id. Implementations must be safe for
// concurrent use. A [Manager] talks only to this interface, so the same
// middleware serves an in-process [Memory] store or a shared Redis backend
// without change.
type SessionStore interface {
	// Load returns the session stored under id and whether it was present (and
	// not expired). A backend error is returned with found=false.
	Load(ctx context.Context, id string) (s *Session, found bool, err error)

	// Save persists s under its id for the given ttl (the idle timeout). A
	// non-positive ttl means the entry does not expire. Saving refreshes the
	// expiry, which is how sliding renewal is implemented.
	Save(ctx context.Context, s *Session, ttl time.Duration) error

	// Delete removes the session under id. Deleting an absent id is not an error.
	Delete(ctx context.Context, id string) error
}

// sessionData is the persisted portion of a [Session]. It is what byte-oriented
// stores serialize; because attributes are untyped, a value round-trips through
// the store's encoding (JSON for [FromByteStore]), so keep remotely stored
// attributes JSON-friendly.
type sessionData struct {
	Attributes map[string]any `json:"attributes"`
	CreatedAt  time.Time      `json:"createdAt"`
}

// ByteStore is the narrow byte-oriented backend that remote session stores
// expose natively (Redis GET/SET/DEL, ...). A starter implements this single
// interface over its client and lifts it to a full [SessionStore] with
// [FromByteStore], so every backend shares one serialization path instead of
// re-deriving it. Implementations must be safe for concurrent use.
type ByteStore interface {
	Get(ctx context.Context, id string) (data []byte, found bool, err error)
	Set(ctx context.Context, id string, data []byte, ttl time.Duration) error
	Delete(ctx context.Context, id string) error
}

// byteSessionStore adapts a [ByteStore] into a [SessionStore] by encoding the
// session's persisted data as JSON.
type byteSessionStore struct {
	store ByteStore
}

// FromByteStore lifts a byte-oriented backend to a full [SessionStore], encoding
// sessions as JSON. This is the single entry point a Redis (or other remote)
// starter uses to expose a session.SessionStore.
func FromByteStore(store ByteStore) SessionStore {
	return &byteSessionStore{store: store}
}

func (b *byteSessionStore) Load(ctx context.Context, id string) (*Session, bool, error) {
	data, ok, err := b.store.Get(ctx, id)
	if err != nil || !ok {
		return nil, false, err
	}
	var d sessionData
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, false, err
	}
	return fromData(id, d), true, nil
}

func (b *byteSessionStore) Save(ctx context.Context, s *Session, ttl time.Duration) error {
	data, err := json.Marshal(s.snapshot())
	if err != nil {
		return err
	}
	return b.store.Set(ctx, s.ID(), data, ttl)
}

func (b *byteSessionStore) Delete(ctx context.Context, id string) error {
	return b.store.Delete(ctx, id)
}

var (
	mu       sync.RWMutex
	registry = map[string]SessionStore{}
)

// Register makes a SessionStore available under name. It panics if name is
// empty, s is nil, or name is already registered, mirroring the driver-registry
// idiom used elsewhere (cache.Register, discovery.Register) so duplicate wiring
// fails loudly at init.
//
// The registry is meant for process-static stores that can be shared by name —
// the bundled [Memory] is registered as "memory". A distributed backend that
// needs a live client (Redis, ...) is instead contributed as a bean behind the
// SessionStore interface, the same way starter-lock-redis contributes a Locker;
// registering a per-application live connection into a package-global map would
// be wrong across tests and restarts.
func Register(name string, s SessionStore) {
	if name == "" {
		panic("session: register with empty name")
	}
	if s == nil {
		panic("session: register nil store for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("session: store already registered: " + name)
	}
	registry[name] = s
}

// Get returns the SessionStore registered under name.
func Get(name string) (SessionStore, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := registry[name]
	return s, ok
}

// MustGet returns the SessionStore registered under name, or an error that lists
// the available stores when none matches.
func MustGet(name string) (SessionStore, error) {
	if s, ok := Get(name); ok {
		return s, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("session: no store registered as %q (registered: %v)", name, names)
}
