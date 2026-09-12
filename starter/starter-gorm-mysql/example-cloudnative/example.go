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

// Command example is a flagship for starter-gorm-mysql that composes the cloud-
// native capability set around a storage client:
//
//   - DISCOVERY: the DB address is resolved through a registered discovery
//     backend (service-name), not hardcoded config.
//   - RESILIENCE: with resilience.enabled, every query runs through the builtin
//     "default" executor; a burst is rejected with ErrRateLimited.
//   - HEALTH: the per-instance gorm health.Indicator is aggregated by
//     starter-actuator on :9370 (/readyz reflects the pool).
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file; editing
//     it hot-reloads the value with no restart.
//   - OBSERVABILITY: gorm observe plugin + observe kit ride the OTel globals.
//
// The app self-tests every capability and exits non-zero on failure.
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
	"syscall"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator"    // aggregates the gorm health.Indicator
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	gormcore "go-spring.org/starter-gorm"
	_ "go-spring.org/starter-gorm-mysql"
	_ "go-spring.org/starter-governance" // registers the centralized governance center
)

const mountDir = "./mount"

// Config binds a hot-reloadable label sourced from the watched mount.
type Config struct {
	Label gs.Dync[string] `value:"${demo.label:=none}"`
}

func init() {
	// A real deployment would point Consul/Nacos/k8s here; a static backend keeps
	// the example self-contained while exercising the same resolve + dial path.
	// The DB client resolves "mysql-cluster" through this "default" backend.
	gs.Provide(func() (discovery.Discovery, error) {
		return discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:3306", Healthy: true}), nil
	}).Name("default")
}

// Service autowires the "primary" gorm instance. Its address is resolved via
// discovery (service-name), its queries are protected by resilience, and its
// pool health is exported as a health.Indicator the actuator collects.
type Service struct {
	DB *gormcore.DB `autowire:"mysql.primary"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	_ = os.RemoveAll(mountDir)
	if err := writeConfigMap("demo.label=hello\n"); err != nil {
		fmt.Fprintln(os.Stderr, "setup mount failed:", err)
		os.Exit(1)
	}

	cfg := gs.Provide(&Config{}).Export(gs.As[gs.Rooter]())
	svc := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svc.Interface().(*Service), cfg.Interface().(*Config))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest(s *Service, c *Config) {
	ctx := context.Background()

	// --- 1. Health --------------------------------------------------------
	mustStatus("http://127.0.0.1:9370/readyz", http.StatusOK)
	mustStatus("http://127.0.0.1:9370/health", http.StatusOK)
	fmt.Println("health: actuator probes UP (gorm health.Indicator aggregated)")

	// --- 2. Discovery -----------------------------------------------------
	// The DB's address comes from the discovery backend, not config. A successful
	// query proves resolve -> dial -> serve.
	if err := s.DB.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
		fail("gorm via discovery: %v", err)
	}
	fmt.Println("discovery: gorm resolved via backend -> query OK")

	// --- 3. Resilience ----------------------------------------------------
	var admitted, rejected int
	for range 15 {
		err := s.DB.WithContext(ctx).Exec("SELECT 1").Error
		switch {
		case err == nil:
			admitted++
		case errors.Is(err, resilience.ErrRateLimited):
			rejected++
		default:
			fail("query: %v", err)
		}
	}
	if admitted == 0 || rejected == 0 {
		fail("resilience ineffective: admitted=%d rejected=%d", admitted, rejected)
	}
	fmt.Printf("resilience: %d queries admitted, %d rejected with ErrRateLimited\n", admitted, rejected)

	// --- 4. Dynamic configuration ----------------------------------------
	if got := c.Label.Value(); got != "hello" {
		fail("initial label: %q", got)
	}
	fmt.Println("dynamic config: initial label OK")

	want := "updated-" + time.Now().Format("150405")
	if err := writeConfigMap("demo.label=" + want + "\n"); err != nil {
		fail("update mount: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := c.Label.Value(); got == want {
			fmt.Println("dynamic config: hot-reload observed:", want)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fail("dynamic config hot-reload timeout")
}

// writeConfigMap writes application.properties into the mount. It writes the
// watched file directly (a WRITE event on the watched dir fires a refresh).
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
