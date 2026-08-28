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

package StarterAdminUI

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/spring/gs"
)

// The guard tests target Server.newHandler directly (the same handler Run
// installs behind http.Server) so they exercise the real wrap path without
// racing the poller goroutine.

// TestGuardToken tests that a configured spring.admin-ui.token rejects
// requests without the bearer header (401) and admits requests with it (200).
func TestGuardToken(t *testing.T) {
	s := &Server{Config: Config{Addr: "127.0.0.1:0", Token: "s3cret"}}
	h := s.newHandler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without token: want 401, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with token: want 200, got %d", rec.Code)
	}
}

// TestGuardBasicAuth tests that configured username/password rejects requests
// without credentials and admits correct Basic credentials.
func TestGuardBasicAuth(t *testing.T) {
	s := &Server{Config: Config{Addr: "127.0.0.1:0", Username: "admin", Password: "pw"}}
	h := s.newHandler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without credentials: want 401, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.SetBasicAuth("admin", "pw")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with credentials: want 200, got %d", rec.Code)
	}
}

// TestGuardDisabled tests that with no credentials configured the handler
// serves unauthenticated (the loopback/trust decision is the operator's).
func TestGuardDisabled(t *testing.T) {
	s := &Server{Config: Config{Addr: "127.0.0.1:0"}}
	h := s.newHandler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("no guard: want 200, got %d", rec.Code)
	}
}

// TestActivationNoAddr tests the activation contract: with no
// spring.admin-ui.addr property the starter registers no gs.Server bean at
// all (default-off, matching starter-actuator).
func TestActivationNoAddr(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Servers []gs.Server `autowire:"?"`
	}) {
		for _, sv := range s.Servers {
			if _, ok := sv.(*Server); ok {
				t.Fatalf("admin-ui server must not exist without spring.admin-ui.addr")
			}
		}
	})
}

// TestActivationAddrSet tests that configuring spring.admin-ui.addr assembles
// and runs the server bean.
func TestActivationAddrSet(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.admin-ui.addr", "127.0.0.1:19385")
	}).RunTest(t, func(s *struct {
		Servers []gs.Server `autowire:""`
	}) {
		var admin *Server
		for _, sv := range s.Servers {
			if v, ok := sv.(*Server); ok {
				admin = v
			}
		}
		if admin == nil {
			t.Fatalf("admin-ui server must exist when spring.admin-ui.addr is set")
		}
	})
}
