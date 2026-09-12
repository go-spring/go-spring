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

// Package main is the observability example for starter-bigcache.
//
// It exercises a BigCache instance (SET hits + GET misses) so the OTel gauges
// registered in starter-bigcache report non-zero values, then scrapes
// the Prometheus pull exporter served by starter-otel at :9090/metrics and
// verifies the bigcache.* metrics appear labeled with the instance name. No
// external service is required - the prometheus exporter is in-process.
//
// Run with -manual to keep the server running for interactive exploration.
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

	"github.com/allegro/bigcache/v3"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
	_ "go-spring.org/starter-otel"
)

// Service injects the "hot" BigCache instance. The bean is created by
// starter-bigcache under ${spring.bigcache.instances.hot}; starter-bigcache
// registers its OTel gauges (labeled cache.name="hot") as a side effect of
// constructing the client. The bean is the *Cache wrapper (bigcache
// has no hook extension point), so per-op span/metric/log are emitted by the
// Get/Set calls below; the OTel gauges above are independent.
type Service struct {
	Hot *StarterBigCache.Cache `autowire:"hot"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Scrape metrics at http://localhost:9090/metrics")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// Generate traffic that produces both hits and misses, so the gauges read
	// non-zero values when scraped.
	for i := 0; i < 20; i++ {
		if err := s.Hot.Set(fmt.Sprintf("hit-%d", i), []byte("v")); err != nil {
			log.Errorf(ctx, log.TagAppDef, "SET failed: %v", err)
			os.Exit(1)
		}
		if _, err := s.Hot.Get(fmt.Sprintf("hit-%d", i)); err != nil {
			log.Errorf(ctx, log.TagAppDef, "GET hit failed: %v", err)
			os.Exit(1)
		}
	}
	// Misses: read keys that were never set.
	for i := 0; i < 5; i++ {
		if _, err := s.Hot.Get(fmt.Sprintf("absent-%d", i)); !errors.Is(err, bigcache.ErrEntryNotFound) {
			log.Errorf(ctx, log.TagAppDef, "expected entry-not-found for absent key, got err=%v", err)
			os.Exit(1)
		}
	}
	fmt.Println("Sent 20 SET/GET (hits) + 5 GET (misses)")

	// Let the exporter settle, then scrape. The pull exporter calls the gauge
	// callbacks on scrape, so this read reflects the stats right now.
	time.Sleep(time.Second)

	body, err := httpGet("http://127.0.0.1:9090/metrics")
	if err != nil {
		fmt.Fprintln(os.Stderr, "metrics scrape failed:", err)
		os.Exit(1)
	}

	// OTel gauge "bigcache.hits" renders as the Prometheus metric
	// "bigcache_hits"; the "cache.name" attribute renders as "cache_name".
	for _, want := range []string{`bigcache_hits{`, `bigcache_misses{`, `cache_name="hot"`} {
		if !strings.Contains(body, want) {
			fmt.Fprintf(os.Stderr, "metrics: %q not found in /metrics output\n", want)
			os.Exit(1)
		}
	}
	fmt.Println("OK: bigcache hits/misses gauges found in /metrics (cache.name=hot)")

	// Confirm the hit gauge reports a non-zero value, proving the gauge wired
	// up to Stats() rather than a never-updated zero.
	if !strings.Contains(body, `bigcache_hits{cache_name="hot"} 0`) {
		fmt.Println("OK: bigcache_hits is non-zero")
	} else {
		fmt.Fprintln(os.Stderr, "bigcache_hits is 0 - gauge did not observe the hits")
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
