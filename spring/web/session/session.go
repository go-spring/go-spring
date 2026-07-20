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

// Package session defines a framework-agnostic, zero-dependency abstraction for
// server-side HTTP sessions — the Spring Session equivalent expressed in Go
// idioms rather than a port of its @EnableRedisHttpSession machinery.
//
// A stateful web / SSO deployment needs a session's attributes to survive across
// replicas: replica A writes, replica B reads. This package splits that concern
// into three pieces so any of them can be swapped independently:
//
//   - [Session] is the server-side state: an id, a bag of attributes, and a
//     creation time. Business handlers read and mutate it via [FromContext].
//   - [SessionStore] is the persistence seam. The bundled [Memory] store has
//     zero third-party dependencies (single-node / tests); a starter contributes
//     a distributed backend (Redis, ...) behind the same interface, so switching
//     from single-node to shared storage changes no business code. Byte-oriented
//     backends implement the narrower [ByteStore] and are lifted with
//     [FromByteStore]. Named stores are shared through a package-level registry
//     ([Register]/[Get]/[MustGet]), with [Memory] registered as "memory".
//   - [Manager] is the HTTP seam. Its [Manager.Middleware] parses the session id
//     from the request cookie, loads the session into the request context, and
//     writes it back on the way out — the single place session transport lives,
//     mirroring the security and gateway middleware posture. It never touches the
//     IoC container.
package session

import (
	"sync"
	"time"
)

// Session is the server-side state behind one client. It is safe for concurrent
// use by a single request's handlers. The id is opaque and server-generated;
// callers never set it. A Session is obtained from the request context via
// [FromContext] — it is not constructed directly by business code.
type Session struct {
	mu        sync.RWMutex
	id        string
	attrs     map[string]any
	createdAt time.Time

	// State flags consumed by Manager at write-back time. They are not persisted.
	isNew    bool // true until the first Save assigns an id
	modified bool // an attribute was set/deleted this request
	invalid  bool // Invalidate was called; the session must be destroyed
	renew    bool // RenewID was called; rotate the id, keep the attributes
}

// newSession returns a fresh, empty session with no id yet. The id is assigned
// lazily by the Manager only if the session is actually used, so a visitor that
// never touches the session never creates a store entry.
func newSession() *Session {
	return &Session{attrs: map[string]any{}, createdAt: time.Now(), isNew: true}
}

// fromData rebuilds a persisted session. Backends call it (directly for [Memory],
// via [FromByteStore] for byte stores) in Load; isNew is false because the
// session already exists in the store.
func fromData(id string, d sessionData) *Session {
	attrs := d.Attributes
	if attrs == nil {
		attrs = map[string]any{}
	}
	return &Session{id: id, attrs: attrs, createdAt: d.CreatedAt}
}

// ID returns the session identifier. It is empty for a brand-new session until
// the first write-back assigns one.
func (s *Session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// CreatedAt reports when the session was first created.
func (s *Session) CreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.createdAt
}

// IsNew reports whether the session was created during this request (as opposed
// to loaded from the store).
func (s *Session) IsNew() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isNew
}

// Get returns the attribute stored under key and whether it was present.
func (s *Session) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.attrs[key]
	return v, ok
}

// Set stores val under key, marking the session dirty so it is persisted on
// write-back.
func (s *Session) Set(key string, val any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs[key] = val
	s.modified = true
}

// Delete removes key, marking the session dirty. Deleting an absent key is a
// no-op but still flags the session so an emptied session is persisted.
func (s *Session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attrs, key)
	s.modified = true
}

// Keys returns the attribute keys in unspecified order.
func (s *Session) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.attrs))
	for k := range s.attrs {
		keys = append(keys, k)
	}
	return keys
}

// Invalidate marks the session for destruction. On write-back the Manager
// deletes it from the store and expires the client cookie. Use it on logout.
func (s *Session) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalid = true
}

// RenewID rotates the session id while preserving its attributes. Call it right
// after a privilege change (e.g. login) to defeat session-fixation attacks: the
// id the client presented before authenticating is discarded and a fresh one is
// issued. The rotation happens on write-back.
func (s *Session) RenewID() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renew = true
}

// snapshot copies the persisted portion of the session for a store to serialize.
func (s *Session) snapshot() sessionData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	attrs := make(map[string]any, len(s.attrs))
	for k, v := range s.attrs {
		attrs[k] = v
	}
	return sessionData{Attributes: attrs, CreatedAt: s.createdAt}
}
