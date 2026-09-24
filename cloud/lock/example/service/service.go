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

// Package service consumes the configured locker exactly as production code
// would: it injects lock.Locker by the instance name from configuration and
// runs one pass of the lock demo — a guarded critical section, a contended
// TryAcquire that correctly skips, and one leader-election handover.
package service

import (
	"context"
	"os"
	"syscall"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// Demo injects the "memory.demo" lock instance declared in conf/app.properties (the
// bean name is backend-qualified, the gorm "<dialect>.<instance>" pattern). The
// bean is the observe-wrapped MemoryLocker; from here it is just a Locker.
type Demo struct {
	Locker lock.Locker `autowire:"memory.demo"`
}

// demoRunner adapts Demo's single pass into a gs.Runner: it executes the demo
// steps once, in order, then returns; with no server to keep alive the
// application exits when the runner finishes.
type demoRunner struct{ d *Demo }

func (r demoRunner) Run(ctx context.Context) error {
	err := r.d.pass(ctx)
	// A runner finishing does not stop an application that may still be
	// shutting down servers; this demo has nothing else to do, so terminate
	// the process gracefully once the pass is done (the family pattern for
	// one-shot starter examples).
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	return err
}

func init() {
	gs.Provide(func() *Demo {
		return &Demo{} // Locker is field-autowired from the "demo" bean
	})
	gs.Provide(func(d *Demo) demoRunner {
		return demoRunner{d: d}
	}).Export(gs.As[gs.Runner]())
}

// pass executes the demo steps once, in order.
func (d *Demo) pass(ctx context.Context) error {
	if err := d.criticalSection(ctx); err != nil {
		return err
	}
	if err := d.contendedSkip(ctx); err != nil {
		return err
	}
	return d.electionHandover(ctx)
}

// criticalSection takes the lock, does the work under it, releases it.
func (d *Demo) criticalSection(ctx context.Context) error {
	held, err := d.Locker.Acquire(ctx, "jobs/rollup", lock.WithTTL(30*time.Second))
	if err != nil {
		return errutil.Explain(err, "acquire jobs/rollup")
	}
	log.Info(ctx, log.TagAppDef, log.Msg("demo: critical section entered"),
		log.String("lock.key", held.Key()), log.String("token", held.Token()))
	select {
	case <-held.Lost():
		return errutil.Explain(nil, "demo: lease lost inside the critical section")
	default:
	}
	if err := held.Unlock(ctx); err != nil {
		return errutil.Explain(err, "unlock jobs/rollup")
	}
	log.Info(ctx, log.TagAppDef, log.Msg("demo: critical section done"))
	return nil
}

// contendedSkip shows TryAcquire's skip semantics: holding the key with one
// handle, a second attempt reports ok=false with a nil error.
func (d *Demo) contendedSkip(ctx context.Context) error {
	held, err := d.Locker.Acquire(ctx, "jobs/rollup")
	if err != nil {
		return errutil.Explain(err, "acquire jobs/rollup")
	}
	defer held.Unlock(ctx)

	_, ok, err := d.Locker.TryAcquire(ctx, "jobs/rollup")
	if err != nil {
		return errutil.Explain(err, "unexpected backend error")
	}
	if ok {
		return errutil.Explain(nil, "demo: contended TryAcquire unexpectedly succeeded")
	}
	log.Info(ctx, log.TagAppDef, log.Msg("demo: contended TryAcquire correctly skipped"))
	return nil
}

// electionHandover runs one full cycle: a candidate becomes leader, stops,
// and a second candidate takes the freed key over.
func (d *Demo) electionHandover(ctx context.Context) error {
	first, err := lock.NewElection(lock.ElectionConfig{
		Locker:        d.Locker,
		Key:           "leaders/reporter",
		RetryInterval: 50 * time.Millisecond,
		OnStartedLeading: func(ctx context.Context) {
			<-ctx.Done() // honour the term context
		},
		OnStoppedLeading: func() {},
	})
	if err != nil {
		return err
	}
	electCtx, stop := context.WithCancel(ctx)
	go func() { _ = first.Run(electCtx) }()
	if err := waitLeader(first, true); err != nil {
		return err
	}
	log.Info(ctx, log.TagAppDef, log.Msg("demo: first candidate became leader"))

	stop()
	if err := waitLeader(first, false); err != nil {
		return err
	}

	second, err := lock.NewElection(lock.ElectionConfig{
		Locker:           d.Locker,
		Key:              "leaders/reporter",
		RetryInterval:    50 * time.Millisecond,
		OnStartedLeading: func(context.Context) {},
		OnStoppedLeading: func() {},
	})
	if err != nil {
		return err
	}
	go func() { _ = second.Run(ctx) }()
	if err := waitLeader(second, true); err != nil {
		return err
	}
	log.Info(ctx, log.TagAppDef, log.Msg("demo: leadership handed over to the second candidate"))
	return nil
}

// waitLeader blocks until e.IsLeader() == want or the deadline passes.
func waitLeader(e *lock.Election, want bool) error {
	deadline := time.Now().Add(3 * time.Second)
	for e.IsLeader() != want {
		if time.Now().After(deadline) {
			return errutil.Explain(nil, "want isLeader=%v, got %v", want, e.IsLeader())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
