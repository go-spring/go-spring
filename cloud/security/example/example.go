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

// Command example demonstrates cloud/security on plain net/http: implementing
// the TokenValidator seam, attaching the resulting identity to the request
// context, gating a route on an authority, and enforcing an authority at the
// method level with the security.Require decorator.
//
// It self-asserts every step and exits non-zero on mismatch, so it doubles as
// the package's smoke test. No external services are required: a fixed token
// table stands in for the crypto a real validator would do. The production
// HTTP shells (starter-http-server / starter-gin / starter-echo) and a JWT
// validator (starter-security-jwt) are shown in their own examples.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"go-spring.org/cloud/security"
)

// tokenStore is the TokenValidator seam: the single thing a starter or the
// calling app implements to plug in credential verification. A real one
// verifies a JWT signature or introspects an opaque token; this one looks the
// token up in a fixed table so the example needs no crypto.
type tokenStore map[string]*security.Authentication

// Validate returns the identity behind token, or a non-nil error for any
// credential it cannot vouch for — the interface contract is an error, never an
// Authentication with Authenticated=false.
func (s tokenStore) Validate(_ context.Context, token string) (*security.Authentication, error) {
	auth, ok := s[token]
	if !ok {
		return nil, errors.New("unknown token")
	}
	out := *auth
	out.Token = token // the raw credential as presented
	return &out, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("security example ok")
}

