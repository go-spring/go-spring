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

	fmt.Println("Response from server: lua filter allowed /hello, blocked /admin, then allowed /admin with token")
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
// resides, so the relative Lua script path in app.properties resolves.
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
}
