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

// Command cloudnative is a self-contained flagship example that composes the
// four cross-cutting cloud-native capabilities go-spring ships, in one app, with
// no external services or docker required:
//
//   - HEALTH: a health.Indicator bean is aggregated by starter-actuator on its
//     management port (:9370). Toggling the indicator flips /readyz between 200
//     and 503 while /health stays up.
//   - DISCOVERY: a static discovery backend (discovery.NewStaticDiscovery)
//     hands out the app's own gin address; the app resolves it client-side and
//     dials the returned endpoint end-to-end.
//   - RESILIENCE: the builtin "default" driver wraps an arbitrary function and
//     an inbound HTTP route under a rate limit; excess calls are rejected with
//     ErrRateLimited / HTTP 429.
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file
//     (file-watch provider). Rewriting the file the way the kubelet does
//     (atomic ..data symlink swap) hot-reloads the value into live traffic.
//
// The application self-tests every capability and exits non-zero on failure, so
// check.sh needs no infrastructure to prove the whole stack works.
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
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator"    // registers the actuator Server bean (gated on spring.actuator.addr)
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	StarterGin "go-spring.org/starter-gin"
)

const mountDir = "./mount"

// ----------------------------------------------------------------------------
// Dynamic configuration bean
// ----------------------------------------------------------------------------

// Config binds a hot-reloadable greeting sourced from the watched mount. The
// value:"${demo.greeting:=none}" tag makes it a gs.Dync field: it is re-bound on
// every property refresh, so editing the mounted file propagates to the /greeting
// handler without a restart.
type Config struct {
	Greeting gs.Dync[string] `value:"${demo.greeting:=none}"`
}

// ----------------------------------------------------------------------------
// Health indicator
// ----------------------------------------------------------------------------

// depDown toggles the demo dependency's health so runTest can exercise both the
// UP and DOWN probe paths.
var depDown atomic.Bool

// dep is a health.Indicator built with the health.NewIndicator helper; it stands
// in for a real dependency (a database pool, a cache client, ...). The actuator
// collects every bean exported as health.Indicator, so this is the whole
// integration — no import of the actuator package and no per-component
// registration API.
var dep = health.NewIndicator(
	"demo:dependency",
	func(ctx context.Context) error {
		if depDown.Load() {
			return errors.New("dependency unavailable")
		}
		return nil
	},
	// A dependency belongs to readiness (and startup), never liveness, so a
	// degraded dependency takes the pod out of rotation without restarting it.
	health.WithGroups(health.GroupReadiness, health.GroupStartup),
)

// exec is the resilience executor built from the builtin "default" driver. It
// is shared by the /limited route and by the arbitrary-function demo.
var exec resilience.Executor

// ----------------------------------------------------------------------------
// Bean wiring
// ----------------------------------------------------------------------------

func init() {
	// Serve the app's own gin address through discovery: a real deployment would
	// point a Consul/Nacos/k8s backend here; a static backend keeps the example
	// self-contained while demonstrating the exact same client-side resolve +
	// dial path.
	discovery.RegisterDiscovery("static", discovery.NewStaticDiscovery(
		discovery.Endpoint{Addr: "127.0.0.1:8081", Healthy: true},
	))

	// Contribute the health indicator to the actuator. Because the actuator
	// collects every bean exported as health.Indicator, this is the whole
	// integration — no import of the actuator package and no per-component
	// registration API.
	gs.Provide(dep).Export(gs.As[health.Indicator]())

	// The config bean is a root object so the container creates it eagerly and
	// binds its Dync field at startup.
	gs.Provide(&Config{}).Export(gs.As[gs.Rooter]())
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// Clean up leftover mount from a prior run so the symlink swap always starts
	// from a known-good state.
	_ = os.RemoveAll(mountDir)

	// Lay down the initial ConfigMap-style mount before the app starts so the
	// file-watch import resolves at startup.
	if err := writeConfigMap("demo.greeting=hello\n"); err != nil {
		fmt.Fprintln(os.Stderr, "setup mount failed:", err)
		os.Exit(1)
	}

	// Build the resilience executor from the builtin "default" driver (registered
	// by cloud/governance/resilience on import — no external dependency).
	driver, err := resilience.GetDriver("default")
	if err != nil {
		fail("resilience driver: %v", err)
	}
	exec, err = driver.NewExecutor(resilience.Policy{RateLimit: 3})
	if err != nil {
		fail("resilience executor: %v", err)
	}

	// Wire the gin routes. The RouterRegister receives the Config bean through
	// the container, so the /greeting handler reads the live Dync value.
	gs.Provide(func(c *Config) StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			e.GET("/", func(ctx *gin.Context) {
				ctx.JSON(http.StatusOK, gin.H{"app": "cloudnative"})
			})

			// The greeting reflects the hot-reloaded config value.
			e.GET("/greeting", func(ctx *gin.Context) {
				ctx.JSON(http.StatusOK, gin.H{"greeting": c.Greeting.Value()})
			})

			// A rate-limited route: admission is enforced by the resilience
			// server-side seam, so excess requests are shed with HTTP 429 before
			// the business handler runs.
			limited := &admissionHandler{
				next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte("ok"))
				}),
				exec:     exec,
				resource: func(*http.Request) string { return "app:limited" },
			}
			e.GET("/limited", gin.WrapH(limited))
		}
	})

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// ----------------------------------------------------------------------------
// Self-test
// ----------------------------------------------------------------------------

