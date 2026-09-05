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

// example demonstrates the security middleware kit of starter-http-server on
// top of the framework's built-in stdlib HTTP server: a bearer-authenticated
// API route, an authority-gated admin route, and a browser-style CSRF-guarded
// route. It self-asserts every gate before exiting 0.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	httpsvr "go-spring.org/starter-http-server"
)

const base = "http://127.0.0.1:9090"

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

// staticValidator accepts exactly one bearer token holding the "admin"
// authority. A real app injects the TokenValidator bean contributed by
// starter-oauth2-resource-server or starter-security-jwt instead.
type staticValidator struct{}

func (staticValidator) Validate(_ context.Context, token string) (*security.Authentication, error) {
	if token != "good-token" {
		return nil, errors.New("bad token")
	}
	return &security.Authentication{
		Principal:     security.Principal{Subject: "alice"},
		Token:         token,
		Authenticated: true,
		Authorities:   []string{"admin"},
	}, nil
}

func main() {
	flag.Parse()

	gs.Provide(func() *gs.HttpServeMux {
		v := staticValidator{}
		mux := http.NewServeMux()

		// /api/me requires a valid token; /api/admin additionally requires the
		// "admin" authority — the ordered chain authenticates before it
		// authorizes.
		mux.Handle("/api/me", httpsvr.Chain(
			httpsvr.Authenticate(v, true),
		)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, _ := security.FromContext(r.Context())
			_, _ = fmt.Fprintf(w, "hello %s", a.Principal.Subject)
		})))
		mux.Handle("/api/admin", httpsvr.Chain(
			httpsvr.Authenticate(v, true),
			httpsvr.Authorize("admin"),
		)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("admin ok"))
		})))

		// A browser-style, cookie-based route guarded by CSRF double-submit.
		mux.Handle("/session", httpsvr.CSRF(httpsvr.CSRFConfig{})(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("session ok"))
			})))

		return &gs.HttpServeMux{Handler: mux}
	})

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running on", base, "— press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest asserts every gate and exits non-zero on the first failure.
func runTest() {
	if code, _ := do("/api/me", ""); code != http.StatusUnauthorized {
		fail("missing token: status = %d, want 401", code)
	}
	if code, _ := do("/api/me", "bad-token"); code != http.StatusUnauthorized {
		fail("invalid token: status = %d, want 401", code)
	}
	if code, body := do("/api/me", "good-token"); code != http.StatusOK || body != "hello alice" {
		fail("valid token: status = %d body = %q, want 200 hello alice", code, body)
	}
	if code, body := do("/api/admin", "good-token"); code != http.StatusOK || body != "admin ok" {
		fail("admin: status = %d body = %q, want 200 admin ok", code, body)
	}

	// CSRF: the safe GET issues the cookie, and an unsafe POST echoing the
	// cookie token in the header is accepted while a bare POST is rejected.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(base + "/session")
	if err != nil {
		fail("session GET: %v", err)
	}
	var token string
	for _, c := range resp.Cookies() {
		if c.Name == security.DefaultCSRFCookieName {
			token = c.Value
		}
	}
	if token == "" {
		fail("session GET did not issue a csrf cookie")
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/session", nil)
	req.AddCookie(&http.Cookie{Name: security.DefaultCSRFCookieName, Value: token})
	req.Header.Set(security.DefaultCSRFHeaderName, token)
	if resp, err = client.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		fail("session POST with token: err = %v status = %v, want 200", err, respStatus(resp))
	}

	log.Infof(context.Background(), log.TagAppDef, "smoke ok: all security gates verified")
	os.Exit(0)
}

func do(path, token string) (int, string) {
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
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
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n])
}

func respStatus(resp *http.Response) int {
	if resp == nil {
		return -1
	}
	return resp.StatusCode
}

func fail(format string, args ...any) {
	fmt.Printf("FAIL: "+format+"\n", args...)
	os.Exit(1)
}
