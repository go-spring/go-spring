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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
	"go-spring.org/stdlib/health"
)

// demoIndicator is a stand-in for a real dependency (a database pool, a cache
// client, ...). Any bean exported as health.Indicator is folded into the
// actuator's /readiness aggregate with no extra wiring; here we register one so
// the smoke test can observe both the UP and DOWN paths.
type demoIndicator struct {
	down atomic.Bool
}

func (d *demoIndicator) HealthName() string { return "demo:dependency" }

func (d *demoIndicator) CheckHealth(ctx context.Context) error {
	if d.down.Load() {
		return errors.New("dependency unavailable")
	}
	return nil
}

// dep is registered as a health.Indicator and kept here so runTest can toggle
// it to exercise the DOWN path.
var dep = &demoIndicator{}

func init() {
	// Contribute the indicator to the actuator. Because the actuator collects
	// every bean exported as health.Indicator, this is the whole integration —
	// no import of the actuator package and no per-component registration API.
	gs.Provide(dep).Export(gs.As[health.Indicator]())
}

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest()
	}()

	// Run the Go-Spring application. The actuator serves on :9370 by default:
	//
	// ~ curl http://127.0.0.1:9370/health
	// ~ curl http://127.0.0.1:9370/readiness
	// ~ curl http://127.0.0.1:9370/info
	gs.Run()
}

// runTest asserts the three endpoints behave as documented: /health is always
// UP, /readiness reflects the aggregated indicator (UP, then DOWN once the
// dependency is toggled), and /info returns build metadata. It exits non-zero
// on any failure, then triggers a graceful shutdown.
func runTest() {
	const base = "http://127.0.0.1:9370"

	// Liveness is up as soon as the process serves.
	mustStatus(base+"/health", http.StatusOK)
	fmt.Println("health OK")

	// The app has reported readiness and the dependency is healthy -> 200.
	mustStatus(base+"/readiness", http.StatusOK)
	fmt.Println("readiness UP")

	// Build info is served.
	mustStatus(base+"/info", http.StatusOK)
	fmt.Println("info OK")

	// Toggle the dependency down; readiness must now fail with 503 while
	// liveness stays up (a degraded dependency must not trip liveness).
	dep.down.Store(true)
	mustStatus(base+"/readiness", http.StatusServiceUnavailable)
	mustStatus(base+"/health", http.StatusOK)
	fmt.Println("readiness DOWN when dependency down, health still UP")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// mustStatus fetches url and exits the process non-zero unless the response
// status matches want.
func mustStatus(url string, want int) {
	resp, err := http.Get(url)
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
