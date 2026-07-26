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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mwValidator is a TokenValidator that accepts exactly one token.
type mwValidator struct {
	good        string
	authorities []string
}

func (s mwValidator) Validate(_ context.Context, token string) (*Authentication, error) {
	if token != s.good {
		return nil, errors.New("bad token")
	}
	return &Authentication{
		Principal:     Principal{Subject: "alice"},
		Token:         token,
		Authenticated: true,
		Authorities:   s.authorities,
	}, nil
}

// okHandler writes 200 and records whether it ran.
func okHandler(ran *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ran != nil {
			*ran = true
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestChainOrder(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(mk("a"), mk("b"), mk("c"))(okHandler(nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := order; len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("chain order = %v, want [a b c]", got)
	}
}

func TestAuthenticate(t *testing.T) {
	v := mwValidator{good: "T"}

	tests := []struct {
		name     string
		token    string
		required bool
		want     int
		wantAuth bool
	}{
		{"valid", "T", true, http.StatusOK, true},
		{"missing-required", "", true, http.StatusUnauthorized, false},
		{"missing-optional", "", false, http.StatusOK, false},
		{"invalid", "X", true, http.StatusUnauthorized, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sawAuth bool
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if a, ok := FromContext(r.Context()); ok && a.Authenticated {
					sawAuth = true
				}
				w.WriteHeader(http.StatusOK)
			})
			h := Authenticate(v, tt.required)(inner)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d", rr.Code, tt.want)
			}
			if sawAuth != tt.wantAuth {
				t.Fatalf("sawAuth = %v, want %v", sawAuth, tt.wantAuth)
			}
		})
	}
}

func TestAuthorize(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		authorities []string
		require     []string
		want        int
	}{
		{"has-authority", "T", []string{"admin"}, []string{"admin"}, http.StatusOK},
		{"missing-authority", "T", []string{"user"}, []string{"admin"}, http.StatusForbidden},
		{"anonymous", "", nil, []string{"admin"}, http.StatusUnauthorized},
		{"any-authenticated", "T", []string{"user"}, nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := mwValidator{good: "T", authorities: tt.authorities}
			ran := false
			h := Chain(Authenticate(v, false), Authorize(tt.require...))(okHandler(&ran))

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d", rr.Code, tt.want)
			}
			if ran != (tt.want == http.StatusOK) {
				t.Fatalf("handler ran = %v, want %v", ran, tt.want == http.StatusOK)
			}
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	h := CORS(CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         600,
	})(okHandler(nil))

	req := httptest.NewRequest(http.MethodOptions, "/api", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow-origin = %q", got)
	}
	if got := rr.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("max-age = %q", got)
	}
}

func TestCORSDisallowedOriginPassesThrough(t *testing.T) {
	ran := false
	h := CORS(CORSConfig{AllowedOrigins: []string{"https://good.example.com"}})(okHandler(&ran))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !ran {
		t.Fatal("handler should still run for a disallowed origin")
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("no allow-origin header should be set for a disallowed origin")
	}
}

func TestCORSWildcardWithCredentialsEchoesOrigin(t *testing.T) {
	h := CORS(CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})(okHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("with credentials the exact origin must be echoed, got %q", got)
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("allow-credentials header missing")
	}
}

func TestCSRF(t *testing.T) {
	h := CSRF(CSRFConfig{})(okHandler(nil))

	// A safe GET issues the cookie.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := rr.Result().Cookies()
	var token string
	for _, c := range cookies {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("safe request did not issue a csrf_token cookie")
	}

	// An unsafe POST without the header is rejected.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("POST without header: status = %d, want 403", rr.Code)
	}

	// An unsafe POST echoing the token is allowed.
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST with matching header: status = %d, want 200", rr.Code)
	}

	// A wrong token is rejected.
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", "wrong")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("POST with wrong header: status = %d, want 403", rr.Code)
	}
}
