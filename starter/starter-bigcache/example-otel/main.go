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
	"strconv"
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
// registers its OTel gauges (labeled instance="hot") as a side effect of
// constructing the client. The bean is the *Cache wrapper (bigcache
// has no hook extension point), so the per-op counter is emitted by
// the wrapper's own observe step as the Get/Set calls below run - there is no
// executor chain underneath; the OTel gauges above are independent of it.
type Service struct {
	Hot *StarterBigCache.Cache `autowire:"hot"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Drop the marks the launching shell left in the environment, so the example
	// does not depend on which terminal started it. `_` is the one that matters:
	// single-character variable names are not portable - on Windows the shell
	// sets them, and inheriting one breaks the run.
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
	for i := range 20 {
		if err := s.Hot.Set(ctx, fmt.Sprintf("hit-%d", i), []byte("v")); err != nil {
			log.Errorf(ctx, log.TagAppDef, "SET failed: %v", err)
			os.Exit(1)
		}
		if _, err := s.Hot.Get(ctx, fmt.Sprintf("hit-%d", i)); err != nil {
			log.Errorf(ctx, log.TagAppDef, "GET hit failed: %v", err)
			os.Exit(1)
		}
	}
	// Misses: read keys that were never set.
	for i := range 5 {
		if _, err := s.Hot.Get(ctx, fmt.Sprintf("absent-%d", i)); !errors.Is(err, bigcache.ErrEntryNotFound) {
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

	// --- Statistics gauges, one set per instance -------------------------------
	// The OTel gauge "bigcache.hits" renders as the Prometheus metric
	// "bigcache_hits", and the instance attribute as instance.
	hotHits := series(body, "bigcache_hits", `instance="hot"`)
	if len(hotHits) == 0 {
		fail(`metrics: no bigcache_hits series with instance="hot"`)
	}
	// 20 hits were driven through `hot`; a series stuck at zero would mean the
	// gauge never reached Stats().
	if v := value(hotHits[0]); v < 20 {
		fail("bigcache_hits{instance=\"hot\"} = %v, want >= 20", v)
	}
	hotMisses := series(body, "bigcache_misses", `instance="hot"`)
	if len(hotMisses) == 0 || value(hotMisses[0]) < 5 {
		fail("bigcache_misses{instance=\"hot\"} is missing or below the 5 misses driven")
	}

	fmt.Println("OK: statistics gauges carry hot's counters")

	// --- Per-operation signals -------------------------------------------------
	// These are the starter's headline signals: every Get/Set/Delete emits a
	// counter, tagged with the operation, the status and the instance. They
	// appear once the first operation of each kind has run.
	total := series(body, "bigcache_operation_total", `instance="hot"`)
	var sawGetOK, sawSetOK bool
	for _, line := range total {
		if strings.Contains(line, `status="error"`) {
			fail("a cache miss was counted as an error: %s", line)
		}
		if strings.Contains(line, `operation="get"`) {
			sawGetOK = true
		}
		if strings.Contains(line, `operation="set"`) {
			sawSetOK = true
		}
	}
	if !sawGetOK || !sawSetOK {
		fail("bigcache_operation_total is missing the get and/or set series")
	}
	// The key never enters a metric label - the cardinality
	// guarantee, checked rather than asserted in prose.
	for line := range strings.SplitSeq(body, "\n") {
		if strings.Contains(line, "bigcache_key=") {
			fail("a cache key leaked into a metric label: %s", line)
		}
	}
	fmt.Println("OK: per-operation counter present, misses counted as ok, no key label")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// series returns the exposition lines of the named metric whose label set
// contains want.
func series(body, name, want string) []string {
	var out []string
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, name+"{") && strings.Contains(line, want) {
			out = append(out, line)
		}
	}
	return out
}

// value reads the sample off one exposition line: it is the token after the
// final space.
func value(line string) float64 {
	f, err := strconv.ParseFloat(line[strings.LastIndex(line, " ")+1:], 64)
	if err != nil {
		fail("unparsable exposition line: %q", line)
	}
	return f
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
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

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
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
