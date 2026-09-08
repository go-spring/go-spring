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

// Command example is a flagship for starter-mongodb that composes the cloud-
// native capability set around a MongoDB client:
//
//   - DISCOVERY: the "disc" instance's address is resolved through a registered
//     discovery backend (service-name), not hardcoded config.
//   - RESILIENCE: with resilience.enabled, every connection dial flows through
//     the builtin "default" executor; a burst of fresh connections over the
//     rate-limit is rejected with ErrRateLimited (the seam is the dial layer —
//     mongo exposes no per-command hook).
//   - HEALTH: the per-instance mongodb health.Indicator is aggregated by
//     starter-actuator on :9370 (/readyz reflects the client pool).
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file; editing
//     it hot-reloads the value with no restart.
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
	"sync"
	"syscall"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"
	"go.mongodb.org/mongo-driver/v2/bson"

	_ "go-spring.org/starter-actuator"    // aggregates the mongodb health.Indicator
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	_ "go-spring.org/starter-governance"
	StarterMongoDB "go-spring.org/starter-mongodb"
)

const mountDir = "./mount"

// Config binds a hot-reloadable label sourced from the watched mount.
type Config struct {
	Label gs.Dync[string] `value:"${demo.label:=none}"`
}

func init() {
	// A real deployment would point Consul/Nacos/k8s here; a static backend keeps
	// the example self-contained while exercising the same resolve + dial path.
	// The "disc" client resolves "mongo-cluster" through this "default" backend
	// bean.
	gs.Provide(func() (discovery.Discovery, error) {
		return discovery.NewStaticDiscovery(
			discovery.Endpoint{Addr: "127.0.0.1:27017", Healthy: true},
		), nil
	}).Name("default")
}

// Service autowires two mongodb instances. "a" is dialed directly from its URI
// and carries the CRUD + resilience checks; "disc" resolves its address via
// discovery (service-name), proving resolve -> dial -> serve. The pool health of
// both is exported as health.Indicators the actuator collects.
type Service struct {
	Mongo *StarterMongoDB.Client `autowire:"a"`
	Disc  *StarterMongoDB.Client `autowire:"disc"`
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
			time.Sleep(time.Millisecond * 800)
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
	fmt.Println("health: actuator probes UP (mongodb health.Indicators aggregated)")

	// --- 2. Client round-trip ---------------------------------------------
	// CRUD against the direct instance proves a live connection end to end.
	coll := s.Mongo.Database("test").Collection("kv")
	if err := coll.Drop(ctx); err != nil {
		fail("drop: %v", err)
	}
	if _, err := coll.InsertOne(ctx, bson.M{"key": "cn:key", "value": "cn-value"}); err != nil {
		fail("insert: %v", err)
	}
	var got bson.M
	if err := coll.FindOne(ctx, bson.M{"key": "cn:key"}).Decode(&got); err != nil {
		fail("find: %v", err)
	}
	if fmt.Sprint(got["value"]) != "cn-value" {
		fail("find value mismatch: got=%v", got["value"])
	}
	fmt.Println("client: direct instance InsertOne/FindOne round-trip OK")

	// --- 3. Discovery -----------------------------------------------------
	// The "disc" instance's address comes from the discovery backend, not config.
	// A successful round-trip proves resolve -> dial -> serve.
	discColl := s.Disc.Database("test").Collection("disc")
	if err := discColl.Drop(ctx); err != nil {
		fail("disc drop: %v", err)
	}
	if _, err := discColl.InsertOne(ctx, bson.M{"key": "cn:key", "value": "disc-value"}); err != nil {
		fail("disc insert: %v", err)
	}
	var discGot bson.M
	if err := discColl.FindOne(ctx, bson.M{"key": "cn:key"}).Decode(&discGot); err != nil {
		fail("disc find: %v", err)
	}
	if fmt.Sprint(discGot["value"]) != "disc-value" {
		fail("disc find value mismatch: got=%v", discGot["value"])
	}
	fmt.Println("discovery: disc resolved via backend -> InsertOne/FindOne round-trip OK")

	// --- 4. Resilience ----------------------------------------------------
	// The mongodb seam is the dial layer (mongo has no per-command hook), so a
	// burst of fresh connections is what gets rate-limited. The pool starts empty;
	// firing many concurrent ops forces the driver to establish new connections
	// at once, and the dials beyond the rate-limit are rejected with ErrRateLimited.
	admitted, rejected, other := resilienceBurst(s)
	if admitted == 0 || rejected == 0 {
		fail("resilience ineffective: admitted=%d rejected=%d other=%d", admitted, rejected, other)
	}
	fmt.Printf("resilience: %d dials admitted, %d rejected with ErrRateLimited (other=%d)\n",
		admitted, rejected, other)

	// --- 5. Dynamic configuration ----------------------------------------
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

// resilienceBurst fires many concurrent operations against the direct instance.
// With the pool empty and max-pool-size large, each op forces a fresh connection
// dial through the resilience executor; the dials over rate-limit are rejected
// with ErrRateLimited (surfaced as the operation's connection error).
func resilienceBurst(s *Service) (admitted, rejected, other int) {
	const n = 40
	ctx := context.Background()
	burstColl := s.Mongo.Database("test").Collection("burst")
	_ = burstColl.Drop(ctx)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := burstColl.InsertOne(ctx, bson.M{"i": i, "k": "v"})
			mu.Lock()
			switch {
			case err == nil:
				admitted++
			case errors.Is(err, resilience.ErrRateLimited):
				rejected++
			default:
				other++
				fmt.Printf("  burst op other error: %v\n", err)
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return admitted, rejected, other
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
