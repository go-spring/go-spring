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

package StarterScheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/lock"
	"go-spring.org/stdlib/scheduling"
)

// Job is the seam between the application and the scheduler: the application
// registers one Job bean per unit of work, and the starter matches each to a
// ${spring.scheduler.jobs.<name>} entry by Name to learn its trigger and
// options. This mirrors the "app owns the work, starter owns the runner" split
// used by the server starters (where the app supplies a register function).
//
// Register a Job with [Provide]:
//
//	scheduler.Provide("cleanup", func(ctx context.Context) error {
//	    return svc.Cleanup(ctx)
//	})
type Job interface {
	// JobName is the name that ties this job to its config entry.
	JobName() string
	// Run performs the work. It should honour ctx for cancellation.
	Run(ctx context.Context) error
}

// NewJob adapts a name and a function into a [Job] bean. It panics on an empty
// name or nil function, since either makes the job unusable.
//
// NewJob returns the [Job] value only; callers who use it directly must register
// it with the container themselves, naming the bean and exporting it as Job so
// the scheduler can collect it:
//
//	gs.Provide(scheduler.NewJob("cleanup", svc.Cleanup)).
//	    Name("cleanup").Export(gs.As[scheduler.Job]())
//
// Prefer [Provide], which does exactly that in one call.
func NewJob(name string, fn func(ctx context.Context) error) Job {
	if name == "" {
		panic("scheduler: job name must not be empty")
	}
	if fn == nil {
		panic("scheduler: job function must not be nil")
	}
	return &namedJob{name: name, fn: fn}
}

// Provide registers a scheduled job in one call: it wraps fn in a [Job] bean,
// names the bean after the job, and exports it as Job so the scheduler collects
// it and matches it to its ${spring.scheduler.jobs.<name>} config entry. This is
// the idiomatic way an application declares work to run:
//
//	scheduler.Provide("cleanup", svc.Cleanup)
//
// It panics on an empty name or nil function (see [NewJob]). For advanced cases
// that need conditions or lifecycle hooks, register with [NewJob] and gs.Provide
// directly.
func Provide(name string, fn func(ctx context.Context) error) {
	gs.Provide(NewJob(name, fn)).Name(name).Export(gs.As[Job]())
}

type namedJob struct {
	name string
	fn   func(ctx context.Context) error
}

func (j *namedJob) JobName() string               { return j.name }
func (j *namedJob) Run(ctx context.Context) error { return j.fn(ctx) }

// trigger builds the scheduling.Trigger for a job from its config, enforcing
// that exactly one of cron/fixed-rate/fixed-delay is set.
func (c JobConfig) trigger(name string) (scheduling.Trigger, error) {
	set := 0
	if c.Cron != "" {
		set++
	}
	if c.FixedRate > 0 {
		set++
	}
	if c.FixedDelay > 0 {
		set++
	}
	if set == 0 {
		return nil, fmt.Errorf("scheduler: job %q must set exactly one of cron/fixed-rate/fixed-delay", name)
	}
	if set > 1 {
		return nil, fmt.Errorf("scheduler: job %q sets more than one of cron/fixed-rate/fixed-delay", name)
	}

	switch {
	case c.Cron != "":
		tr, err := scheduling.ParseCron(c.Cron)
		if err != nil {
			return nil, fmt.Errorf("scheduler: job %q: %w", name, err)
		}
		return tr, nil
	case c.FixedRate > 0:
		return scheduling.FixedRate(c.FixedRate), nil
	default:
		return scheduling.FixedDelay(c.FixedDelay), nil
	}
}

// policy parses the concurrency policy string.
func (c JobConfig) policy(name string) (scheduling.ConcurrencyPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(c.Concurrency)) {
	case "", "skip":
		return scheduling.Skip, nil
	case "queue":
		return scheduling.Queue, nil
	case "replace":
		return scheduling.Replace, nil
	default:
		return 0, fmt.Errorf("scheduler: job %q has invalid concurrency %q (want skip|queue|replace)", name, c.Concurrency)
	}
}

// lockerAdapter adapts a lock.Locker to the minimal scheduling.Locker the
// scheduler needs, baking in the lease options so the scheduler abstraction
// stays free of the lock package. The lock is auto-renewed by the lock package
// while held, so a long-running job keeps the lease.
type lockerAdapter struct {
	l    lock.Locker
	opts []lock.Option
}

func (a lockerAdapter) TryAcquire(ctx context.Context, key string) (scheduling.Lock, bool, error) {
	l, ok, err := a.l.TryAcquire(ctx, key, a.opts...)
	if !ok || l == nil {
		return nil, ok, err
	}
	return l, true, nil
}

func lockTTLOption(ttl time.Duration) []lock.Option {
	if ttl <= 0 {
		return nil
	}
	return []lock.Option{lock.WithTTL(ttl)}
}
