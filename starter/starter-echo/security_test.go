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

package StarterEcho

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/security"
)

// secValidator is a security.TokenValidator that accepts exactly one token.
type secValidator struct {
	good        string
	authorities []string
}

func (s secValidator) Validate(_ context.Context, token string) (*security.Authentication, error) {
	if token != s.good {
		return nil, errors.New("bad token")
	}
	return &security.Authentication{
		Principal:     security.Principal{Subject: "alice"},
		Token:         token,
		Authenticated: true,
		Authorities:   s.authorities,
	}, nil
}

func TestEchoAuthenticateAuthorize(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		authorities []string
		require     []string
		want        int
	}{
		{"ok", "T", []string{"admin"}, []string{"admin"}, http.StatusOK},
		{"missing-authority", "T", []string{"user"}, []string{"admin"}, http.StatusForbidden},
		{"anonymous", "", nil, []string{"admin"}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.HideBanner = true
			ran := false
			e.GET("/x", func(c echo.Context) error {
				ran = true
				return c.String(http.StatusOK, "ok")
			}, Authenticate(secValidator{good: "T", authorities: tt.authorities}, false), Authorize(tt.require...))

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rr := httptest.NewRecorder()
			e.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d", rr.Code, tt.want)
			}
			if ran != (tt.want == http.StatusOK) {
				t.Fatalf("handler ran = %v, want %v", ran, tt.want == http.StatusOK)
			}
		})
	}
}

func TestEchoAuthenticateStoresAuthentication(t *testing.T) {
	e := echo.New()
	e.HideBanner = true
	var subject string
	e.GET("/x", func(c echo.Context) error {
		if a, ok := security.FromContext(c.Request().Context()); ok {
			subject = a.Principal.Subject
		}
		return c.String(http.StatusOK, "ok")
	}, Authenticate(secValidator{good: "T"}, true))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer T")
	e.ServeHTTP(httptest.NewRecorder(), req)
	if subject != "alice" {
		t.Fatalf("subject = %q, want alice", subject)
	}
}

func TestEchoCSRF(t *testing.T) {
	e := echo.New()
	e.HideBanner = true
	ran := false
	ok := func(c echo.Context) error {
		ran = true
		return c.String(http.StatusOK, "ok")
	}
	e.GET("/x", ok, CSRF(CSRFConfig{}))
	e.POST("/x", ok, CSRF(CSRFConfig{}))

	// A safe GET issues the cookie.
	rr := httptest.NewRecorder()
	e.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	var token string
	for _, c := range rr.Result().Cookies() {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("safe request did not issue a csrf_token cookie")
	}

	// An unsafe POST without the header is rejected.
	ran = false
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	rr = httptest.NewRecorder()
	e.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || ran {
		t.Fatalf("POST without header: status = %d ran = %v, want 403 false", rr.Code, ran)
	}

	// An unsafe POST echoing the token is allowed.
	ran = false
	req = httptest.NewRequest(http.MethodPost, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	rr = httptest.NewRecorder()
	e.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !ran {
		t.Fatalf("POST with matching header: status = %d ran = %v, want 200 true", rr.Code, ran)
	}
}
