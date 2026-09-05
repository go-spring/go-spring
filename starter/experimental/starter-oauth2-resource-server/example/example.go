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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	httpsvr "go-spring.org/starter-http-server"

	// Blank-importing the starter registers the validators configured under
	// spring.security.oauth2.resource.jwt.
	_ "go-spring.org/starter-oauth2-resource-server"
)

// secret is the shared HMAC key. It matches
// spring.security.oauth2.resource.jwt.api.secret in conf/app.properties, so
// tokens this example mints verify against the validator the starter builds —
// no external identity provider needed. Real deployments configure issuer-uri
// instead and let the starter discover the issuer's JWKS endpoint.
const secret = "example-shared-secret"

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// The starter exports its validator as the framework-neutral
	// security.TokenValidator bean named "api", so the application depends only
	// on the seam and composes it with the security middleware kit.
	gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
		mux := http.NewServeMux()

		// /me echoes the authenticated subject — reaching it means the bearer
		// token already verified in the Authenticate middleware.
		mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
			a, _ := security.FromContext(r.Context())
			_, _ = fmt.Fprintf(w, "hello %s", a.Principal.Subject)
		})

		// The security chain: identify the caller first, then gate routes with
		// authority checks.
		mux.Handle("/orders", httpsvr.Authorize("orders:read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("orders ok"))
		})))
		handler := httpsvr.Chain(
			httpsvr.Authenticate(v, true),
			httpsvr.Authorize(),
		)(mux)
		return &gs.HttpServeMux{Handler: handler}
	}, gs.TagArg("api"))

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// mint signs an HS256 token for subject with the given scopes.
func mint(subject string, scopes ...string) string {
	claims := jwt.MapClaims{
		"sub":   subject,
		"aud":   "example-api", // must match spring.security...api.audiences
		"exp":   time.Now().Add(time.Hour).Unix(),
		"scope": scopes,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		fail("mint token: %v", err)
	}
	return s
}

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
		fail("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func runTest() {
	// Feature 1: no bearer token is rejected (required=true).
	if status, _ := do("/me", ""); status != http.StatusUnauthorized {
		fail("no token: expected 401, got %d", status)
	}

	// Feature 2: a valid token authenticates and the subject is echoed back.
	if status, body := do("/me", mint("alice", "orders:read")); status != 200 || body != "hello alice" {
		fail("user /me: status=%d body=%q", status, body)
	}

	// Feature 3: a token without the "orders:read" scope is forbidden.
	if status, _ := do("/orders", mint("bob")); status != http.StatusForbidden {
		fail("bob /orders: expected 403, got %d", status)
	}

	// Feature 4: a scoped token clears the authority check.
	if status, body := do("/orders", mint("alice", "orders:read")); status != 200 || body != "orders ok" {
		fail("alice /orders: status=%d body=%q", status, body)
	}

	// Feature 5: a token signed with the wrong secret is rejected.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "x", "exp": time.Now().Add(time.Hour).Unix()})
	forged, _ := tok.SignedString([]byte("wrong"))
	if status, _ := do("/me", forged); status != http.StatusUnauthorized {
		fail("forged token: expected 401, got %d", status)
	}

	log.Infof(context.Background(), log.TagAppDef, "example: all checks passed")
	os.Exit(0)
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, "example: "+format, args...)
	os.Exit(1)
}
