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
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/spring/cloud/lock"
	"go-spring.org/spring/gs"

	// Blank-import the scheduler starter: it registers a gs.Server that drives
	// every ${spring.scheduler.jobs.<name>} entry against a Job bean of the same
	// name. No network port is opened — it is a global/infrastructure starter.
	scheduler "go-spring.org/starter-scheduler"
)

// Fire counters, incremented by the jobs so the smoke test can assert that each
// trigger kind actually fired.
var (
	tickCount   atomic.Int64 // fixed-rate job
	delayCount  atomic.Int64 // fixed-delay job
	lockedCount atomic.Int64 // fixed-rate job guarded by a lock
)

func main() {
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

	go func() {
		// Give readiness a moment to trigger and the scheduler to fire a few times.
		time.Sleep(1500 * time.Millisecond)
		runTest()
	}()

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
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init pins the working directory to this source file's directory so relative
// config paths resolve regardless of how the binary is invoked.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve source file directory")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	fmt.Println(wd)
}
