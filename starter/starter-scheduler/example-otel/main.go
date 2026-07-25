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

// Package main is the observability example for starter-scheduler.
//
// It runs the scheduler starter with OpenTelemetry instrumentation,
// verifies that scheduler job spans reach Jaeger, then self-exits.
// Traces are exported via OTLP/gRPC to Jaeger (docker-compose).
//
// Run with -manual to keep the server running for interactive exploration.
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
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/spring/cloud/lock"
	"go-spring.org/spring/gs"

	// Blank-import the scheduler starter: it registers a gs.Server that drives
	// every ${spring.scheduler.jobs.<name>} entry against a Job bean of the same
	// name. No network port is opened — it is a global/infrastructure starter.
	scheduler "go-spring.org/starter-scheduler"

	_ "go-spring.org/starter-otel"
)

// Fire counters, incremented by the jobs so the smoke test can assert that each
// trigger kind actually fired.
var (
	tickCount   atomic.Int64 // fixed-rate job
	delayCount  atomic.Int64 // fixed-delay job
	lockedCount atomic.Int64 // fixed-rate job guarded by a lock
)

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// A Job bean per unit of work. scheduler.Provide names the bean after the job
	// and exports it as Job, so the scheduler collects it and matches it to its
	// ${spring.scheduler.jobs.<name>} config entry.
	scheduler.Provide("tick", func(ctx context.Context) error {
		tickCount.Add(1)
		return nil
	})
	scheduler.Provide("delay", func(ctx context.Context) error {
		delayCount.Add(1)
		time.Sleep(50 * time.Millisecond) // simulate work; fixed-delay never overlaps
		return nil
	})
	scheduler.Provide("beat", func(ctx context.Context) error {
		return nil // cron "* * * * *" — wired but too slow to fire in the smoke window
	})
	scheduler.Provide("locked", func(ctx context.Context) error {
		lockedCount.Add(1)
		return nil
	})

	// An in-process lock.Locker named "memory" so the "locked" job's
	// ${...locked.lock=memory} key resolves. In production this bean would be
	// contributed by starter-lock-{redis,etcd,consul} for cross-replica dedup.
	ml := lock.NewMemoryLocker()
	gs.Provide(ml).Name("memory").Export(gs.As[lock.Locker]()).Destroy(func(l lock.Locker) {
		_ = ml.Close()
	})

	if !*manual {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Open Jaeger at http://localhost:16686")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest() {
	tick := tickCount.Load()
	delay := delayCount.Load()
	locked := lockedCount.Load()

	fmt.Printf("fires: tick(fixed-rate)=%d delay(fixed-delay)=%d locked(lock)=%d\n", tick, delay, locked)

	if tick < 3 {
		fmt.Println("ERROR: fixed-rate job did not fire enough times")
		os.Exit(1)
	}
	if delay < 2 {
		fmt.Println("ERROR: fixed-delay job did not fire enough times")
		os.Exit(1)
	}
	if locked < 1 {
		fmt.Println("ERROR: locked job never fired (lock wiring broken)")
		os.Exit(1)
	}

	fmt.Println("starter-scheduler smoke test passed")

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=scheduler-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'scheduler-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'scheduler-otel-example'")

	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
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

// init pins the working directory to this source file's directory so relative
// config paths resolve regardless of how the binary is invoked.
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
