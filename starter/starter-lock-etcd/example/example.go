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
	"go-spring.org/spring/cloud/lock"

	// Blank-import the etcd backend. Switching to Redis/Consul is a one-line
	// change here plus a properties swap under spring.lock.
	_ "go-spring.org/starter-lock-etcd"
)

// Service consumes the framework-agnostic lock.Locker interface. The concrete
// implementation is chosen by which starter is blank-imported.
type Service struct {
	Locker lock.Locker `autowire:"main"`
}

func main() {
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
	ctx := context.Background()
	key := "example/resource"

	// Feature 1: TryAcquire succeeds when no one holds the key.
	first, ok, err := s.Locker.TryAcquire(ctx, key, lock.WithTTL(10*time.Second))
	if err != nil {
		fail("first TryAcquire error: %v", err)
	}
	if !ok || first == nil {
		fail("first TryAcquire should have succeeded")
	}
	if first.Key() != key {
		fail("first.Key() mismatch: %q", first.Key())
	}
	if first.Token() == "" {
		fail("first.Token() must be non-empty")
	}

	// Feature 2: mutual exclusion — a second TryAcquire on the same key
	// reports contention (ok=false, err=nil, lock=nil).
	if second, ok2, err2 := s.Locker.TryAcquire(ctx, key); err2 != nil {
		fail("second TryAcquire error: %v", err2)
	} else if ok2 || second != nil {
		fail("second TryAcquire should have been contended")
	}

	// Feature 3: Lost stays open while the lock is held.
	select {
	case <-first.Lost():
		fail("Lost() fired before Unlock")
	default:
	}

	// Feature 4: idempotent Unlock — closes Lost() and returns nil on repeat.
	if err := first.Unlock(ctx); err != nil {
		fail("Unlock error: %v", err)
	}
	select {
	case <-first.Lost():
	case <-time.After(2 * time.Second):
		fail("Lost() did not close after Unlock")
	}
	if err := first.Unlock(ctx); err != nil {
		fail("second Unlock should be idempotent, got: %v", err)
	}

	// Feature 5: the key is now free — a fresh acquisition succeeds.
	next, ok3, err := s.Locker.TryAcquire(ctx, key)
	if err != nil {
		fail("re-acquire error: %v", err)
	}
	if !ok3 || next == nil {
		fail("re-acquire should have succeeded after Unlock")
	}
	_ = next.Unlock(ctx)

	fmt.Println("Response from server: all etcd lock features OK")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory where
// this source file resides, so relative paths in app.properties resolve
// against the example folder.
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
