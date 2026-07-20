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

// Package security defines a framework-agnostic, zero-dependency abstraction for
// authentication and authorization — the Spring Security equivalent expressed in
// Go idioms rather than a port of its filter-chain machinery.
//
// It answers two questions for a resource server: "who is the caller?"
// ([Authentication] carried on the request context) and "may this caller do
// this?" ([HasAnyAuthority] / the aspect [Require] marker). Token verification
// itself is pluggable: a starter implements the single [TokenValidator]
// interface (e.g. JWT verification) and registers it via [RegisterValidator];
// method-level guards then resolve validators by name without depending on any
// concrete crypto library.
package security

import (
	"context"
	"errors"
	"slices"
)

// Principal is the authenticated identity extracted from a credential. Subject
// is the stable identifier (the JWT "sub", a user id, ...); Claims carries the
// raw, backend-specific attributes so callers can route on anything the token
// carried without this package modelling every field.
type Principal struct {
	// Subject is the stable identifier of the caller (e.g. the JWT "sub" claim).
	Subject string

	// Claims holds the raw credential attributes, passed through untouched.
	Claims map[string]any
}

// Authentication is the result of validating a credential. It is what a
// [TokenValidator] produces and what [FromContext] returns to downstream code.
type Authentication struct {
	// Principal is the identity behind the credential.
	Principal Principal

	// Token is the raw credential (e.g. the bearer token) as presented.
	Token string

	// Authenticated reports whether the credential was successfully verified. An
	// unauthenticated Authentication (nil or Authenticated=false) means the
	// request carried no valid identity; guards treat it as anonymous.
	Authenticated bool

	// Authorities are the granted permissions used for authorization decisions —
	// scopes and roles flattened into one namespace (e.g. "orders:read",
	// "ROLE_ADMIN"). Callers decide the naming convention.
	Authorities []string
}

// TokenValidator verifies a raw credential and, on success, returns the
// [Authentication] it represents. It is the single seam a company (or starter)
// implements to plug in JWT verification, opaque-token introspection, etc.
//
// Implementations must be safe for concurrent use and must return a non-nil
// error for any credential they cannot vouch for, rather than an
// Authentication with Authenticated=false.
type TokenValidator interface {
	Validate(ctx context.Context, token string) (*Authentication, error)
}

// HasAuthority reports whether a carries the given authority. A nil or
// unauthenticated Authentication has no authorities.
func (a *Authentication) HasAuthority(authority string) bool {
	if a == nil || !a.Authenticated {
		return false
	}
	return slices.Contains(a.Authorities, authority)
}

// HasAnyAuthority reports whether a carries at least one of the given
// authorities. With no arguments it reports whether a is authenticated at all.
func (a *Authentication) HasAnyAuthority(authorities ...string) bool {
	if a == nil || !a.Authenticated {
		return false
	}
	if len(authorities) == 0 {
		return true
	}
	for _, want := range authorities {
		if a.HasAuthority(want) {
			return true
		}
	}
	return false
}

// HasAllAuthorities reports whether a carries every one of the given
// authorities. With no arguments it reports whether a is authenticated at all.
func (a *Authentication) HasAllAuthorities(authorities ...string) bool {
	if a == nil || !a.Authenticated {
		return false
	}
	for _, want := range authorities {
		if !a.HasAuthority(want) {
			return false
		}
	}
	return true
}

var (
	// ErrUnauthenticated indicates the request carried no valid identity. A
	// resource server maps it to HTTP 401.
	ErrUnauthenticated = errors.New("security: unauthenticated")

	// ErrForbidden indicates the caller is authenticated but lacks the required
	// authority. A resource server maps it to HTTP 403.
	ErrForbidden = errors.New("security: forbidden")
)
