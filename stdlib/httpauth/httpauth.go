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

// Package httpauth provides a minimal HTTP authentication guard for
// management-plane endpoints (pprof, actuator, admin UIs): bearer-token or
// HTTP Basic authentication with constant-time credential compares, plus an
// IsLoopback helper to decide when an unauthenticated listener is acceptable.
// It deliberately supports nothing else - no sessions, no JWTs, no scopes.
package httpauth

import (
	"crypto/subtle"
	"net"
	"net/http"
)

// Guard is a static HTTP authentication configuration: either a bearer
// Token, or HTTP Basic Username+Password. Token takes precedence when both
// are set. The zero value disables authentication and [Guard.Wrap] returns
// the wrapped handler unchanged.
type Guard struct {
	// Token, when set, requires an "Authorization: Bearer <token>" header
	// on every request. Takes precedence over Username/Password.
	Token string

	// Username and Password, when both set, require HTTP Basic
	// authentication.
	Username string
	Password string
}

// Enabled reports whether any authentication scheme is configured.
func (g Guard) Enabled() bool {
	return g.Token != "" || (g.Username != "" && g.Password != "")
}

// Wrap guards next with the configured authentication scheme. Requests that
// fail the check get a 401; Basic-mode failures also get a
// "WWW-Authenticate: Basic" challenge. With no scheme configured it returns
// next unchanged.
func (g Guard) Wrap(next http.Handler) http.Handler {
	if !g.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.Token != "" {
			if !bearerMatches(r, g.Token) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		} else {
			user, pass, ok := r.BasicAuth()
			if !ok || !constantTimeEqual(user, g.Username) || !constantTimeEqual(pass, g.Password) {
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// bearerMatches reports whether the request carries the expected token in
// an "Authorization: Bearer <token>" header.
func bearerMatches(r *http.Request, want string) bool {
	got := r.Header.Get("Authorization")
	const prefix = "Bearer "
	return len(got) > len(prefix) && got[:len(prefix)] == prefix &&
		constantTimeEqual(got[len(prefix):], want)
}

// constantTimeEqual compares two strings without leaking their length
// relationship through timing.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// IsLoopback reports whether addr binds only to a loopback interface. An
// empty or wildcard host (":9981", "0.0.0.0:9981") is treated as
// non-loopback, since such listeners accept off-host traffic.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost"
}
