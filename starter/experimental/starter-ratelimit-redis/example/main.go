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
	"errors"
	"flag"
	"fmt"
	"go-spring.org/cloud/chain"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import both starters: starter-go-redis publishes the *goredis.Client
	// bean named by spring.ratelimit.redis.client, and starter-ratelimit-redis
	// contributes the Redis-backed resilience.Counters store — the one the
	// resilience driver injects, in place of the private per-executor stores a
	// container with no store bean gets.
	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-ratelimit-redis"
)

// The budget under test, configured as a governance rule in conf/governance.yaml:
// burst 5, sustained 2/s. These constants mirror that rule so the assertions
// below can name the numbers they expect; the executor reads the rule, not
// them.
const (
	rate  = 2.0
	burst = 5
)

// service is the resilience service label the two handlers share: the same
// label means the same executor and the same scope, so both "replicas" spend
// one budget per scope.
const service = "ratelimit-redis:api"

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func init() {
	gs.Provide(func(center *governance.Center) *gs.HttpServeMux {
		// Handlers A and B model two replicas of a service: they share NO
		// in-process state. Each gets its own executor handle for the same
		// service label, and both draw on the one Redis-backed counter store
		// this process contributed, so the budget is global.
		mux := http.NewServeMux()
		mux.Handle("/a/", serve(executor(center.Resilience())))
		mux.Handle("/b/", serve(executor(center.Resilience())))
		return &gs.HttpServeMux{Handler: mux}
	})
}

// executor returns the handle for the shared service label. ClientExecutorFor
// resolves its backing executor lazily, on each Execute, so the policy is read
// after the governance center has gone live.
func executor(mgr *resilience.Manager) chain.Executor {
	return mgr.ClientExecutorFor("ratelimit-redis", service)
}

// serve runs one protected call per request. The executor's rate-limit stage
// charges the shared store; an over-budget call comes back as ErrRateLimited.
func serve(exec chain.Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := exec.Execute(r.Context(), func(context.Context) error { return nil })
		switch {
		case err == nil:
			_, _ = w.Write([]byte("ok"))
		case errors.Is(err, chain.ErrRateLimited):
			http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
		default:
			http.Error(w, "executor error: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func main() {
	flag.Parse()

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
	// against A and B: exactly `burst` pass in total, the rest get 429 — the
	// counter state lives in Redis, not behind either handler.
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
