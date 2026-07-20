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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterSecurityJWT "go-spring.org/starter-security-jwt"
	"go-spring.org/spring/web/security"
)

// secret is the shared HMAC key. It matches spring.security.jwt.api.secret in
// conf/app.properties, so tokens this example mints verify against the
// authenticator the starter builds — no external identity provider needed.
const secret = "example-shared-secret"

func main() {
	// Provide a *gs.HttpServeMux whose handler is the business mux wrapped by the
	// "api" JWT authenticator. gs registers the default HttpServeMux only when
	// none is present, so this custom one wins and every request is authenticated
	// before reaching a handler, regardless of the web framework behind it.
	gs.Provide(func(auth *StarterSecurityJWT.Authenticator) *gs.HttpServeMux {
		mux := http.NewServeMux()

		// /me echoes the authenticated subject. Reaching this handler means the
		// bearer token already verified in the Wrap middleware.
		mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
			a, _ := security.FromContext(r.Context())
			_, _ = fmt.Fprintf(w, "hello %s", a.Principal.Subject)
		})

		// /admin adds a method-level authority check on top of authentication:
		// the verified identity must carry the "admin" authority.
		mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
			a, _ := security.FromContext(r.Context())
			if !a.HasAuthority("admin") {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte("admin ok"))
		})

		return &gs.HttpServeMux{Handler: auth.Wrap(mux)}
	}, gs.TagArg("api"))

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Run()

	// Example usage:
	//
	// ~ curl -i http://127.0.0.1:9090/me
	// HTTP/1.1 401 Unauthorized
	//
	// ~ curl -i -H "Authorization: Bearer <user-token>" http://127.0.0.1:9090/me
	// HTTP/1.1 200 OK
	// hello alice
	//
	// ~ curl -i -H "Authorization: Bearer <user-token>" http://127.0.0.1:9090/admin
	// HTTP/1.1 403 Forbidden
	//
	// ~ curl -i -H "Authorization: Bearer <admin-token>" http://127.0.0.1:9090/admin
	// HTTP/1.1 200 OK
	// admin ok
}

// mint signs an HS256 token for subject with the given roles.
func mint(subject string, roles ...string) string {
	claims := jwt.MapClaims{
		"sub":   subject,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"roles": roles,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		fail("mint token: %v", err)
	}
	return s
}

func runTest() {
	// Feature 1: no bearer token is rejected (required=true by default).
	if status, _ := do("/me", ""); status != http.StatusUnauthorized {
		fail("no token: expected 401, got %d", status)
	}

	// Feature 2: a valid token authenticates and the subject is echoed back.
	userTok := mint("alice", "user")
	if status, body := do("/me", userTok); status != 200 || body != "hello alice" {
		fail("user /me: status=%d body=%q", status, body)
	}

	// Feature 3: a user without the "admin" authority is forbidden.
	if status, _ := do("/admin", userTok); status != http.StatusForbidden {
		fail("user /admin: expected 403, got %d", status)
	}

	// Feature 4: an admin token clears the method-level authority check.
	adminTok := mint("root", "admin")
	if status, body := do("/admin", adminTok); status != 200 || body != "admin ok" {
		fail("admin /admin: status=%d body=%q", status, body)
	}

	// Feature 5: a garbage token is rejected.
	if status, _ := do("/me", "not-a-real-token"); status != http.StatusUnauthorized {
		fail("bad token: expected 401, got %d", status)
	}

	fmt.Println("Response from server: jwt rejected missing/invalid tokens, authenticated alice, and enforced the admin authority")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// do issues a GET and returns status and body. token, when non-empty, is sent
// as an Authorization: Bearer header.
func do(path, token string) (int, string) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9090"+path, nil)
	if err != nil {
		fail("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("do %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}