func run() error {
	// The identity model is plain data: Subject is the stable id, Claims carries
	// whatever else the credential held, Authorities is the flattened permission
	// set the guards read.
	validator := tokenStore{
		"alice-token": {
			Principal:     security.Principal{Subject: "alice", Claims: map[string]any{"team": "orders"}},
			Authenticated: true,
			Authorities:   []string{"orders:read"},
		},
		"admin-token": {
			Principal:     security.Principal{Subject: "root"},
			Authenticated: true,
			Authorities:   []string{"orders:read", "orders:write", "ROLE_ADMIN"},
		},
	}

	svc := &orderService{}
	mux := http.NewServeMux()
	mux.Handle("/public", authenticate(validator, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("public"))
	})))
	mux.Handle("/orders", authenticate(validator, true,
		authorize("orders:read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, _ := security.FromContext(r.Context())
			_, _ = fmt.Fprintf(w, "orders of %s", auth.Principal.Subject)
		}))))
	mux.Handle("/orders/place", authenticate(validator, true, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Method-level gate: the business call is wrapped, not the route. On
		// failure Require returns the sentinel the response maps to a status.
		err := security.Require("orders:write")(r.Context(), svc.place)
		switch {
		case errors.Is(err, security.ErrUnauthenticated):
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
		case errors.Is(err, security.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte("placed"))
		}
	})))
	mux.Handle("/admin", authenticate(validator, true,
		authorize("ROLE_ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("admin ok"))
		}))))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. An unprotected route serves anonymous callers: required=false lets a
	// request with no credential through, attaching no Authentication.
	if code, body, err := get(srv.URL+"/public", ""); err != nil {
		return err
	} else if code != http.StatusOK || body != "public" {
		return fmt.Errorf("anonymous /public: code=%d body=%q", code, body)
	}
	fmt.Println("anonymous request passed the auth-optional route: OK")

	// 2. A present-but-invalid credential is always rejected, even where the
	// route does not require one — a bad token is not "anonymous".
	if code, _, err := get(srv.URL+"/public", "garbage"); err != nil {
		return err
	} else if code != http.StatusUnauthorized {
		return fmt.Errorf("garbage token /public: code=%d, want 401", code)
	}
	fmt.Println("invalid token rejected on the auth-optional route: OK")

	// 3. A missing credential on a protected route is 401.
	if code, _, err := get(srv.URL+"/orders", ""); err != nil {
		return err
	} else if code != http.StatusUnauthorized {
		return fmt.Errorf("anonymous /orders: code=%d, want 401", code)
	}
	fmt.Println("protected route rejected the anonymous caller: OK")

	// 4. A valid credential authenticates; the route gate admits the authority it
	// asked for, and the handler reads the subject back off the context.
	if code, body, err := get(srv.URL+"/orders", "alice-token"); err != nil {
		return err
	} else if code != http.StatusOK || body != "orders of alice" {
		return fmt.Errorf("alice /orders: code=%d body=%q", code, body)
	}
	fmt.Println("valid token authenticated and cleared the route gate: OK")

	// 5. The method-level decorator is authoritative too: alice holds orders:read
	// but not orders:write, so the wrapped business call never runs.
	if code, _, err := get(srv.URL+"/orders/place", "alice-token"); err != nil {
		return err
	} else if code != http.StatusForbidden {
		return fmt.Errorf("alice /orders/place: code=%d, want 403", code)
	}
	fmt.Println("method-level Require denied the missing authority: OK")

	// 6. The holder of orders:write clears the same decorator.
	if code, body, err := get(srv.URL+"/orders/place", "admin-token"); err != nil {
		return err
	} else if code != http.StatusOK || body != "placed" {
		return fmt.Errorf("admin /orders/place: code=%d body=%q", code, body)
	}
	fmt.Println("method-level Require admitted the held authority: OK")

	// 7. The same authority set backs the route gate: ROLE_ADMIN is required for
	// /admin and alice does not carry it.
	if code, _, err := get(srv.URL+"/admin", "alice-token"); err != nil {
		return err
	} else if code != http.StatusForbidden {
		return fmt.Errorf("alice /admin: code=%d, want 403", code)
	}
	if code, body, err := get(srv.URL+"/admin", "admin-token"); err != nil {
		return err
	} else if code != http.StatusOK || body != "admin ok" {
		return fmt.Errorf("admin /admin: code=%d body=%q", code, body)
	}
	fmt.Println("route gate enforced ROLE_ADMIN: OK")

	// 8. The authority predicates are nil-safe: a nil Authentication (the "no
	// credential attached" case) holds nothing, so downstream code needs no nil
	// guard.
	var anon *security.Authentication
	if anon.HasAuthority("orders:read") || anon.HasAnyAuthority() || anon.HasAllAuthorities("orders:read") {
		return errors.New("nil Authentication must hold no authorities")
	}
	admin := &security.Authentication{Authenticated: true, Authorities: []string{"orders:read", "orders:write"}}
	if !admin.HasAnyAuthority("orders:write", "orders:delete") || !admin.HasAllAuthorities("orders:read", "orders:write") {
		return errors.New("authority predicates disagreed with the authority set")
	}
	fmt.Println("authority predicates are nil-safe and set-correct: OK")

	// 9. The shared pure helpers: every server family parses bearer credentials
	// through ParseBearerToken so acceptance cannot drift, and compares CSRF
	// tokens in constant time through MatchCSRFToken.
	for header, want := range map[string]string{
		"Bearer abc": "abc",
		"bearer abc": "abc", // scheme is case-insensitive
		"Basic abc":  "",
		"":           "",
	} {
		if got := security.ParseBearerToken(header); got != want {
			return fmt.Errorf("ParseBearerToken(%q)=%q, want %q", header, got, want)
		}
	}
	token := security.NewCSRFToken()
	if !security.MatchCSRFToken(token, token) || security.MatchCSRFToken(token, token+"x") || security.MatchCSRFToken("", "") {
		return errors.New("CSRF double-submit comparison is not constant-time correct")
	}
	fmt.Println("shared parsing and CSRF helpers behave: OK")

	return nil
}

// authenticate is a server-family middleware in miniature. Each family ships
// its own shell in its own idiom — stdlib decorators in starter-http-server,
// gin.HandlerFunc in starter-gin, echo.MiddlewareFunc in starter-echo — but
// they all parse the credential the same way and attach the same identity.
// required=false is the "authority decision deferred to a later gate" case.
func authenticate(v security.TokenValidator, required bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := security.ParseBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			if required {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		auth, err := v.Validate(r.Context(), token)
		if err != nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(security.WithAuthentication(r.Context(), auth)))
	})
}

// authorize is the route-level gate: the same authority set the method-level
// Require reads, checked where the route is registered. It maps the sentinels
// to 401/403 the way a resource server does.
func authorize(authorities ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, _ := security.FromContext(r.Context())
			if auth == nil || !auth.Authenticated {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
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

// orderService is the business object the method-level decorator wraps. Its
// place method is called only when Require has admitted the caller.
type orderService struct{}

func (s *orderService) place(_ context.Context) error { return nil }

// get performs one GET, optionally carrying an Authorization: Bearer header,
// and returns the status and body.
func get(url, token string) (int, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", err
	}
	return resp.StatusCode, string(b), nil
}