const (
	appBase      = "http://127.0.0.1:8081" // starter-gin business port
	actuatorBase = "http://127.0.0.1:9370" // starter-actuator management port
)

// runTest exercises all four capabilities and exits non-zero on any failure.
// On success it triggers SIGTERM so check.sh observes a clean exit (and the
// graceful-drain path runs as a bonus).
func runTest() {
	ctx := context.Background()

	// --- 1. Health -------------------------------------------------------
	mustStatus(actuatorBase+"/readyz", http.StatusOK)
	depDown.Store(true)
	mustStatus(actuatorBase+"/readyz", http.StatusServiceUnavailable)
	mustStatus(actuatorBase+"/health", http.StatusOK) // degraded dep must not trip liveness
	depDown.Store(false)
	mustStatus(actuatorBase+"/readyz", http.StatusOK)
	fmt.Println("health: readiness aggregate OK (UP -> DOWN when dependency down -> UP)")

	// --- 2. Discovery ----------------------------------------------------
	r, err := discovery.NewResolver(ctx, "static", "cloudnative-app")
	if err != nil {
		fail("new resolver: %v", err)
	}
	ep, err := r.Pick()
	if err != nil {
		fail("resolve: %v", err)
	}
	_ = r.Stop()
	if ep.Addr != "127.0.0.1:8081" {
		fail("unexpected resolved addr: %s", ep.Addr)
	}
	// Dial the resolved endpoint end-to-end: the discovery address points at the
	// app itself, so a 200 proves resolve -> dial -> serve with no hardcoded
	// connection.
	mustStatus("http://"+ep.Addr+"/", http.StatusOK)
	fmt.Printf("discovery: resolved %q -> dialed OK\n", ep.Addr)

	// --- 3. Resilience ---------------------------------------------------
	// (a) Wrap an arbitrary function under the rate limit. Firing 15 calls
	// through the 3 QPS executor must reject a non-empty tail with
	// ErrRateLimited while still admitting a non-empty head.
	var admitted, rejected int
	for range 15 {
		err := exec.Execute(ctx, "app:fn", func(context.Context) error { return nil })
		switch {
		case err == nil:
			admitted++
		case errors.Is(err, resilience.ErrRateLimited):
			rejected++
		default:
			fail("execute: %v", err)
		}
	}
	if admitted == 0 || rejected == 0 {
		fail("resilience rate limit ineffective: admitted=%d rejected=%d", admitted, rejected)
	}
	fmt.Printf("resilience: Executor rate limit admitted=%d rejected=%d\n", admitted, rejected)

	// (b) Inbound admission on /limited sheds excess requests with 429.
	okHTTP, limitedHTTP := 0, 0
	for range 15 {
		resp, err := http.Get(appBase + "/limited")
		if err != nil {
			fail("limited request: %v", err)
		}
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			okHTTP++
		case http.StatusTooManyRequests:
			limitedHTTP++
		default:
			fail("limited unexpected status %d", resp.StatusCode)
		}
	}
	if okHTTP == 0 || limitedHTTP == 0 {
		fail("resilience /limited ineffective: ok=%d limited=%d", okHTTP, limitedHTTP)
	}
	fmt.Printf("resilience: /limited admitted=%d rejected=%d\n", okHTTP, limitedHTTP)

	// --- 4. Dynamic configuration ----------------------------------------
	body := mustBody(appBase + "/greeting")
	if !strings.Contains(body, "hello") {
		fail("initial greeting missing: %s", body)
	}
	fmt.Println("dynamic config: initial greeting OK")

	want := "world-" + time.Now().Format("150405")
	if err := writeConfigMap("demo.greeting=" + want + "\n"); err != nil {
		fail("update mount: %v", err)
	}

	// The directory watcher observes the atomic ..data swap and triggers a
	// refresh, which re-binds the Dync field; the /greeting handler then serves
	// the new value. Poll until it appears or time out.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := bodyOrEmpty(appBase + "/greeting"); strings.Contains(got, want) {
			fmt.Println("dynamic config: hot-reload observed:", want)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fail("dynamic config hot-reload timeout")
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// writeConfigMap writes application.properties into the mount using the same
// atomic scheme as the kubelet: the payload goes into a fresh timestamped data
// directory, then the ..data symlink is atomically renamed onto it. The key
// symlink (application.properties -> ..data/application.properties) is created
// once and survives the swap because it points through ..data.
func writeConfigMap(propsContent string) error {
	const key = "application.properties"

	if err := os.MkdirAll(mountDir, 0o755); err != nil {
		return err
	}

	dataDir := fmt.Sprintf("..%d", time.Now().UnixNano())
	dataPath := filepath.Join(mountDir, dataDir)
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dataPath, key), []byte(propsContent), 0o644); err != nil {
		return err
	}

	dataLink := filepath.Join(mountDir, "..data")
	tmpLink := filepath.Join(mountDir, "..data_tmp")
	_ = os.Remove(tmpLink)
	if err := os.Symlink(dataDir, tmpLink); err != nil {
		return err
	}
	if err := os.Rename(tmpLink, dataLink); err != nil {
		return err
	}

	keyLink := filepath.Join(mountDir, key)
	if _, err := os.Lstat(keyLink); os.IsNotExist(err) {
		if err := os.Symlink(filepath.Join("..data", key), keyLink); err != nil {
			return err
		}
	}
	return nil
}

