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

package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Middleware is the unit of the Web security filter chain: it wraps an
// http.Handler and returns a handler that runs some concern (CORS, CSRF,
// authentication, authorization) before delegating to the next one. It is the
// Go-idiomatic equivalent of a Spring Security filter — an ordinary
// net/http decorator, not a bespoke filter registry.
type Middleware func(http.Handler) http.Handler

// Chain composes middlewares into one, applied outermost-first: the request
// flows through ms[0], then ms[1], ... , then the wrapped handler, and the
// response unwinds in reverse. This is the seam for ordering the security
// concerns — typically Chain(CORS, CSRF, Authenticate, Authorize) so a caller
// is identified before an authority decision, and both after the cross-origin
// and CSRF gates. Chain() with no arguments is the identity.
func Chain(ms ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(ms) - 1; i >= 0; i-- {
			next = ms[i](next)
		}
		return next
	}
}

// BearerToken extracts the token from an "Authorization: Bearer <token>"
// header, returning "" when the header is absent or not a bearer credential.
func BearerToken(r *http.Request) string {
	const prefix = "bearer "
	h := r.Header.Get("Authorization")
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// Authenticate returns the authentication filter: it reads the bearer token,
// verifies it with v, and on success attaches the resulting Authentication to
// the request context so downstream handlers and the Authorize filter (or the
// method-level Require aspect) can read it via FromContext.
//
// When the request carries no token: required=true rejects with 401;
// required=false passes the request through with no Authentication attached,
// deferring the decision to a later authority check. An invalid token always
// yields 401. This is the transport-agnostic counterpart to a resource-server
// starter's own Wrap — it works with any registered TokenValidator.
func Authenticate(v TokenValidator, required bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := BearerToken(r)
			if token == "" {
				if required {
					writeUnauthorized(w, "missing bearer token")
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			auth, err := v.Validate(r.Context(), token)
			if err != nil {
				writeUnauthorized(w, "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAuthentication(r.Context(), auth)))
		})
	}
}

// Authorize returns the authorization filter: it requires that the request
// already carries a verified Authentication (see Authenticate) holding at least
// one of authorities. With no authorities it degrades to "authenticated caller
// required". It is the HTTP-layer counterpart of the Require aspect: use this to
// gate a route, Require to gate a service method.
//
// A missing/anonymous identity yields 401; an authenticated caller lacking the
// authority yields 403.
func Authorize(authorities ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, _ := FromContext(r.Context())
			if auth == nil || !auth.Authenticated {
				writeUnauthorized(w, "unauthenticated")
				return
			}
			if !auth.HasAnyAuthority(authorities...) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeUnauthorized writes a 401 with a Bearer challenge.
func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	http.Error(w, msg, http.StatusUnauthorized)
}

// CORSConfig configures the cross-origin resource sharing filter. The zero value
// allows nothing; set at least AllowedOrigins to enable cross-origin access.
type CORSConfig struct {
	// AllowedOrigins is the set of permitted request origins. The single entry
	// "*" allows any origin, but only when AllowCredentials is false — the CORS
	// spec forbids the wildcard together with credentials, so with credentials
	// on, list the exact origins instead.
	AllowedOrigins []string

	// AllowedMethods is the set of methods advertised in a preflight response.
	// Defaults to GET, POST, PUT, PATCH, DELETE, OPTIONS when empty.
	AllowedMethods []string

	// AllowedHeaders is the set of request headers advertised in a preflight
	// response. The single entry "*" reflects whatever the preflight requests.
	AllowedHeaders []string

	// ExposedHeaders is the set of response headers the browser may expose to
	// script.
	ExposedHeaders []string

	// AllowCredentials advertises that the response may be read when the request
	// carries credentials (cookies, HTTP auth). Incompatible with a "*" origin.
	AllowCredentials bool

	// MaxAge is the number of seconds a preflight result may be cached. 0 omits
	// the header.
	MaxAge int
}

// CORS returns the cross-origin filter. For a request carrying an Origin that
// matches AllowedOrigins it adds the Access-Control-Allow-* response headers; a
// preflight (OPTIONS with Access-Control-Request-Method) is answered directly
// with 204 and does not reach the wrapped handler. Requests without an Origin,
// or with a disallowed one, pass through unchanged (the browser enforces the
// absence of the headers).
func CORS(c CORSConfig) Middleware {
	methods := c.AllowedMethods
	if len(methods) == 0 {
		methods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	}
	allowMethods := strings.Join(methods, ", ")
	allowHeaders := strings.Join(c.AllowedHeaders, ", ")
	exposeHeaders := strings.Join(c.ExposedHeaders, ", ")
	wildcardHeaders := slices.Contains(c.AllowedHeaders, "*")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || !c.originAllowed(origin) {
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			if slices.Contains(c.AllowedOrigins, "*") && !c.AllowCredentials {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}
			if c.AllowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			if exposeHeaders != "" {
				h.Set("Access-Control-Expose-Headers", exposeHeaders)
			}

			// Preflight: answer directly, do not invoke the handler.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", allowMethods)
				if wildcardHeaders {
					if req := r.Header.Get("Access-Control-Request-Headers"); req != "" {
						h.Set("Access-Control-Allow-Headers", req)
					}
				} else if allowHeaders != "" {
					h.Set("Access-Control-Allow-Headers", allowHeaders)
				}
				if c.MaxAge > 0 {
					h.Set("Access-Control-Max-Age", strconv.Itoa(c.MaxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// originAllowed reports whether origin is permitted by the configuration.
func (c CORSConfig) originAllowed(origin string) bool {
	for _, o := range c.AllowedOrigins {
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

// CSRFConfig configures the double-submit-cookie CSRF filter.
type CSRFConfig struct {
	// CookieName is the cookie holding the CSRF token. Default "csrf_token".
	CookieName string

	// HeaderName is the request header a state-changing request must echo the
	// cookie token in. Default "X-CSRF-Token".
	HeaderName string

	// CookiePath is the path attribute of the issued cookie. Default "/".
	CookiePath string

	// Secure marks the cookie Secure (HTTPS only). Leave false for local HTTP
	// development.
	Secure bool

	// SafeMethods are the methods that do not require a token and that (re)issue
	// the cookie when absent. Defaults to GET, HEAD, OPTIONS, TRACE.
	SafeMethods []string
}

// CSRF returns the CSRF filter using the stateless double-submit-cookie pattern:
// a random token is stored in a cookie on safe requests, and every unsafe
// request (POST/PUT/PATCH/DELETE, ...) must echo that same token in the
// configured header. A mismatch or a missing token yields 403. This defends
// browser form/AJAX flows without server-side session state; it is orthogonal to
// bearer-token APIs, which are not CSRF-prone and typically omit it.
func CSRF(c CSRFConfig) Middleware {
	cookieName := c.CookieName
	if cookieName == "" {
		cookieName = "csrf_token"
	}
	headerName := c.HeaderName
	if headerName == "" {
		headerName = "X-CSRF-Token"
	}
	cookiePath := c.CookiePath
	if cookiePath == "" {
		cookiePath = "/"
	}
	safe := c.SafeMethods
	if len(safe) == 0 {
		safe = []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, _ := r.Cookie(cookieName)

			if slices.Contains(safe, r.Method) {
				// Ensure a token exists so the client can echo it on later writes.
				if cookie == nil || cookie.Value == "" {
					http.SetCookie(w, &http.Cookie{
						Name:     cookieName,
						Value:    newToken(),
						Path:     cookiePath,
						Secure:   c.Secure,
						SameSite: http.SameSiteLaxMode,
					})
				}
				next.ServeHTTP(w, r)
				return
			}

			// Unsafe method: the header token must match the cookie token.
			header := r.Header.Get(headerName)
			if cookie == nil || cookie.Value == "" || header == "" ||
				subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newToken returns a 32-byte URL-safe random token.
func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
