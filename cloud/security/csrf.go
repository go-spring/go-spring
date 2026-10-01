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
	"crypto/subtle"

	"go-spring.org/stdlib/randutil"
)

// Default CSRF token exchange names, shared by every server family's CSRF
// middleware so a browser frontend works unchanged across them.
const (
	// DefaultCSRFCookieName is the cookie holding the CSRF token.
	DefaultCSRFCookieName = "csrf_token"

	// DefaultCSRFHeaderName is the header an unsafe request must echo the
	// cookie token in.
	DefaultCSRFHeaderName = "X-CSRF-Token"
)

// DefaultSafeMethods returns the safe HTTP methods — GET, HEAD, OPTIONS, TRACE
// — which RFC 9110 defines as read-only and which a CSRF filter therefore lets
// through untouched. A fresh slice is returned so callers cannot mutate a
// shared one.
func DefaultSafeMethods() []string {
	return []string{"GET", "HEAD", "OPTIONS", "TRACE"}
}

// NewCSRFToken returns a fresh 32-byte URL-safe random token, suitable for the
// double-submit-cookie exchange: the server stores it in a cookie and the
// client must echo it back in a header on every state-changing request.
func NewCSRFToken() string { return randutil.URLSafe(32) }

// MatchCSRFToken reports whether the header token equals the cookie token,
// comparing in constant time so a timing attack cannot recover the expected
// value. Either side empty is a mismatch.
func MatchCSRFToken(cookieToken, headerToken string) bool {
	if cookieToken == "" || headerToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) == 1
}
