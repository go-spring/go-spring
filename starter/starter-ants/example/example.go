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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterAnts "go-spring.org/starter-ants"
)

// panics counts task panics caught by the handler registered via
// SetPanicHandler, proving the injection point is wired through the starter.
var panics int64

func init() {
	// Register a global panic handler before the container starts. Without it,
	// a panicking task would crash the worker goroutine.
	StarterAnts.SetPanicHandler(func(p any) {
		atomic.AddInt64(&panics, 1)
		log.Warnf(context.Background(), log.TagAppDef, "recovered task panic: %v", p)
	})
}

type Service struct {
	IO      StarterAnts.Pool             `autowire:"io"`
	CPU     StarterAnts.Pool             `autowire:"cpu"`
	Metrics *StarterAnts.MetricsObserver `autowire:""`
}

func main() {
	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: Submit runs tasks concurrently on the pool. Submit 100 tasks
	// to the (blocking) cpu pool that each increment a counter and confirm all
	// of them ran.
	var counter int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		err := s.CPU.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
		})
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "CPU submit failed: %v", err)
			os.Exit(1)
		}
	}
	wg.Wait()
	if atomic.LoadInt64(&counter) != 100 {
		log.Errorf(ctx, log.TagAppDef, "expected 100 tasks to run, got %d", counter)
		os.Exit(1)
	}

	// Feature 2: instance isolation — the two named pools are fully independent,
	// which is proven by their distinct capacities coming from configuration.
	if s.IO.Cap() != 2 || s.CPU.Cap() != 8 {
		log.Errorf(ctx, log.TagAppDef, "unexpected caps: io=%d cpu=%d", s.IO.Cap(), s.CPU.Cap())
		os.Exit(1)
	}

	// Feature 3: nonblocking overload — the `io` pool has capacity 2 and is
	// configured nonblocking, so occupying both workers with a blocking task
	// makes the next Submit return ErrPoolOverload instead of blocking.
	block := make(chan struct{})
	for i := 0; i < 2; i++ {
		if err := s.IO.Submit(func() { <-block }); err != nil {
			log.Errorf(ctx, log.TagAppDef, "IO submit to fill pool failed: %v", err)
			os.Exit(1)
		}
	}
	// Give the workers a moment to pick up both blocking tasks.
	time.Sleep(time.Millisecond * 50)
	if err := s.IO.Submit(func() {}); err == nil {
		log.Errorf(ctx, log.TagAppDef, "expected error on full nonblocking pool, got nil")
		os.Exit(1)
	}
	fmt.Println("Nonblocking pool correctly rejected submit")
	close(block)

	// Feature 4: pool metrics via Pool interface. Running/Free/Cap/Waiting are
	// read straight off the pool.
	fmt.Println("CPU pool:", "running:", s.CPU.Running(),
		"free:", s.CPU.Free(), "cap:", s.CPU.Cap(), "waiting:", s.CPU.Waiting())

	// Feature 5: panic handler. A task that panics is caught by the handler
	// registered via SetPanicHandler instead of crashing the worker.
	var pwg sync.WaitGroup
	pwg.Add(1)
	if err := s.CPU.Submit(func() {
		defer pwg.Done()
		panic("boom")
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "submit panicking task failed: %v", err)
		os.Exit(1)
	}
	pwg.Wait()
	// Give the deferred panic handler a moment to run after Done returns.
	time.Sleep(time.Millisecond * 50)
	if atomic.LoadInt64(&panics) == 0 {
		log.Errorf(ctx, log.TagAppDef, "expected panic handler to fire, got 0")
		os.Exit(1)
	}
	fmt.Println("Panic handler fired:", atomic.LoadInt64(&panics), "times")

	// Feature 6: MetricsObserver — aggregated metrics across all pools.
	stats := s.Metrics.Snapshot()
	s.Metrics.Enrich(&stats, map[string]StarterAnts.Pool{
		"io":  s.IO,
		"cpu": s.CPU,
	})
	fmt.Println("=== Pool Metrics ===")
	for _, ps := range stats.Pools {
		fmt.Printf("  %s: cap=%d running=%d waiting=%d free=%d\n",
			ps.Name, ps.Cap, ps.Running, ps.Waiting, ps.Free)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

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
