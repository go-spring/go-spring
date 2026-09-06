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

	"github.com/gin-gonic/gin"
	"go-spring.org/cloud/security"
	StarterGin "go-spring.org/starter-gin"
	luohua "go-spring.org/starter-luohua"
)

// TestGuardedRoute exercises luohua's identity through the real gin security
// shell over HTTP: no token → 401, wrong authority → 403, a luohua token with
// the right authority → 200 with the authenticated subject.
func TestGuardedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sso := luohua.NewLuohuaSSO("example-secret", "luohua")

	router := gin.New()
	router.GET("/api/orders",
		StarterGin.Authenticate(sso, true),
		StarterGin.Authorize(luohua.AuthorityOrdersRead),
		func(c *gin.Context) {
			id, _ := security.FromContext(c.Request.Context())
			c.JSON(http.StatusOK, gin.H{"subject": id.Principal.Subject})
		})

	do := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := do(""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", rec.Code)
	}

	// A token valid for luohua but lacking orders:read → 403.
	weak, err := sso.Issue("u", "acme", []string{"other:perm"}, time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if rec := do("Bearer " + weak); rec.Code != http.StatusForbidden {
		t.Fatalf("weak token: got %d, want 403", rec.Code)
	}

	// A luohua token carrying orders:read → 200.
	good, err := sso.Issue("user-7", "acme", []string{luohua.AuthorityOrdersRead}, time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if rec := do("Bearer " + good); rec.Code != http.StatusOK {
		t.Fatalf("good token: got %d, want 200", rec.Code)
	}
}
