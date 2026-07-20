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
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/lock"

	// Blank-import both starters: starter-go-redis publishes the *redis.Client
	// under spring.go-redis.<name>, and starter-lock-redis contributes a
	// lock.Locker per spring.lock.<name> that reuses that client by name.
	_ "go-spring.org/starter-go-redis"
	_ "go-spring.org/starter-lock-redis"
)

// Service exercises the injected Locker. The `jobs` tag matches the instance
// name in app.properties (spring.lock.jobs).
type Service struct {
	Lock lock.Locker `autowire:"jobs"`
}

func main() {
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(svrBean.Interface().(*Service))
	}()

	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: TryAcquire on a free key succeeds.
	held, ok, err := s.Lock.TryAcquire(ctx, "demo")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "TryAcquire failed: %v", err)
		os.Exit(1)
	}
	if !ok || held == nil {
		log.Errorf(ctx, log.TagAppDef, "TryAcquire should have succeeded on a free key")
		os.Exit(1)
	}
	fmt.Println("Acquired lock:", held.Key(), "token=", held.Token())

	// Feature 2: A second TryAcquire on the same key must observe contention.
	_, ok2, err := s.Lock.TryAcquire(ctx, "demo")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "second TryAcquire returned error: %v", err)
		os.Exit(1)
	}
	if ok2 {
		log.Errorf(ctx, log.TagAppDef, "second TryAcquire should have failed while lock is held")
		os.Exit(1)
	}
	fmt.Println("Contention observed as expected")

	// Feature 3: Unlock releases the lock, and a subsequent TryAcquire succeeds.
	if err := held.Unlock(ctx); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Unlock failed: %v", err)
		os.Exit(1)
	}
	// Unlock is idempotent: a second Unlock must return nil.
	if err := held.Unlock(ctx); err != nil {
		log.Errorf(ctx, log.TagAppDef, "second Unlock should be a no-op, got: %v", err)
		os.Exit(1)
	}

	held2, ok3, err := s.Lock.TryAcquire(ctx, "demo")
	if err != nil || !ok3 {
		log.Errorf(ctx, log.TagAppDef, "TryAcquire after Unlock failed: ok=%v err=%v", ok3, err)
		os.Exit(1)
	}
	_ = held2.Unlock(ctx)
	fmt.Println("Re-acquired after Unlock: OK")

	// Feature 4: Acquire (blocking) with an already-free key returns immediately.
	acquireCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	held3, err := s.Lock.Acquire(acquireCtx, "acq-demo", lock.WithTTL(5*time.Second))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Acquire failed: %v", err)
		os.Exit(1)
	}
	_ = held3.Unlock(ctx)
	fmt.Println("Acquire (blocking) on free key: OK")

	fmt.Println("starter-lock-redis smoke test passed")
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
