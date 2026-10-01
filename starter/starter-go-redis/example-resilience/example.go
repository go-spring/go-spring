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

// Command example demonstrates starter-go-redis's resilience protection: with
// resilience.enabled, every Redis command runs through the selected driver's
// Executor (rate limit / circuit breaker / bulkhead), so a burst over the limit
// is rejected with ErrRateLimited before reaching Redis. Configuration is
// backend-neutral — the same ${resilience.*} keys drive every client starter.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"

	StarterGoRedis "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-governance-file" // registers the centralized governance center
)

// Service autowires the named "cache" redis instance. Its ops are protected by
// the resilience executor built from the ${resilience.*} config.
type Service struct {
	Redis *StarterGoRedis.Client `autowire:"cache"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	svc := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svc.Interface().(*Service))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest fires a burst of Set calls and asserts that the resilience rate limit
// admits a non-empty head and rejects a non-empty tail with ErrRateLimited.
func runTest(s *Service) {
	ctx := context.Background()
	var ok, rejected int
	for i := range 15 {
		_, err := s.Redis.Set(ctx, "resilience:key", fmt.Sprintf("%d", i), 0).Result()
		switch {
		case err == nil:
			ok++
		case errors.Is(err, resilience.ErrRateLimited):
			rejected++
		default:
			fail("Set: %v", err)
		}
	}
	if ok == 0 || rejected == 0 {
		fail("resilience ineffective: ok=%d rejected=%d", ok, rejected)
	}
	fmt.Printf("go-redis resilience: %d Set admitted, %d rejected with ErrRateLimited\n", ok, rejected)

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
