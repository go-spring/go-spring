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
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"
)

// Options configures how a [Manager] carries the session id in the HTTP cookie
// and how long an idle session survives. The zero value is not usable; pass it
// to [NewManager], which fills sensible defaults for any zero field.
type Options struct {
	// CookieName is the name of the session cookie. Default "SESSION".
	CookieName string

	// Path scopes the cookie. Default "/", so the session travels with every
	// request to the host — required when different handlers (or replicas) mount
	// the middleware under different route prefixes.
	Path string

	// Domain scopes the cookie to a domain. Empty (default) means host-only.
	Domain string

	// Secure marks the cookie so browsers only send it over HTTPS.
	Secure bool

	// SameSite is the cookie's SameSite policy. Default http.SameSiteLaxMode.
	SameSite http.SameSite

	// IdleTimeout is how long a session may sit idle before it expires. Every
	// request that carries a session refreshes this deadline (sliding renewal).
	// Non-positive means the session never expires server-side and the cookie is
	// a session cookie (no Max-Age). Default 30m.
	IdleTimeout time.Duration
}

func (o *Options) normalize() {
	if o.CookieName == "" {
		o.CookieName = "SESSION"
	}
	if o.Path == "" {
		o.Path = "/"
	}
	if o.SameSite == 0 {
		o.SameSite = http.SameSiteLaxMode
	}
	if o.IdleTimeout == 0 {
		o.IdleTimeout = 30 * time.Minute
	}
}

// Manager loads and stores sessions over HTTP. It is the single seam where
// session transport lives; construct one with [NewManager] and mount
// [Manager.Middleware] in front of the handlers that use the session. Multiple
// Managers may share one [SessionStore] — that is exactly how replicas share
// session state.
type Manager struct {
	store SessionStore
	opt   Options
}

// NewManager builds a Manager over store with opt (defaults filled for zero
// fields).
func NewManager(store SessionStore, opt Options) *Manager {
	opt.normalize()
	return &Manager{store: store, opt: opt}
}

// Middleware returns an http.Handler that loads the session referenced by the
// request cookie (or prepares a fresh one), attaches it to the request context
// for next, and writes it back — creating, refreshing, rotating, or destroying
// the store entry and cookie — before the response headers are sent.
//
// A visitor who never touches the session creates no store entry and receives no
// cookie: the id is assigned lazily on the first Set.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sess *Session
		if id := m.readCookie(r); id != "" {
			if s, ok, err := m.store.Load(r.Context(), id); err == nil && ok {
				sess = s
			}
		}
		if sess == nil {
			sess = newSession()
		}

		sw := &sessionWriter{ResponseWriter: w, m: m, r: r, sess: sess}
		next.ServeHTTP(sw, r.WithContext(WithSession(r.Context(), sess)))
		// Handlers that never wrote a body (e.g. an empty 200) still need their
		// session persisted and cookie set.
		sw.commit()
	})
}

// readCookie returns the session id carried by the request, or "".
func (m *Manager) readCookie(r *http.Request) string {
	c, err := r.Cookie(m.opt.CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// setCookie writes the session cookie with the current id and idle Max-Age.
func (m *Manager) setCookie(w http.ResponseWriter, id string) {
	c := &http.Cookie{
		Name:     m.opt.CookieName,
		Value:    id,
		Path:     m.opt.Path,
		Domain:   m.opt.Domain,
		Secure:   m.opt.Secure,
		HttpOnly: true,
		SameSite: m.opt.SameSite,
	}
	if m.opt.IdleTimeout > 0 {
		c.MaxAge = int(m.opt.IdleTimeout.Seconds())
	}
	http.SetCookie(w, c)
}

// clearCookie expires the session cookie on the client.
func (m *Manager) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.opt.CookieName,
		Value:    "",
		Path:     m.opt.Path,
		Domain:   m.opt.Domain,
		Secure:   m.opt.Secure,
		HttpOnly: true,
		SameSite: m.opt.SameSite,
		MaxAge:   -1,
	})
}

// generateID returns a cryptographically-random, URL-safe session id. 32 bytes
// (256 bits) of entropy makes ids unguessable, defeating session-prediction.
func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// sessionWriter wraps the ResponseWriter so the session is written back exactly
// once, before the first header/byte reaches the client — Set-Cookie must precede
// the response body. Mutations made after the first write are not persisted, the
// same constraint any header carries.
type sessionWriter struct {
	http.ResponseWriter
	m         *Manager
	r         *http.Request
	sess      *Session
	committed bool
}

func (sw *sessionWriter) WriteHeader(code int) {
	sw.commit()
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *sessionWriter) Write(b []byte) (int, error) {
	sw.commit()
	return sw.ResponseWriter.Write(b)
}

// commit persists the session state decided during the request. It runs once:
//
//   - invalidated  -> delete the store entry (if any) and expire the cookie.
//   - modified, or an existing session (sliding renewal) -> assign an id if the
//     session is new, rotate it if RenewID was called, save with the idle ttl,
//     and (re)set the cookie.
//   - untouched new session -> nothing (no id, no store entry, no cookie).
func (sw *sessionWriter) commit() {
	if sw.committed {
		return
	}
	sw.committed = true

	m, ctx, s := sw.m, sw.r.Context(), sw.sess

	if s.invalid {
		if s.id != "" {
			_ = m.store.Delete(ctx, s.id)
		}
		m.clearCookie(sw.ResponseWriter)
		return
	}

	hadID := s.id != ""
	// Persist when the session changed, or when it already existed (each access
	// slides the idle deadline forward). A brand-new, untouched session is left
	// alone so anonymous traffic never allocates a session.
	if !s.modified && !hadID {
		return
	}

	if s.renew && hadID {
		_ = m.store.Delete(ctx, s.id)
		s.id = ""
		hadID = false
	}
	if s.id == "" {
		id, err := generateID()
		if err != nil {
			// Without a random id we cannot safely persist; skip write-back and
			// leave the client without a cookie rather than issue a weak id.
			return
		}
		s.id = id
	}
	if err := m.store.Save(ctx, s, m.opt.IdleTimeout); err != nil {
		// Mid-response: the headers are about to be written, so there is no way
		// to surface the error to the handler. Drop the cookie so we do not hand
		// the client an id that was never stored.
		if !hadID {
			return
		}
	}
	m.setCookie(sw.ResponseWriter, s.id)
}
