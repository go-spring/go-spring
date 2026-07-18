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

	"github.com/panjf2000/ants/v2"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-ants"
)

type Service struct {
	IO  *ants.Pool `autowire:"io"`
	CPU *ants.Pool `autowire:"cpu"`
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
	if err := s.IO.Submit(func() {}); err != ants.ErrPoolOverload {
		log.Errorf(ctx, log.TagAppDef, "expected ErrPoolOverload on full nonblocking pool, got %v", err)
		os.Exit(1)
	}
	close(block)

	fmt.Println("Response from server:", "tasks:", counter,
		"io.cap:", s.IO.Cap(), "cpu.cap:", s.CPU.Cap())
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
