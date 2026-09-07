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

	"go-spring.org/cloud/security"
	HTTPServer "go-spring.org/starter-http-server"
	luohua "go-spring.org/starter-luohua"
)

// TestHTTPGuardedRoute exercises luohua's identity through the real
// starter-http-server security shell over HTTP: no token → 401, wrong authority
// → 403, a luohua token with the right authority → 200 with the subject. It is
// the net/http-family counterpart of router_test.go's gin TestGuardedRoute and
// proves luohua's company token scheme feeds every go-spring HTTP family shell.
func TestHTTPGuardedRoute(t *testing.T) {
	sso := luohua.NewLuohuaSSO("example-secret", "luohua")

	handler := HTTPServer.Chain(
		HTTPServer.Authenticate(sso, true),
		HTTPServer.Authorize(luohua.AuthorityOrdersRead),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := security.FromContext(r.Context())
		subject := ""
		if id != nil {
			subject = id.Principal.Subject
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(subject))
	}))

	do := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
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
