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

// Command example is a flagship for starter-neo4j that composes the cloud-
// native capability set around a graph client:
//
//   - DISCOVERY: the neo4j address is resolved through a registered discovery
//     backend (service-name), not hardcoded config. Because the neo4j driver
//     exposes no dialer injection point, resolution happens once at startup.
//   - RESILIENCE: with resilience.enabled, queries routed through
//     StarterNeo4j.Query / RunWithResilience run through the builtin "default"
//     executor; a burst is rejected with ErrRateLimited.
//   - HEALTH: the per-instance neo4j health.Indicator is aggregated by
//     starter-actuator on :9370 (/readyz reflects the server).
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file; editing
//     it hot-reloads the value with no restart.
//   - OBSERVABILITY: StarterNeo4j.Query rides the observe kit on the OTel globals.
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

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator"    // aggregates the neo4j health.Indicator
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	_ "go-spring.org/starter-governance"
	StarterNeo4j "go-spring.org/starter-neo4j"
)

const mountDir = "./mount"

// Config binds a hot-reloadable label sourced from the watched mount.
type Config struct {
	Label gs.Dync[string] `value:"${demo.label:=none}"`
}

func init() {
	// A real deployment would point Consul/Nacos/k8s here; a static backend keeps
	// the example self-contained while exercising the same resolve + dial path.
	// The neo4j client resolves "neo4j-cluster" through this "default" backend
	// bean.
	gs.Provide(func() (discovery.Discovery, error) {
		return discovery.NewStaticDiscovery(
			discovery.Endpoint{Addr: "127.0.0.1:7687", Healthy: true},
		), nil
	}).Name("default")
}

// Service autowires the "graph" neo4j instance. Its address is resolved via
// discovery (service-name), its queries are protected by resilience, and its
// connectivity is exported as a health.Indicator the actuator collects.
type Service struct {
	Neo4j *StarterNeo4j.Client `autowire:"graph"`
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
	fmt.Println("health: actuator probes UP (neo4j health.Indicator aggregated)")

	// --- 2. Discovery -----------------------------------------------------
	// The instance's address comes from the discovery backend, not config. A
	// successful Cypher round-trip proves resolve -> dial -> serve.
	if err := queryAge(ctx, s, "MERGE (p:Person {name: $name}) SET p.age = $age RETURN p",
		map[string]any{"name": "alice", "age": 30}); err != nil {
		fail("neo4j via discovery (write): %v", err)
	}
	res, err := s.query(ctx,
		"MATCH (p:Person {name: $name}) RETURN p.age AS age",
		map[string]any{"name": "alice"})
	if err != nil || len(res.Records) == 0 {
		fail("neo4j via discovery (read): err=%v records=%d", err, len(res.Records))
	}
	age, _ := res.Records[0].Get("age")
	if age != int64(30) {
		fail("neo4j via discovery: age expected 30, got %v", age)
	}
	fmt.Println("discovery: neo4j resolved via backend -> Cypher round-trip OK")

	// --- 3. Resilience ----------------------------------------------------
	// Queries routed through RunWithResilience are protected by the builtin
	// "default" executor; a burst over rate-limit is rejected with ErrRateLimited.
	var admitted, rejected int
	for range 15 {
		err := StarterNeo4j.RunWithResilience(ctx, s.Neo4j, func(ctx context.Context) error {
			return queryAge(ctx, s, "MERGE (p:Person {name: $name}) SET p.age = $age RETURN p",
				map[string]any{"name": "bob", "age": 40})
		})
		switch {
		case err == nil:
			admitted++
		case errors.Is(err, resilience.ErrRateLimited):
			rejected++
		default:
			fail("resilient query: %v", err)
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

// query runs a Cypher statement through the instrumented StarterNeo4j.Query entry
// point (trace + metric + access log + resilience guard) and returns the eager
// result.
func (s *Service) query(ctx context.Context, cypher string, params map[string]any) (*neo4j.EagerResult, error) {
	return StarterNeo4j.Query(ctx, s.Neo4j, cypher, params, neo4j.EagerResultTransformer)
}

func queryAge(ctx context.Context, s *Service, cypher string, params map[string]any) error {
	_, err := s.query(ctx, cypher, params)
	return err
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

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

// init sets the working directory to this source file's directory so relative
// config lookups (conf/) resolve against the source location.
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
