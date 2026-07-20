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
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/session"

	// Blank-import both starters: starter-go-redis publishes the *redis.Client
	// under spring.go-redis.<name>, and starter-session-redis contributes a
	// session.SessionStore per spring.session.redis.<name> that reuses that
	// client by name.
	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-session-redis"
)

// idleTimeout is deliberately short so the smoke test can prove expiry without a
// long wait. Every request that carries a session slides this deadline forward.
const idleTimeout = 2 * time.Second

func main() {
	// mgrA and mgrB share ONE Redis-backed store. They model two replicas: no
	// in-process state is shared between them, only Redis. What A writes, B reads.
	gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
		opt := session.Options{IdleTimeout: idleTimeout}
		mgrA := session.NewManager(store, opt)
		mgrB := session.NewManager(store, opt)

		mux := http.NewServeMux()

		// Replica A: writes an attribute.
		mux.Handle("/a/set", mgrA.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, _ := session.FromContext(r.Context())
			s.Set("user", r.URL.Query().Get("user"))
			_, _ = w.Write([]byte("ok"))
		})))

		// Replica A: rotates the session id (session-fixation defense on login)
		// while keeping the attributes.
		mux.Handle("/a/login", mgrA.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, _ := session.FromContext(r.Context())
			s.RenewID()
			_, _ = w.Write([]byte("ok"))
		})))

		// Replica B: reads the attribute A wrote — only possible via the shared store.
		mux.Handle("/b/get", mgrB.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, _ := session.FromContext(r.Context())
			if v, ok := s.Get("user"); ok {
				_, _ = w.Write([]byte(v.(string)))
			}
		})))

		return &gs.HttpServeMux{Handler: mux}
	}, gs.TagArg("web"))

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest()
	}()

	gs.Run()
}

const base = "http://127.0.0.1:9090"

func runTest() {
	// Feature 1: cross-replica sharing. A writes user=alice and hands back a
	// cookie; B, sharing only Redis, reads it back.
	cookie := doExpect("/a/set?user=alice", "", "ok")
	if cookie == "" {
		fail("no session cookie issued by /a/set")
	}
	fmt.Println("A wrote session, cookie=", cookie)

	if body := getWith("/b/get", cookie); body != "alice" {
		fail("cross-replica read: expected \"alice\", got %q", body)
	}
	fmt.Println("B read \"alice\" from the shared store: OK")

	// Feature 2: session-fixation defense. Login rotates the id; the new cookie
	// differs, the attributes survive, and the OLD id is gone from the store.
	newCookie := doExpect("/a/login", cookie, "ok")
	if newCookie == "" || newCookie == cookie {
		fail("login did not rotate the session id (old=%q new=%q)", cookie, newCookie)
	}
	if body := getWith("/b/get", newCookie); body != "alice" {
		fail("after renew: expected \"alice\", got %q", body)
	}
	if body := getWith("/b/get", cookie); body != "" {
		fail("old session id should be dead after renew, got %q", body)
	}
	fmt.Println("Session id rotated on login, old id destroyed: OK")

	// Feature 3: idle expiry. Let the session sit idle past the timeout, then a
	// read finds nothing.
	time.Sleep(idleTimeout + time.Second)
	if body := getWith("/b/get", newCookie); body != "" {
		fail("session should have expired after idle timeout, got %q", body)
	}
	fmt.Println("Session expired after idle timeout: OK")

	fmt.Println("starter-session-redis smoke test passed")
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// doExpect sends GET path (optionally carrying cookie), asserts the body equals
// want, and returns the session cookie value from the response (or "" if none).
func doExpect(path, cookie, want string) string {
	body, setCookie := get(path, cookie)
	if body != want {
		fail("%s: expected body %q, got %q", path, want, body)
	}
	return setCookie
}

// getWith sends GET path carrying cookie and returns the body.
func getWith(path, cookie string) string {
	body, _ := get(path, cookie)
	return body
}

// get performs one GET, returning the response body and the SESSION cookie value
// set by the response (empty when none or when it was cleared).
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
	b, _ := io.ReadAll(resp.Body)
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
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve source file directory")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	fmt.Println(wd)
}
