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
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import both starters: starter-go-redis publishes the *redis.Client
	// under spring.go-redis.instances.<name>, and starter-ratelimit-redis contributes a
	// limiter driver per spring.ratelimit.redis.instances.<name> that reuses that client.
	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-ratelimit-redis"
)

// The budget under test: burst 5, sustained 2/s. Slow enough that the smoke
// test's first burst drains the bucket without a refill racing it, fast enough
// that the refill check only waits ~2s.
var (
	rate  = 2.0
	burst = 5
)

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Handler A and handler B model two replicas of a service. They share NO
	// in-process state; each builds its own limiter from the registered "redis"
	// driver, and the budget is enforced by the shared Redis token bucket.
	gs.Provide(func(d resilience.LimiterDriver) *gs.HttpServeMux {
		limA, err := d.NewRateLimiter(resilience.LimitPolicy{Rate: rate, Burst: burst})
		if err != nil {
			panic(err)
		}
		limB, err := d.NewRateLimiter(resilience.LimitPolicy{Rate: rate, Burst: burst})
		if err != nil {
			panic(err)
		}

		mux := http.NewServeMux()
		serve := func(l resilience.RateLimiter) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				ok, err := l.Allow(r.Context(), "api")
				if err != nil {
					http.Error(w, "limiter backend error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				if !ok {
					http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write([]byte("ok"))
			}
		}
		mux.Handle("/a/", serve(limA))
		mux.Handle("/b/", serve(limB))
		return &gs.HttpServeMux{Handler: mux}
	}, gs.TagArg("gateway"))

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Try: curl http://127.0.0.1:9090/a/ && curl http://127.0.0.1:9090/b/")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

const base = "http://127.0.0.1:9090"

func runTest() {
	// Feature 1: shared budget across "replicas". Ten alternating requests
	// against A and B: exactly `burst` pass in total, the rest get 429 —
	// the bucket lives in Redis, not behind either handler.
	pass, limited := 0, 0
	for i := 0; i < burst*2; i++ {
		path := "/a/"
		if i%2 == 1 {
			path = "/b/"
		}
		if status(path) == http.StatusOK {
			pass++
		} else {
			limited++
		}
	}
	if pass != burst || limited != burst {
		fail("shared budget: got pass=%d limited=%d, want %d/%d", pass, limited, burst, burst)
	}
	fmt.Printf("Shared budget across replicas: %d passed, %d limited (burst=%d): OK\n", pass, limited, burst)

	// Feature 2: continuous refill. Waiting ~2s at rate=%v/s grants ~4 more
	// tokens (capped at burst), so a fresh burst is (partially) allowed again.
	time.Sleep(2200 * time.Millisecond)
	pass = 0
	for i := 0; i < burst; i++ {
		path := "/a/"
		if i%2 == 1 {
			path = "/b/"
		}
		if status(path) == http.StatusOK {
			pass++
		}
	}
	if pass < 2 {
		fail("refill: expected at least 2 allows after 2.2s at rate=%v/s, got %d", rate, pass)
	}
	fmt.Printf("Continuous refill at rate=%v/s: %d of %d re-allowed: OK\n", rate, pass, burst)

	fmt.Println("starter-ratelimit-redis smoke test passed")
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// status sends one GET to path and returns the HTTP status code.
func status(path string) int {
	resp, err := http.Get(base + path)
	if err != nil {
		fail("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

// init pins the working directory to this source file's directory so relative
// config paths resolve regardless of how the binary is invoked.
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
