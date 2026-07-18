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
	StarterLuaFilter "go-spring.org/starter-lua-filter"
)

func main() {
	// Provide a *gs.HttpServeMux whose handler is the business mux wrapped by
	// the "guard" Lua filter. Because gs registers the default HttpServeMux only
	// when none is present, this custom one wins and every request passes
	// through the Lua script first — regardless of the web framework behind it.
	gs.Provide(func(guard *StarterLuaFilter.Filter) *gs.HttpServeMux {
		guardFilter = guard
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("hello"))
		})
		mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("admin ok"))
		})
		return &gs.HttpServeMux{Handler: guard.Wrap(mux)}
	}, gs.TagArg("guard"))

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl -i http://127.0.0.1:9090/hello
	// HTTP/1.1 200 OK
	// X-Lua-Filter: guard
	// hello
	//
	// ~ curl -i http://127.0.0.1:9090/admin
	// HTTP/1.1 403 Forbidden
	// forbidden: bad token
	//
	// ~ curl -i -H 'X-Token: sesame' http://127.0.0.1:9090/admin
	// HTTP/1.1 200 OK
	// admin ok
}

// guardFilter is captured at wiring time so runTest can trigger a hot reload.
var guardFilter *StarterLuaFilter.Filter

// scriptPath is a runtime-owned copy of the Lua script, so the hot-reload demo
// can rewrite it without touching the checked-in fixture.
var scriptPath string

func runTest() {
	// Feature 1: a normal request passes and carries the header the Lua filter
	// injected on every response.
	status, hdr, body := do("GET", "/hello", "")
	if status != 200 || body != "hello" || hdr != "guard" {
		fail("hello: status=%d hdr=%q body=%q", status, hdr, body)
	}

	// Feature 2: the filter short-circuits /admin without the token — the
	// business handler is never reached.
	status, _, body = do("GET", "/admin", "")
	if status != 403 {
		fail("admin without token: expected 403, got %d body=%q", status, body)
	}

	// Feature 3: the same path passes once the script's condition is satisfied.
	status, _, body = do("GET", "/admin", "sesame")
	if status != 200 || body != "admin ok" {
		fail("admin with token: status=%d body=%q", status, body)
	}

	// Feature 4: hot reload — rewrite the script to also gate /hello, then
	// reload it at runtime. Subsequent requests pick up the new rules without a
	// restart. A bad edit would fail Reload and leave the running script intact.
	rewriteScript(`resp.set_header("X-Lua-Filter", "guard")
if req.path == "/hello" then
    deny(403, "hello disabled")
    return
end
`)
	if err := guardFilter.Reload(); err != nil {
		fail("reload failed: %v", err)
	}
	status, _, body = do("GET", "/hello", "")
	if status != 403 {
		fail("hello after reload: expected 403, got %d body=%q", status, body)
	}

	fmt.Println("Response from server: lua filter allowed /hello, blocked /admin, then blocked /hello after hot reload")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// do issues a request and returns status, the X-Lua-Filter response header, and
// the body. token, when non-empty, is sent as X-Token.
func do(method, path, token string) (int, string, string) {
	req, err := http.NewRequest(method, "http://127.0.0.1:9090"+path, nil)
	if err != nil {
		fail("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("do %s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("X-Lua-Filter"), string(b)
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory to the directory where this source file
// resides, then seeds a writable copy of the Lua script and points the starter
// at it via a GS_ environment override, so the hot-reload demo can rewrite the
// script without mutating the checked-in fixture.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)

	src, err := os.ReadFile("./scripts/guard.lua")
	if err != nil {
		panic(err)
	}
	f, err := os.CreateTemp("", "guard-*.lua")
	if err != nil {
		panic(err)
	}
	if _, err := f.Write(src); err != nil {
		panic(err)
	}
	_ = f.Close()
	scriptPath = f.Name()

	// GS_SPRING_LUA_FILTER_GUARD_SCRIPT overrides
	// spring.lua.filter.guard.script from app.properties.
	if err := os.Setenv("GS_SPRING_LUA_FILTER_GUARD_SCRIPT", scriptPath); err != nil {
		panic(err)
	}
}

// rewriteScript replaces the runtime script file's contents, staging a new
// version for the next Reload.
func rewriteScript(src string) {
	if err := os.WriteFile(scriptPath, []byte(src), 0o600); err != nil {
		fail("rewrite script: %v", err)
	}
}
