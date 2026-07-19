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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank import registers the gateway module (Server + route table) with the
	// container. Routes are pure config (see conf/app.properties), so no
	// application bean is required for the gateway to serve.
	_ "go-spring.org/starter-gateway"
)

// backendAddr is the in-process upstream the gateway forwards to. It stands in
// for a real microservice; the routes in conf/app.properties target it directly
// via http://127.0.0.1:19000.
const backendAddr = "127.0.0.1:19000"

func main() {
	// Start the upstream the gateway proxies to. It echoes the (post-filter) path
	// and reflects the X-From header the addRequestHeader filter injects, so the
	// test can prove both proxying and filtering happened.
	startBackend()

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// Run the Go-Spring application. The gateway server listens on :9440.
	gs.Run()

	// Example usage:
	//
	// ~ curl -i http://127.0.0.1:9440/api/echo
	// HTTP/1.1 200 OK
	// path=/echo from=gw           # /api stripped, X-From injected
	//
	// ~ curl -i http://127.0.0.1:9440/nope
	// HTTP/1.1 404 Not Found       # no route matches
}

// startBackend runs a tiny HTTP upstream that reports the path it received and
// the X-From header, then blocks until it is accepting connections.
func startBackend() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s from=%s", r.URL.Path, r.Header.Get("X-From"))
	})
	ln, err := net.Listen("tcp", backendAddr)
	if err != nil {
		fail("backend listen: %v", err)
	}
	go func() { _ = http.Serve(ln, mux) }()
}

func runTest() {
	// Feature 1: a request under /api/** is routed, its /api prefix stripped, and
	// the addRequestHeader filter injects X-From before the upstream sees it.
	status, body := do("GET", "/api/echo")
	if status != 200 || body != "path=/echo from=gw" {
		fail("api route: status=%d body=%q", status, body)
	}

	// Feature 2: a path matching no route's predicates gets a clean 404 from the
	// gateway itself — the upstream is never contacted.
	status, _ = do("GET", "/nope")
	if status != 404 {
		fail("unmatched route: expected 404, got %d", status)
	}

	fmt.Println("Response from server: gateway routed /api/** to the upstream (prefix stripped, header injected) and 404'd an unmatched path")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// do issues a request to the gateway's listen port and returns the status and
// body.
func do(method, path string) (int, string) {
	req, err := http.NewRequest(method, "http://127.0.0.1:9440"+path, nil)
	if err != nil {
		fail("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("do %s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

// init sets the working directory to this source file's directory so the
// relative conf/app.properties path resolves regardless of where `go run` is
// invoked from.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
