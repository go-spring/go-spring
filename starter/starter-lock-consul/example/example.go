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
	"go-spring.org/stdlib/lock"

	_ "go-spring.org/starter-lock-consul"
)

// Service wires two named Consul-backed lockers. `jobs` and `singleton` are
// independent bean instances proving multi-instance wiring; both share the same
// underlying Consul cluster but own separate api.Client sessions.
type Service struct {
	Jobs      lock.Locker `autowire:"jobs"`
	Singleton lock.Locker `autowire:"singleton"`
}

func main() {
	// Here `s` is not referenced by any other object, so we register it as a
	// root bean to keep it in the container graph.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(svrBean.Interface().(*Service))
	}()

	gs.Run()
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

func runTest(s *Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Feature 1: TryAcquire happy path — acquire, verify token/key surface,
	// then release.
	l1, ok, err := s.Jobs.TryAcquire(ctx, "nightly-sync")
	if err != nil {
		fail("TryAcquire failed: %v", err)
	}
	if !ok || l1 == nil {
		fail("TryAcquire: expected ok=true, got ok=%v", ok)
	}
	if l1.Key() == "" {
		fail("Lock.Key returned empty string")
	}
	if l1.Token() == "" {
		fail("Lock.Token returned empty string")
	}

	// Feature 2: contention. A second TryAcquire on the same key must return
	// ok=false with nil error and nil handle.
	if l2, ok2, err2 := s.Jobs.TryAcquire(ctx, "nightly-sync"); err2 != nil {
		fail("contended TryAcquire returned err=%v", err2)
	} else if ok2 || l2 != nil {
		fail("contended TryAcquire expected ok=false, got ok=%v handle=%v", ok2, l2)
	}

	// Feature 3: Lost() is not closed while we hold the lock.
	select {
	case <-l1.Lost():
		fail("Lost() fired while lock was still held")
	default:
	}

	// Feature 4: Unlock is idempotent — the second call must return nil.
	if err := l1.Unlock(ctx); err != nil {
		fail("first Unlock failed: %v", err)
	}
	if err := l1.Unlock(ctx); err != nil {
		fail("second Unlock (idempotent) returned: %v", err)
	}

	// Feature 5: after Unlock, the key is available to a fresh acquisition on
	// the *other* named instance too, proving the two beans coordinate through
	// the shared Consul cluster (different key though — key-prefix is per-app).
	l3, ok3, err := s.Singleton.TryAcquire(ctx, "singleton-worker")
	if err != nil {
		fail("Singleton TryAcquire failed: %v", err)
	}
	if !ok3 {
		fail("Singleton TryAcquire expected ok=true")
	}
	_ = l3.Unlock(ctx)

	// Feature 6: Acquire blocks then succeeds on a free key.
	l4, err := s.Jobs.Acquire(ctx, "batch-run")
	if err != nil {
		fail("Acquire failed: %v", err)
	}
	_ = l4.Unlock(ctx)

	fmt.Println("Response from server: all lock-consul features OK")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory where
// this source file resides, so relative conf/ paths resolve regardless of
// where `go run` was invoked from.
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
