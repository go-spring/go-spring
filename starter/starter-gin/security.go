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

package StarterGin

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"go-spring.org/cloud/security"
)

// Authenticate returns the bearer-authentication middleware: it reads the
// Authorization header, verifies the token with v, and on success attaches the
// resulting Authentication to the request context so handlers and the Authorize
// middleware (or the method-level security.Require decorator) can read it via
// security.FromContext(c.Request.Context()).
//
// When the request carries no token: required=true aborts with 401;
// required=false calls the remaining handlers with no Authentication attached,
// deferring the decision to a later authority check. An invalid token always
// yields 401.
func Authenticate(v security.TokenValidator, required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := security.ParseBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			if required {
				c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Next()
			return
		}
		auth, err := v.Validate(c.Request.Context(), token)
		if err != nil {
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Request = c.Request.WithContext(security.WithAuthentication(c.Request.Context(), auth))
		c.Next()
	}
}

// Authorize returns the authorization middleware: it requires that the request
// already carries a verified Authentication (see Authenticate) holding at least
// one of authorities. With no authorities it degrades to "authenticated caller
// required". It is the route-level counterpart of the method-level
// security.Require decorator: use this to gate a route group, Require to gate a
// service method.
//
// A missing/anonymous identity yields 401; an authenticated caller lacking the
// authority yields 403.
func Authorize(authorities ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth, _ := security.FromContext(c.Request.Context())
		if !auth.HasAnyAuthority() {
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if !auth.HasAnyAuthority(authorities...) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

// CSRFConfig configures the double-submit-cookie CSRF middleware.
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

// CSRF returns the CSRF middleware using the stateless double-submit-cookie
// pattern: a random token is stored in a cookie on safe requests, and every
// unsafe request (POST/PUT/PATCH/DELETE, ...) must echo that same token in the
// configured header. A mismatch or a missing token aborts with 403. This
// defends browser form/AJAX flows without server-side session state; it is
// orthogonal to bearer-token APIs, which are not CSRF-prone and typically omit
// it.
func CSRF(cfg CSRFConfig) gin.HandlerFunc {
	cookieName := cfg.CookieName
	if cookieName == "" {
		cookieName = security.DefaultCSRFCookieName
	}
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = security.DefaultCSRFHeaderName
	}
	cookiePath := cfg.CookiePath
	if cookiePath == "" {
		cookiePath = "/"
	}
	safe := cfg.SafeMethods
	if len(safe) == 0 {
		safe = []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}
	}

	return func(c *gin.Context) {
		cookie, _ := c.Cookie(cookieName)

		if slices.Contains(safe, c.Request.Method) {
			// Ensure a token exists so the client can echo it on later writes.
			if cookie == "" {
				c.SetSameSite(http.SameSiteLaxMode)
				c.SetCookie(cookieName, security.NewCSRFToken(), 0, cookiePath, "", cfg.Secure, false)
			}
			c.Next()
			return
		}

		// Unsafe method: the header token must match the cookie token.
		if !security.MatchCSRFToken(cookie, c.GetHeader(headerName)) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