// mustStatus fetches url and exits non-zero unless the response status matches.
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

// mustBody GETs url and returns the body, exiting non-zero on error/non-200.
func mustBody(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		fail("request %s: %v", url, err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail("unexpected status for %s: got %d want 200", url, resp.StatusCode)
	}
	return string(b)
}

// bodyOrEmpty GETs url and returns the body ("" on any error), for polling.
func bodyOrEmpty(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return string(b)
}

// fail prints a message to stderr and exits non-zero.
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory where
// this source file resides, so relative config lookups (conf/) and the ./mount
// path resolve against the source location rather than the launch path.
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

// admissionHandler is the application's own server-side admission wrapper: each
// request flows through a resilience.Executor so rate limiting / bulkhead /
// breaker shed overload with HTTP 429/503 before the business handler runs.
// Inbound serving is not retried (the Executor's policy carries MaxRetries=0),
// since handlers are not idempotent. Server-side resilience is the application's
// job — the resilience library is client-side — so this seam lives here, not in
// the library. It is a minimal, static-policy shedder; for adaptive load shedding
// (AIMD, feedback-based) wire a dedicated limiter here instead.
type admissionHandler struct {
	next     http.Handler
	exec     resilience.Executor
	resource func(*http.Request) string
}

func (h *admissionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	served := false
	err := h.exec.Execute(r.Context(), h.resource(r), func(ctx context.Context) error {
		if served {
			return nil
		}
		served = true
		h.next.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})
	if err != nil && !served {
		switch {
		case errors.Is(err, resilience.ErrCircuitOpen):
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		default: // ErrRateLimited, ErrBulkheadFull
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		}
	}
}
