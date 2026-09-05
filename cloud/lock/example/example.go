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

// Command example demonstrates cloud/lock on the bundled MemoryLocker:
// Acquire/TryAcquire contention, the fencing token, Lost() on lease expiry,
// and a leader-election term handover. No external services are required.
//
// It self-asserts every step and exits non-zero on mismatch, so it doubles
// as the package's smoke test.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-spring.org/cloud/lock"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("lock example ok")
}

func run() error {
	locker := lock.NewMemoryLocker()
	defer locker.Close()
	ctx := context.Background()

	// 1. Acquire takes the lock; a second holder on the same key must fail.
	first, err := locker.Acquire(ctx, "jobs/rollup", lock.WithTTL(30*time.Second))
	if err != nil {
		return err
	}
	fmt.Printf("acquired key=%s token=%s\n", first.Key(), first.Token())

	if _, ok, err := locker.TryAcquire(ctx, "jobs/rollup"); ok || err != nil {
		return fmt.Errorf("contended TryAcquire: ok=%v err=%v", ok, err)
	}
	fmt.Println("contended TryAcquire correctly skipped")

	// 2. Unlock is idempotent; a fresh acquisition can carry an explicit
	// fencing token.
	if err := first.Unlock(ctx); err != nil {
		return fmt.Errorf("unlock first: %w", err)
	}
	if err := first.Unlock(ctx); err != nil {
		return fmt.Errorf("second unlock must be idempotent, got %v", err)
	}
	second, err := locker.Acquire(ctx, "jobs/rollup", lock.WithToken("worker-42"))
	if err != nil {
		return err
	}
	if second.Token() != "worker-42" {
		return fmt.Errorf("want explicit token worker-42, got %s", second.Token())
	}
	fmt.Println("explicit token honored, unlock idempotent")

	// 3. Lost() fires when the lease is gone: acquire with a short TTL and
	// no renew, let it expire, then a takeover by another holder signals the
	// loss to the stale one.
	expiring, err := locker.Acquire(ctx, "jobs/expire",
		lock.WithTTL(80*time.Millisecond), lock.WithRenewInterval(-1))
	if err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	taker, ok, err := locker.TryAcquire(ctx, "jobs/expire")
	if err != nil || !ok {
		return fmt.Errorf("expired lease not reclaimed: ok=%v err=%v", ok, err)
	}
	select {
	case <-expiring.Lost():
		fmt.Println("Lost() fired on lease takeover")
	default:
		return fmt.Errorf("Lost() did not fire after takeover")
	}
	_ = taker.Unlock(ctx)

	// 4. Election: run until this instance is leader, then stop it and watch
	// a second candidate take over the freed key — one handover cycle.
	electCtx, stopElect := context.WithCancel(ctx)
	elect := newCandidate(locker)
	go func() { _ = elect.Run(electCtx) }()

	if err := waitLeader(elect, true); err != nil {
		return err
	}
	fmt.Println("election: candidate became leader")

	// Also drop the remaining critical-section lock from step 2-3 area.
	if err := second.Unlock(ctx); err != nil {
		return fmt.Errorf("unlock second: %w", err)
	}

	// Stop the leader; its Run unlocks and returns, freeing the key.
	stopElect()
	waitLeader(elect, false)
	fmt.Println("election: leader resigned")

	next := newCandidate(locker)
	go func() { _ = next.Run(ctx) }()
	if err := waitLeader(next, true); err != nil {
		return err
	}
	fmt.Println("election: leadership handed over")
	return nil
}

// newCandidate builds one election candidate on the leaders/reporter key.
func newCandidate(locker lock.Locker) *lock.Election {
	return lock.NewElection(lock.ElectionConfig{
		Locker:        locker,
		Key:           "leaders/reporter",
		TTL:           200 * time.Millisecond,
		RetryInterval: 50 * time.Millisecond,
		OnElected: func(ctx context.Context) {
			<-ctx.Done() // honour the term context
		},
		OnResigned: func() {},
	})
}

// waitLeader blocks until e.IsLeader() == want or the deadline passes.
func waitLeader(e *lock.Election, want bool) error {
	deadline := time.Now().Add(3 * time.Second)
	for e.IsLeader() != want {
		if time.Now().After(deadline) {
			return fmt.Errorf("want isLeader=%v, got %v", want, e.IsLeader())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
