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

// Command example is a self-contained flagship for starter-echo that composes
// the full cloud-native capability set in one app, with no external services or
// docker required:
//
//   - HEALTH: a health.Indicator bean is aggregated by starter-actuator on its
//     management port (:9370); toggling it flips /readyz between 200 and 503.
//   - DISCOVERY: a static discovery backend hands out the app's own echo address;
//     the app resolves it client-side and dials the endpoint end-to-end.
//   - RESILIENCE: the builtin "default" driver rate-limits an arbitrary function
//     (Executor.Execute -> ErrRateLimited) and an inbound route (/limited -> 429).
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file
//     (file-watch provider); editing it hot-reloads the value into /greeting.
//   - OBSERVABILITY: starter-echo's Tracing/Metrics/AccessLog middleware are on
//     by default, riding the OTel globals.
//
// The app self-tests every capability and exits non-zero on failure, so check.sh
// needs no infrastructure to prove the whole stack works.
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

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator"    // registers the actuator Server bean
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	StarterEcho "go-spring.org/starter-echo"
)

const mountDir = "./mount"

// Config binds a hot-reloadable greeting sourced from the watched mount.
type Config struct {
	Greeting gs.Dync[string] `value:"${demo.greeting:=none}"`
}

// depDown toggles the demo dependency's health so runTest can exercise both the
// UP and DOWN probe paths.
var depDown atomic.Bool

// dep stands in for a real dependency's health check; the actuator
// aggregates every bean exported as health.Indicator.
var dep = &health.Indicator{
	Name:   "demo:dependency",
	Groups: []health.Group{health.GroupReadiness, health.GroupStartup},
	Probe: func(ctx context.Context) error {
		if depDown.Load() {
			return errors.New("dependency unavailable")
		}
		return nil
	},
}

// exec is the resilience executor built from the builtin "default" driver.
var exec resilience.Executor

func init() {
	// Serve the app's own echo address through discovery (a real deployment
	// would point Consul/Nacos/k8s here; a static backend keeps it self-contained).
	// The backend is a named bean — the container is the discovery directory —
	// cited by the clients' `discovery: static` config key.
	gs.Provide(func() (discovery.Discovery, error) {
		return discovery.NewStaticDiscovery(
			discovery.Endpoint{Addr: "127.0.0.1:8082", Healthy: true},
		), nil
	}).Name("static")

	gs.Provide(dep)
	gs.Provide(&Config{}).Export(gs.As[gs.Rooter]())
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	_ = os.RemoveAll(mountDir)
	if err := writeConfigMap("demo.greeting=hello\n"); err != nil {
		fmt.Fprintln(os.Stderr, "setup mount failed:", err)
		os.Exit(1)
	}

	driver, err := resilience.GetDriver("default")
	if err != nil {
		fail("resilience driver: %v", err)
	}
	exec, err = driver.NewExecutor(resilience.Policy{RateLimit: 3})
	if err != nil {
		fail("resilience executor: %v", err)
	}

	// Wire the echo routes. The RouterRegister receives the Config bean through
	// the container, so /greeting reads the live Dync value.
	gs.Provide(func(c *Config) StarterEcho.RouterRegister {
		return func(e *echo.Echo) {
			e.GET("/", func(ctx echo.Context) error {
				return ctx.JSON(http.StatusOK, map[string]any{"app": "cloudnative"})
			})

			e.GET("/greeting", func(ctx echo.Context) error {
				return ctx.JSON(http.StatusOK, map[string]any{"greeting": c.Greeting.Value()})
			})

			// A rate-limited route: the handler body runs through the resilience
			// executor, so excess requests are shed with HTTP 429.
			e.GET("/limited", func(ctx echo.Context) error {
				err := exec.Execute(ctx.Request().Context(), "app:limited", func(context.Context) error {
					return ctx.String(http.StatusOK, "ok")
				})
				if errors.Is(err, resilience.ErrRateLimited) {
					return ctx.String(http.StatusTooManyRequests, "too many requests")
				}
				return err
			})
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

const (
	appBase      = "http://127.0.0.1:8082" // starter-echo business port
	actuatorBase = "http://127.0.0.1:9370" // starter-actuator management port
)

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
	// The smoke test runs outside the container, so it re-builds the same
	// static backend the init above registered as the "static" bean.
	resolver, err := discovery.NewResolver(ctx, discovery.NewStaticDiscovery(
		discovery.Endpoint{Addr: "127.0.0.1:8082", Healthy: true},
	), "cloudnative-echo")
	if err != nil {
		fail("new resolver: %v", err)
	}
	bal, err := loadbalance.New(loadbalance.RoundRobin)
	if err != nil {
		fail("new balancer: %v", err)
	}
	ep, err := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal).Pick(loadbalance.PickInfo{})
	if err != nil {
		fail("resolve: %v", err)
	}
	if ep.Addr != "127.0.0.1:8082" {
		fail("unexpected resolved addr: %s", ep.Addr)
	}
	mustStatus("http://"+ep.Addr+"/", http.StatusOK)
	fmt.Printf("discovery: resolved %q -> dialed OK\n", ep.Addr)

	// --- 3. Resilience ---------------------------------------------------
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

// writeConfigMap writes application.properties into the mount. It writes the
// watched file directly (a WRITE event on the watched dir fires a refresh); the
// k8s-style atomic ..data swap is demonstrated by the starter-config-file
// example, which owns that concern.
func writeConfigMap(propsContent string) error {
	if err := os.MkdirAll(mountDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(mountDir, "application.properties"), []byte(propsContent), 0o644)
}

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

func bodyOrEmpty(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return string(b)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

// init sets the working directory to this source file's directory so relative
// config lookups (conf/) and ./mount resolve against the source location.
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
