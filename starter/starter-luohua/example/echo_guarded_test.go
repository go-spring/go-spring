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

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/security"
	StarterEcho "go-spring.org/starter-echo"
	luohua "go-spring.org/starter-luohua"
)

// TestEchoGuardedRoute exercises luohua's identity through the real
// starter-echo security shell over HTTP: no token → 401, wrong authority → 403,
// a luohua token with the right authority → 200 with the subject. Together with
// router_test.go (gin) and http_server_guarded_test.go it shows luohua's
// company token scheme feeds every go-spring HTTP family security shell.
func TestEchoGuardedRoute(t *testing.T) {
	sso := luohua.NewLuohuaSSO("example-secret", "luohua")

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.GET("/api/orders", func(c echo.Context) error {
		id, _ := security.FromContext(c.Request().Context())
		subject := ""
		if id != nil {
			subject = id.Principal.Subject
		}
		return c.String(http.StatusOK, subject)
	}, StarterEcho.Authenticate(sso, true), StarterEcho.Authorize(luohua.AuthorityOrdersRead))

	do := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := do(""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", rec.Code)
	}

	weak, err := sso.Issue("u", "acme", []string{"other:perm"}, time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if rec := do("Bearer " + weak); rec.Code != http.StatusForbidden {
		t.Fatalf("weak token: got %d, want 403", rec.Code)
	}

	good, err := sso.Issue("user-7", "acme", []string{luohua.AuthorityOrdersRead}, time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if rec := do("Bearer " + good); rec.Code != http.StatusOK || rec.Body.String() != "user-7" {
		t.Fatalf("good token: got %d body=%q, want 200 body=user-7", rec.Code, rec.Body.String())
	}
}
