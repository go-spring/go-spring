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

// Command example demonstrates cloud/session against the bundled in-process
// Memory store: mounting the Manager middleware, lazy session allocation,
// typed attribute access, session-id rotation on login, logout invalidation,
// and idle expiry.
//
// It self-asserts every step and exits non-zero on mismatch, so it doubles as
// the package's smoke test. No external services are required. The shared
// (Redis) backend variant lives in starter-session-redis/example.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"go-spring.org/cloud/session"
)

// user is an attribute value read back through the typed session.Get.
type user struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// idleTimeout is deliberately short so the example can prove idle expiry
// without a long wait. Every request that carries a session slides this
// deadline forward.
const idleTimeout = 2 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("session example ok")
}

func run() error {
	// One Manager, one store: the middleware is the single seam where session
	// transport lives; handlers only ever see the request context.
	mgr := session.NewManager(session.NewMemory(), session.Options{IdleTimeout: idleTimeout})
	mux := http.NewServeMux()

	mux.Handle("/public", mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler never touches the session: no store entry, no cookie.
		_, _ = w.Write([]byte("public"))
	})))

	mux.Handle("/login", mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		// Rotate the id the client presented pre-authentication (session-fixation
		// defense), then record who logged in.
		s.RenewID()
		session.Set(s, "user", user{ID: 7, Name: r.URL.Query().Get("name")})
		_, _ = w.Write([]byte("ok"))
	})))

	mux.Handle("/profile", mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		u, ok, err := session.Get[user](s, "user")
		if err != nil || !ok {
			_, _ = w.Write([]byte("anonymous"))
			return
		}
		_, _ = fmt.Fprintf(w, "%s", u.Name)
	})))

	mux.Handle("/logout", mgr.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := session.FromContext(r.Context())
		s.Invalidate()
		_, _ = w.Write([]byte("bye"))
	})))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Lazy allocation: a request that never touches the session gets no
	// cookie and creates no store entry.
	body, cookie, err := get(srv.URL+"/public", "")
	if err != nil {
		return err
	}
	if body != "public" || cookie != "" {
		return fmt.Errorf("anonymous request: body=%q cookie=%q, want no cookie", body, cookie)
	}
	fmt.Println("untouched request issued no cookie: OK")

	// 2. Before login there is no session to read.
	if body, _, err = get(srv.URL+"/profile", ""); err != nil {
		return err
	}
	if body != "anonymous" {
		return fmt.Errorf("pre-login profile: got %q, want %q", body, "anonymous")
	}

	// 3. Login rotates the id and stores a typed attribute; the old cookie the
	// client may have held is no longer valid.
	body, cookie, err = get(srv.URL+"/login?name=alice", "stale-pre-auth-id")
	if err != nil {
		return err
	}
	if body != "ok" || cookie == "" || cookie == "stale-pre-auth-id" {
		return fmt.Errorf("login: body=%q cookie=%q, want a rotated id", body, cookie)
	}
	fmt.Println("login rotated the session id: OK")

	// 4. The next request reads the attribute back through the typed getter.
	if body, _, err = get(srv.URL+"/profile", cookie); err != nil {
		return err
	}
	if body != "alice" {
		return fmt.Errorf("profile: got %q, want %q", body, "alice")
	}
	fmt.Println("typed attribute survived the round trip: OK")

	// 5. Logout destroys the session; the cookie is expired and the id is dead.
	if body, _, err = get(srv.URL+"/logout", cookie); err != nil {
		return err
	}
	if body != "bye" {
		return fmt.Errorf("logout: got %q, want %q", body, "bye")
	}
	if body, _, err = get(srv.URL+"/profile", cookie); err != nil {
		return err
	}
	if body != "anonymous" {
		return fmt.Errorf("post-logout profile: got %q, want %q", body, "anonymous")
	}
	fmt.Println("logout invalidated the session: OK")

	// 6. Idle expiry: after the timeout with no intervening request, the
	// session is gone even though logout never ran.
	_, cookie, err = get(srv.URL+"/login?name=bob", "")
	if err != nil {
		return err
	}
	time.Sleep(idleTimeout + 500*time.Millisecond)
	if body, _, err = get(srv.URL+"/profile", cookie); err != nil {
		return err
	}
	if body != "anonymous" {
		return fmt.Errorf("expired profile: got %q, want %q", body, "anonymous")
	}
	fmt.Println("session expired after idle timeout: OK")

	return nil
}

// get performs one GET, optionally carrying a session cookie, and returns the
// response body plus the session cookie set by the response (empty when none
// or when it was cleared).
func get(url, cookie string) (body, sessionCookie string, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "SESSION", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	for _, c := range resp.Cookies() {
		if c.Name == "SESSION" && c.MaxAge >= 0 {
			sessionCookie = c.Value
		}
	}
	return string(b), sessionCookie, nil
}
