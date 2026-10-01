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
// this?" ([HasAnyAuthority] / the [Require] upfront check). Token verification
// itself is pluggable: a starter implements the single [TokenValidator]
// interface (e.g. JWT verification) and contributes it as a container bean;
// a resource-server middleware takes that validator, validates the incoming
// credential, and attaches the resulting [Authentication] to the context for
// downstream guards. This package is framework-free and models only the seam —
// which validator wires in is a wiring decision, not a name-keyed lookup here.
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
	return slices.ContainsFunc(authorities, a.HasAuthority)
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

// ValidatorFunc adapts a plain function to [TokenValidator], the way
// [http.HandlerFunc] adapts a function to [http.Handler]. A caller that has no
// state to carry — a test double, an example, a one-off rule — writes the
// verification inline instead of declaring a type for it.
type ValidatorFunc func(ctx context.Context, token string) (*Authentication, error)

// Validate implements [TokenValidator].
func (f ValidatorFunc) Validate(ctx context.Context, token string) (*Authentication, error) {
	return f(ctx, token)
}

// ctxKey is an unexported context key type so the stored Authentication cannot
// collide with keys from other packages.
type ctxKey struct{}

// WithAuthentication returns a copy of ctx carrying auth. A resource-server
// middleware calls it after verifying a credential so downstream handlers and
// method-level guards can read the identity via [FromContext].
func WithAuthentication(ctx context.Context, auth *Authentication) context.Context {
	return context.WithValue(ctx, ctxKey{}, auth)
}

// FromContext returns the [Authentication] carried by ctx, if any. The boolean
// reports whether an Authentication was present at all — a caller that needs to
// distinguish "no identity attached" from "attached but anonymous" can use it;
// most callers can ignore it and rely on the *Authentication methods being
// nil-safe.
func FromContext(ctx context.Context) (*Authentication, bool) {
	auth, ok := ctx.Value(ctxKey{}).(*Authentication)
	return auth, ok
}

// Require is the @PreAuthorize equivalent as a plain upfront check: call it at
// the top of a method to gate the body before it runs.
//
// It reads the [Authentication] carried on the context (put there by a
// resource-server middleware via [WithAuthentication]) and:
//   - returns [ErrUnauthenticated] when the caller carries no verified identity;
//   - returns [ErrForbidden] when authenticated but holding none of authorities;
//   - otherwise returns nil.
//
// With no authorities it degrades to "an authenticated caller is required".
// The caller maps the sentinels to a transport status:
//
//	if err := security.Require(ctx, "orders:write"); err != nil {
//	    return err // ErrUnauthenticated -> 401, ErrForbidden -> 403
//	}
//	return svc.Place(ctx, req)
//
// It is an ordinary function, not a middleware and not a decorator: there is
// deliberately no shared interceptor-chain protocol, so it never wraps the
// business call — the guard is a statement at the top of the method.
func Require(ctx context.Context, authorities ...string) error {
	auth, _ := FromContext(ctx)
	if auth == nil || !auth.Authenticated {
		return ErrUnauthenticated
	}
	if !auth.HasAnyAuthority(authorities...) {
		return ErrForbidden
	}
	return nil
}

var (
	// ErrUnauthenticated indicates the request carried no valid identity. A
	// resource server maps it to HTTP 401.
	ErrUnauthenticated = errors.New("security: unauthenticated")

	// ErrForbidden indicates the caller is authenticated but lacks the required
	// authority. A resource server maps it to HTTP 403.
	ErrForbidden = errors.New("security: forbidden")
)
