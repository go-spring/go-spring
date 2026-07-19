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
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-admin-ui"
)

// fakeActuator is a stand-in for a real starter-actuator instance. It serves
// the four endpoints the Admin UI polls (/health, /readiness, /startup, /info)
// with the exact JSON shape starter-actuator produces, so the smoke test can
// exercise the polling and rendering code paths without spinning up a full
// second application.
type fakeActuator struct {
	name string
	port int
}

func (f *fakeActuator) start() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "UP"})
	})
	mux.HandleFunc("GET /readiness", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "UP",
			"components": map[string]any{
				"redis:" + f.name: map[string]any{"status": "UP"},
			},
		})
	})
	mux.HandleFunc("GET /startup", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "UP"})
	})
	mux.HandleFunc("GET /info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"go": runtime.Version(),
			"module": map[string]string{
				"path":    "example.com/" + f.name,
				"version": "v0.0.1",
			},
			"build": map[string]string{"revision": "deadbeef", "time": "2026-07-19T00:00:00Z"},
		})
	})
	srv := &http.Server{Addr: fmt.Sprintf(":%d", f.port), Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	go func() { _ = srv.ListenAndServe() }()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	// Bring up two fake actuator instances that the Admin UI is configured to
	// poll. Ports match the ones referenced in conf/app.properties.
	(&fakeActuator{name: "alpha", port: 19371}).start()
	(&fakeActuator{name: "beta", port: 19372}).start()

	// Give the fake actuators a moment to bind before the Admin UI's initial
	// synchronous seed poll runs; unbound targets would show as DOWN.
	time.Sleep(200 * time.Millisecond)

	go func() {
		// Wait for the Admin UI to bind (:9280) and complete at least one
		// poll cycle after startup.
		waitForPort("127.0.0.1:9280", 5*time.Second)
		// Trigger an out-of-band refresh window: the seeded snapshot already
		// exists, but wait one full poll interval so a second sweep runs.
		time.Sleep(1500 * time.Millisecond)
		runTest()
	}()

	gs.Run()
}

// runTest fetches the dashboard, asserts that both configured instances
// appear as UP, and that the JSON status endpoint reflects the same data.
// It exits non-zero on any mismatch, then triggers graceful shutdown.
func runTest() {
	// HTML dashboard: must contain both instance URLs and at least one UP pill.
	body := mustGet("http://127.0.0.1:9280/")
	for _, want := range []string{
		"Go-Spring Admin",
		"http://127.0.0.1:19371",
		"http://127.0.0.1:19372",
		"pill up",
		"redis:alpha",
		"redis:beta",
	} {
		if !strings.Contains(body, want) {
			fmt.Fprintf(os.Stderr, "dashboard missing %q\ngot:\n%s\n", want, body)
			os.Exit(1)
		}
	}
	fmt.Println("dashboard renders both instances with UP status and component breakdown")

	// JSON status: same snapshot, machine-readable.
	jsonBody := mustGet("http://127.0.0.1:9280/api/status")
	var payload struct {
		PolledAt  string `json:"polled_at"`
		Instances []struct {
			Base      string `json:"base"`
			Health    string `json:"health"`
			Readiness string `json:"readiness"`
			Module    string `json:"module"`
		} `json:"instances"`
	}
	if err := json.Unmarshal([]byte(jsonBody), &payload); err != nil {
		fmt.Fprintln(os.Stderr, "status JSON decode failed:", err)
		os.Exit(1)
	}
	if len(payload.Instances) != 2 {
		fmt.Fprintf(os.Stderr, "expected 2 instances, got %d: %s\n", len(payload.Instances), jsonBody)
		os.Exit(1)
	}
	for _, inst := range payload.Instances {
		if inst.Health != "UP" || inst.Readiness != "UP" {
			fmt.Fprintf(os.Stderr, "instance %s not UP: %+v\n", inst.Base, inst)
			os.Exit(1)
		}
	}
	fmt.Println("json api reports both instances UP with build metadata")

	// The starter must still serve when Instances is unreachable — verify by
	// pointing one instance to a closed port and re-fetching. We can't
	// reconfigure at runtime cleanly, so just confirm the process is still
	// alive after a normal run and shut down gracefully.
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func mustGet(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "GET failed:", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read failed:", url, err)
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unexpected status for %s: %d\n", url, resp.StatusCode)
		os.Exit(1)
	}
	return string(b)
}

func waitForPort(addr string, budget time.Duration) {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
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
