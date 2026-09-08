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

// Command example is a flagship for starter-elasticsearch that composes the
// cloud-native capability set around an Elasticsearch client:
//
//   - DISCOVERY: the node addresses are resolved through a registered discovery
//     backend (service-name), not hardcoded config.
//   - RESILIENCE: with resilience.enabled, every request runs through the
//     builtin "default" executor; a burst is rejected with ErrRateLimited.
//   - HEALTH: the per-instance elasticsearch health.Indicator is aggregated by
//     starter-actuator on :9370 (/readyz reflects the cluster).
//   - DYNAMIC CONFIG: a gs.Dync[string] field is bound to a watched file; editing
//     it hot-reloads the value with no restart.
//   - OBSERVABILITY: the observe kit rides the OTel globals.
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
	"strings"
	"syscall"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-actuator"    // aggregates the elasticsearch health.Indicator
	_ "go-spring.org/starter-config-file" // registers the file-watch config provider
	StarterElasticsearch "go-spring.org/starter-elasticsearch"
	_ "go-spring.org/starter-governance"
)

const mountDir = "./mount"

// Config binds a hot-reloadable label sourced from the watched mount.
type Config struct {
	Label gs.Dync[string] `value:"${demo.label:=none}"`
}

func init() {
	// A real deployment would point Consul/Nacos/k8s here; a static backend keeps
	// the example self-contained while exercising the same resolve + dial path.
	// The elasticsearch client resolves "es-cluster" through this "default"
	// backend bean into "http://127.0.0.1:9200" node addresses.
	gs.Provide(func() (discovery.Discovery, error) {
		return discovery.NewStaticDiscovery(
			discovery.Endpoint{Addr: "127.0.0.1:9200", Healthy: true},
		), nil
	}).Name("default")
}

const indexName = "cn-docs"

// Service autowires the "docs" elasticsearch instance. Its node addresses are
// resolved via discovery (service-name), its requests are protected by
// resilience, and its cluster health is exported as a health.Indicator the
// actuator collects.
type Service struct {
	ES *StarterElasticsearch.Client `autowire:"docs"`
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
	// A non-nil parent context is mandatory: DefaultDriver's transport installs
	// OTel instrumentation that derives its span from the request context and
	// panics on a nil parent. Every Index/Get must carry a WithContext.
	ctx := context.Background()

	// --- 1. Health --------------------------------------------------------
	mustStatus("http://127.0.0.1:9370/readyz", http.StatusOK)
	mustStatus("http://127.0.0.1:9370/health", http.StatusOK)
	fmt.Println("health: actuator probes UP (elasticsearch health.Indicator aggregated)")

	// --- 2. Discovery + round-trip ----------------------------------------
	// The instance's node addresses come from the discovery backend, not config
	// (the dummy static `addresses` is overridden because service-name is set). A
	// successful index -> get -> search proves resolve -> dial -> serve.
	body := `{"title":"hello","views":1}`
	idxRes, err := s.ES.Index(indexName, strings.NewReader(body),
		s.ES.Index.WithContext(ctx),
		s.ES.Index.WithDocumentID("1"),
		s.ES.Index.WithRefresh("true"))
	if err != nil {
		fail("index: %v", err)
	}
	if idxRes.IsError() {
		fail("index: res=%v", idxRes)
	}
	closeBody(idxRes.Body)

	getRes, err := s.ES.Get(indexName, "1", s.ES.Get.WithContext(ctx))
	if err != nil {
		fail("get: %v", err)
	}
	if getRes.IsError() {
		fail("get: res=%v", getRes)
	}
	getBody, _ := io.ReadAll(getRes.Body)
	closeBody(getRes.Body)
	if !strings.Contains(string(getBody), `"found":true`) {
		fail("get did not find the document: %s", getBody)
	}
	fmt.Println("discovery: elasticsearch resolved via backend -> index/get round-trip OK")

	// --- 3. Resilience ----------------------------------------------------
	var admitted, rejected int
	for range 15 {
		_, err := s.ES.Index(indexName, strings.NewReader(body),
			s.ES.Index.WithContext(ctx),
			s.ES.Index.WithDocumentID("rl"),
			s.ES.Index.WithRefresh("true"))
		switch {
		case err == nil:
			admitted++
		case errors.Is(err, resilience.ErrRateLimited):
			rejected++
		default:
			fail("index: %v", err)
		}
	}
	if admitted == 0 || rejected == 0 {
		fail("resilience ineffective: admitted=%d rejected=%d", admitted, rejected)
	}
	fmt.Printf("resilience: %d index admitted, %d rejected with ErrRateLimited\n", admitted, rejected)

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

// closeBody drains and closes a response body if non-nil.
func closeBody(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
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
