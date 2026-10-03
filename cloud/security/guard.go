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

// Guard is the management plane's static-credential authentication: the
// minimal HTTP guard for pprof, actuator and admin UI endpoints. It supports
// bearer-token or HTTP Basic authentication with constant-time credential
// compares — and deliberately nothing else: no sessions, no JWTs, no scopes.
// Deciding whether an unauthenticated listener is acceptable (loopback-only
// binding) is a network question, not an auth one: the caller checks
// netutil.IsLoopback. Token VERIFICATION belongs one level up (see
// [Authentication] and [TokenValidator]); the Guard answers with fixed
// credentials an operator configured.

package security

import (
	"crypto/subtle"
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
