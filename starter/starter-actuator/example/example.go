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
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
)

// depDown toggles the demo dependency's health so runTest can observe both the
// UP and DOWN probe paths.
var depDown atomic.Bool

// dep stands in for a real dependency's health check (a database pool, a
// cache client, ...). Any bean exported as health.Indicator is folded into
// the actuator's /readiness aggregate with no extra wiring; here we register
// one so the smoke test can observe both the UP and DOWN paths.
var dep = &health.Indicator{
	Name: "demo:dependency",
	Probe: func(ctx context.Context) error {
		if depDown.Load() {
			return errors.New("dependency unavailable")
		}
		return nil
	},
}

func init() {
	// Contribute the indicator to the actuator. Because the actuator collects
	// every bean exported as health.Indicator, this is the whole integration —
	// no import of the actuator package and no per-component registration API.
	gs.Provide(dep)
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond)
			runTest()
		}()
	} else {

		// Run the Go-Spring application. The actuator serves on :9370 by default:
		//
		// ~ curl http://127.0.0.1:9370/health
		// ~ curl http://127.0.0.1:9370/readiness
		// ~ curl http://127.0.0.1:9370/info

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest asserts the endpoints behave as documented: /health is always UP,
// /readiness reflects the aggregated indicator (UP, then DOWN once the
// dependency is toggled, then UP again), /startup reports readiness completion,
// and /info returns build metadata. It then triggers a graceful shutdown and
// asserts the drain sequence flips /readiness to OUT_OF_SERVICE while liveness
// stays up. It exits non-zero on any failure.
func runTest() {
	const base = "http://127.0.0.1:9370"

	// The management port is guarded by a bearer token: without the header
	// every endpoint answers 401, including the probes.
	mustStatusNoAuth(base+"/health", http.StatusUnauthorized)
	fmt.Println("auth 401 without token OK")

	// Liveness is up as soon as the process serves.
	mustStatus(base+"/health", http.StatusOK)
	fmt.Println("health OK")

	// The app has reported readiness and the dependency is healthy -> 200.
	mustStatus(base+"/readiness", http.StatusOK)
	fmt.Println("readiness UP")

	// Startup has completed (readiness barrier crossed) -> 200.
	mustStatus(base+"/startup", http.StatusOK)
	fmt.Println("startup OK")

	// Build info is served.
	mustStatus(base+"/info", http.StatusOK)
	fmt.Println("info OK")

	// The z-suffixed canonical probe paths behave like their legacy aliases.
	mustStatus(base+"/healthz", http.StatusOK)
	mustStatus(base+"/readyz", http.StatusOK)
	mustStatus(base+"/startupz", http.StatusOK)
	fmt.Println("healthz/readyz/startupz OK")

	// Toggle the dependency down; readiness must now fail with 503 while
	// liveness stays up (a degraded dependency must not trip liveness).
	depDown.Store(true)
	mustStatus(base+"/readiness", http.StatusServiceUnavailable)
	mustStatus(base+"/health", http.StatusOK)
	fmt.Println("readiness DOWN when dependency down, health still UP")

	// Restore the dependency so the subsequent 503 is attributable to draining,
	// not to the indicator.
	depDown.Store(false)
	mustStatus(base+"/readiness", http.StatusOK)

	// Trigger graceful shutdown. The server flips readiness to OUT_OF_SERVICE and
	// finishes its own drain before stopping. Poll until /readiness reports 503
	// while /health stays 200, proving the pod would be drained from Service
	// endpoints before it stops accepting traffic.
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if statusOf(base+"/readiness") == http.StatusServiceUnavailable {
			mustStatus(base+"/health", http.StatusOK)
			fmt.Println("readiness OUT_OF_SERVICE during drain, health still UP")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "readiness did not flip to OUT_OF_SERVICE during drain")
	os.Exit(1)
}

// actuatorToken mirrors spring.actuator.token in conf/app.properties: every
// request the smoke test makes must present it as a bearer token.
const actuatorToken = "dev-actuator-token"

// authedGet issues a GET against url, presenting the actuator bearer token
// unless auth is false.
func authedGet(url string, auth bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+actuatorToken)
	}
	return http.DefaultClient.Do(req)
}

// statusOf returns the HTTP status code for an authenticated GET of url, or -1
// on error (for example once the server has stopped serving at the end of the
// drain window).
func statusOf(url string) int {
	resp, err := authedGet(url, true)
	if err != nil {
		return -1
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// mustStatusNoAuth fetches url WITHOUT credentials and exits non-zero unless
// the response status matches want (used to assert the guard rejects).
func mustStatusNoAuth(url string, want int) {
	resp, err := authedGet(url, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", url, err)
		os.Exit(1)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		fmt.Fprintf(os.Stderr, "unexpected status for %s: got %d want %d\n", url, resp.StatusCode, want)
		os.Exit(1)
	}
}

// mustStatus fetches url with the bearer token and exits the process non-zero
// unless the response status matches want.
func mustStatus(url string, want int) {
	resp, err := authedGet(url, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", url, err)
		os.Exit(1)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		fmt.Fprintf(os.Stderr, "unexpected status for %s: got %d want %d\n", url, resp.StatusCode, want)
		os.Exit(1)
	}
}

// fail prints a message to stderr and exits non-zero.
func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides, so relative config lookups (conf/) resolve
// against the source location rather than the process launch path.
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
