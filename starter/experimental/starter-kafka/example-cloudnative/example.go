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

// Command example is a flagship for starter-kafka that composes the cloud-
// native capability set around a Kafka (franz-go) client:
//
//   - ROUND-TRIP: the client op is a produce -> consume through the broker;
//     publishing via GuardedProduceSync routes the sync produce path through
//     the selected resilience driver.
//   - RESILIENCE: with spring.kafka.a.resilience.enabled, synchronous produces
//     run through the builtin "default" executor; a burst is rejected with
//     ErrRateLimited. (Consume is not guarded: franz-go's poll path is passive.)
//   - HEALTH: the app exports its own kafka health.Indicator (Ping probe), which
//     starter-actuator aggregates on :9370 (/readyz reflects the broker).
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file; editing
//     it hot-reloads the value with no restart.
//   - OBSERVABILITY: kotel spans/metrics + the observe access-log hook ride the
//     OTel globals installed by starter-otel when present.
//
// Discovery is intentionally omitted: Kafka seeds its clients with bootstrap
// brokers (spring.kafka.a.brokers), so there is no service-name to resolve.
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

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator"    // aggregates health.Indicator beans on :9370
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	_ "go-spring.org/starter-governance"  // centralized governance center (govern.* config)
	StarterKafka "go-spring.org/starter-kafka"
)

const (
	topic    = "hello"
	mountDir = "./mount"
)

// Config binds a hot-reloadable label sourced from the watched mount.
type Config struct {
	Label gs.Dync[string] `value:"${demo.label:=none}"`
}

// Service autowires the "a" kafka instance.
type Service struct {
	Client *kgo.Client `autowire:"a"`
}

// Health returns the app's kafka health indicator: the starter itself
// registers no health.Indicator, so exporting one here lets the actuator
// aggregate a Ping probe against the broker.
func (s *Service) Health() *health.Indicator {
	return &health.Indicator{
		Name: "kafka:a",
		// The broker belongs to readiness + startup (never liveness, so a
		// broker outage cannot restart the pod); an unreachable broker is
		// critical and fails the aggregate probe.
		Groups: []health.Group{health.GroupReadiness, health.GroupStartup},
		Probe: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return s.Client.Ping(pingCtx)
		},
	}
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
	gs.Provide((*Service).Health)

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

// publish sends a single record and waits for the broker ack, routed through the
// resilience executor attached to the client (a no-op pass-through when
// resilience is disabled).
func (s *Service) publish(ctx context.Context, value string) error {
	rec := &kgo.Record{Topic: topic, Value: []byte(value)}
	return StarterKafka.GuardedProduceSync(ctx, s.Client, rec).FirstErr()
}

// consume polls fetches until a record with the wanted value is seen (consuming
// can lag producing), honoring ctx for a bounded wait.
func (s *Service) consume(ctx context.Context, want string) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fetches := s.Client.PollFetches(ctx)
		if err := fetches.Err(); err != nil {
			return err
		}
		var found bool
		fetches.EachRecord(func(r *kgo.Record) {
			if string(r.Value) == want {
				found = true
			}
		})
		if found {
			return nil
		}
	}
}

func runTest(s *Service, c *Config) {
	ctx := context.Background()

	// --- 1. Health --------------------------------------------------------
	mustStatus("http://127.0.0.1:9370/readyz", http.StatusOK)
	mustStatus("http://127.0.0.1:9370/health", http.StatusOK)
	fmt.Println("health: actuator probes UP (kafka Ping health.Indicator aggregated)")

	// --- 2. Produce -> consume round-trip (the client op) -----------------
	if err := s.publish(ctx, "cn-value"); err != nil {
		fail("produce: %v", err)
	}
	pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.consume(pollCtx, "cn-value"); err != nil {
		fail("consume: %v", err)
	}
	fmt.Println("round-trip: produced -> consumed through the broker OK")

	// --- 3. Resilience ----------------------------------------------------
	var admitted, rejected int
	for range 40 {
		rec := &kgo.Record{Topic: topic, Value: []byte("rl")}
		err := StarterKafka.GuardedProduceSync(ctx, s.Client, rec).FirstErr()
		switch {
		case err == nil:
			admitted++
		case errors.Is(err, resilience.ErrRateLimited):
			rejected++
		default:
			fail("produce: %v", err)
		}
	}
	if admitted == 0 || rejected == 0 {
		fail("resilience ineffective: admitted=%d rejected=%d", admitted, rejected)
	}
	fmt.Printf("resilience: %d produce admitted, %d rejected with ErrRateLimited\n", admitted, rejected)

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
