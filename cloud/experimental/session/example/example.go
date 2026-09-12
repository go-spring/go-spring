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

// Command example demonstrates cloud/experimental/session in a Go-Spring
// application over the bundled in-process Memory store: mounting the Manager
// middleware, lazy session allocation, typed attribute access, session-id
// rotation on login, logout invalidation, and idle expiry.
//
// The store arrives as a bean. starter.go next to this file is a local
// contributor starter that turns each spring.session.memory.instances.<name>
// config entry into a session.SessionStore; a distributed backend is the same
// bean type from a published starter (starter-session-redis) behind the sibling
// config key, and nothing else in this file changes.
//
// It self-asserts every step and exits non-zero on mismatch, so it doubles as
// the package's smoke test. No external services are required.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/experimental/session"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
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

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// One Manager, one store: the middleware is the single seam where session
	// transport lives; handlers only ever see the request context.
	gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
		mgr := session.NewManager(store, session.Options{IdleTimeout: idleTimeout})

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

		return &gs.HttpServeMux{Handler: mux}
	})

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

const base = "http://127.0.0.1:9092"

func runTest() {
	// 1. Lazy allocation: a request that never touches the session gets no
	// cookie and creates no store entry.
	body, cookie := get("/public", "")
	if body != "public" || cookie != "" {
		fail("anonymous request: body=%q cookie=%q, want no cookie", body, cookie)
	}
	fmt.Println("untouched request issued no cookie: OK")

	// 2. Before login there is no session to read.
	if body, _ = get("/profile", ""); body != "anonymous" {
		fail("pre-login profile: got %q, want %q", body, "anonymous")
	}

	// 3. Login rotates the id and stores a typed attribute; the old cookie the
	// client may have held is no longer valid.
	body, cookie = get("/login?name=alice", "stale-pre-auth-id")
	if body != "ok" || cookie == "" || cookie == "stale-pre-auth-id" {
		fail("login: body=%q cookie=%q, want a rotated id", body, cookie)
	}
	fmt.Println("login rotated the session id: OK")

	// 4. The next request reads the attribute back through the typed getter.
	if body, _ = get("/profile", cookie); body != "alice" {
		fail("profile: got %q, want %q", body, "alice")
	}
	fmt.Println("typed attribute survived the round trip: OK")

	// 5. Logout destroys the session; the cookie is expired and the id is dead.
	if body, _ = get("/logout", cookie); body != "bye" {
		fail("logout: got %q, want %q", body, "bye")
	}
	if body, _ = get("/profile", cookie); body != "anonymous" {
		fail("post-logout profile: got %q, want %q", body, "anonymous")
	}
	fmt.Println("logout invalidated the session: OK")

	// 6. Idle expiry: after the timeout with no intervening request, the
	// session is gone even though logout never ran.
	_, cookie = get("/login?name=bob", "")
	time.Sleep(idleTimeout + 500*time.Millisecond)
	if body, _ = get("/profile", cookie); body != "anonymous" {
		fail("expired profile: got %q, want %q", body, "anonymous")
	}
	fmt.Println("session expired after idle timeout: OK")

	fmt.Println("session example ok")
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// get performs one GET, optionally carrying a session cookie, and returns the
// response body plus the session cookie set by the response (empty when none
// or when it was cleared).
func get(path, cookie string) (body, sessionCookie string) {
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		fail("build request %s: %v", path, err)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "SESSION", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		fail("read %s: %v", path, err)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "SESSION" && c.MaxAge >= 0 {
			sessionCookie = c.Value
		}
	}
	return string(b), sessionCookie
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

// init pins the working directory to this source file's directory so relative
// config paths resolve regardless of how the binary is invoked.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
