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

package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-spring.org/cloud/session"
	"go-spring.org/stdlib/testing/assert"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := session.NewMemory()

	_, ok, err := store.Load(ctx, "missing")
	assert.That(t, err).Nil()
	assert.That(t, ok).False()
}

func TestRegistryMemoryDefault(t *testing.T) {
	s, err := session.GetStore("memory")
	assert.Error(t, err).Nil()
	assert.That(t, s).NotNil()

	_, err = session.GetStore("nope")
	assert.Error(t, err)
}

func TestRegisterGuards(t *testing.T) {
	assert.Panic(t, func() { session.Register("", session.NewMemory()) }, "empty name")
	assert.Panic(t, func() { session.Register("x", nil) }, "nil store")
	assert.Panic(t, func() { session.Register("memory", session.NewMemory()) }, "already registered")
}

// newSessionValue drives a session through the middleware to obtain a *Session
// under a store, returning the recorder so the caller can inspect the cookie.
func TestMiddlewareCreatesAndReuses(t *testing.T) {
	store := session.NewMemory()
	mgr := session.NewManager(store, session.Options{IdleTimeout: time.Minute})

	// First request sets an attribute -> a cookie is issued and the session is
	// persisted.
	h := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext(r.Context())
		assert.That(t, ok).True()
		if _, has, _ := session.Get[string](s, "user"); !has {
			session.Set(s, "user", "alice")
		}
		if v, has, _ := session.Get[string](s, "user"); has {
			_, _ = w.Write([]byte(v))
		}
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.That(t, rec.Body.String()).Equal("alice")

	cookie := rec.Result().Cookies()[0]
	assert.That(t, cookie.Value).NotEqual("")
	assert.That(t, cookie.HttpOnly).True()

	// Second request carries the cookie -> the same session (attribute survives).
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	assert.That(t, rec2.Body.String()).Equal("alice")
}

func TestMiddlewareAnonymousNoCookie(t *testing.T) {
	store := session.NewMemory()
	mgr := session.NewManager(store, session.Options{})

	h := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // never touches the session
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.That(t, len(rec.Result().Cookies())).Equal(0)
}

func TestCrossManagerSharing(t *testing.T) {
	// Two managers over one store model two replicas. What A writes, B reads.
	store := session.NewMemory()
	a := session.NewManager(store, session.Options{IdleTimeout: time.Minute})
	b := session.NewManager(store, session.Options{IdleTimeout: time.Minute})

	write := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		session.Set(s, "k", "v")
	}))
	read := b.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		if v, has, _ := session.Get[string](s, "k"); has {
			_, _ = w.Write([]byte(v))
		}
	}))

	rec := httptest.NewRecorder()
	write.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/set", nil))
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/get", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	read.ServeHTTP(rec2, req)
	assert.That(t, rec2.Body.String()).Equal("v")
}

func TestRenewIDPreventsFixation(t *testing.T) {
	store := session.NewMemory()
	mgr := session.NewManager(store, session.Options{IdleTimeout: time.Minute})

	set := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		session.Set(s, "user", "bob")
	}))
	rec := httptest.NewRecorder()
	set.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	oldCookie := rec.Result().Cookies()[0]

	// A "login" request rotates the id while keeping attributes.
	login := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		s.RenewID()
		v, _, _ := session.Get[string](s, "user")
		_, _ = w.Write([]byte(v))
	}))
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(oldCookie)
	rec2 := httptest.NewRecorder()
	login.ServeHTTP(rec2, req)

	newCookie := rec2.Result().Cookies()[0]
	assert.That(t, newCookie.Value).NotEqual(oldCookie.Value)
	assert.That(t, rec2.Body.String()).Equal("bob")

	// The old id must be gone from the store.
	_, ok, _ := store.Load(context.Background(), oldCookie.Value)
	assert.That(t, ok).False()
}

func TestInvalidateDestroys(t *testing.T) {
	store := session.NewMemory()
	mgr := session.NewManager(store, session.Options{IdleTimeout: time.Minute})

	set := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		session.Set(s, "k", "v")
	}))
	rec := httptest.NewRecorder()
	set.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := rec.Result().Cookies()[0]

	logout := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		s.Invalidate()
	}))
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	logout.ServeHTTP(rec2, req)

	// Cookie is expired and the store entry is gone.
	assert.That(t, rec2.Result().Cookies()[0].MaxAge).Equal(-1)
	_, ok, _ := store.Load(context.Background(), cookie.Value)
	assert.That(t, ok).False()
}

func TestSlidingRenewalAndExpiry(t *testing.T) {
	store := session.NewMemory()
	mgr := session.NewManager(store, session.Options{IdleTimeout: 40 * time.Millisecond})

	set := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		session.Set(s, "k", "v")
	}))
	touch := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		if v, has, _ := session.Get[string](s, "k"); has {
			_, _ = w.Write([]byte(v))
		}
	}))

	rec := httptest.NewRecorder()
	set.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := rec.Result().Cookies()[0]

	// Access within the window twice: sliding renewal keeps it alive.
	for range 3 {
		time.Sleep(25 * time.Millisecond)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookie)
		r := httptest.NewRecorder()
		touch.ServeHTTP(r, req)
		assert.That(t, r.Body.String()).Equal("v")
	}

	// Now let it sit idle past the timeout: it expires.
	time.Sleep(60 * time.Millisecond)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	r := httptest.NewRecorder()
	touch.ServeHTTP(r, req)
	assert.That(t, r.Body.String()).Equal("")
}

func TestFromByteStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := session.FromByteStore(newMapByteStore())
	mgr := session.NewManager(store, session.Options{IdleTimeout: time.Minute})

	set := mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		session.Set(s, "n", 42)
	}))
	rec := httptest.NewRecorder()
	set.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := rec.Result().Cookies()[0]

	s, ok, err := store.Load(ctx, cookie.Value)
	assert.That(t, err).Nil()
	assert.That(t, ok).True()
	v, has, err := session.Get[int](s, "n")
	assert.That(t, err).Nil()
	assert.That(t, has).True()
	assert.That(t, v).Equal(42)
}

// mapByteStore is an in-memory ByteStore used only to exercise FromByteStore's
// JSON path in tests.
type mapByteStore struct {
	m map[string][]byte
}

func newMapByteStore() *mapByteStore { return &mapByteStore{m: map[string][]byte{}} }

func (s *mapByteStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	b, ok := s.m[id]
	return b, ok, nil
}

func (s *mapByteStore) Set(_ context.Context, id string, data []byte, _ time.Duration) error {
	s.m[id] = data
	return nil
}

func (s *mapByteStore) Delete(_ context.Context, id string) error {
	delete(s.m, id)
	return nil
}
