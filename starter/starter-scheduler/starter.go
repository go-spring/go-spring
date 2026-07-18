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

// Package StarterScheduler runs periodic and cron-scheduled background jobs as
// part of the Go-Spring application lifecycle. Blank-importing this package
// registers a scheduler that drives every ${spring.scheduler.jobs.<name>} entry
// against a [Job] bean of the same name.
//
// This is a global / infrastructure-archetype starter (see starter/DESIGN.md
// §2.4): it opens no network port. Instead it exports a gs.Server so the
// scheduler participates in the server lifecycle — jobs start firing once the
// application is ready and, on SIGTERM, Stop drains the in-flight runs before
// the process exits (the drain the graceful-shutdown orchestration expects).
//
// The three trigger kinds — cron, fixed-rate and fixed-delay — come from
// [go-spring.org/stdlib/scheduling]. Multi-replica de-duplication is opt-in per
// job via the `lock` config key, which names a lock.Locker bean contributed by
// starter-lock-{redis,etcd,consul}; only the replica that wins the lock runs the
// fire. See [go-spring.org/stdlib/lock].
//
// Register jobs from the application:
//
//	scheduler.Provide("cleanup", svc.Cleanup)
//
// and declare their schedules in configuration:
//
//	spring.scheduler.jobs.cleanup.cron=0 */5 * * * *   # every 5 minutes
package StarterScheduler

import (
	"context"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/lock"
	"go-spring.org/stdlib/scheduling"
)

func init() {
	// Register the scheduler as a gs.Server under a distinct name so it coexists
	// with any HTTP/RPC server the app also runs. Enabled by default, but only
	// active once at least one Job bean is present — an app that imports the
	// starter without registering jobs pays nothing.
	gs.Provide(&Server{}).
		Name("schedulerServer").
		Condition(gs.OnProperty("spring.scheduler.enabled").HavingValue("true").MatchIfMissing()).
		Condition(gs.OnBean[Job]()).
		Export(gs.As[gs.Server]())
}

// Server drives the scheduled jobs and plugs the scheduler into the Go-Spring
// server lifecycle. Its exported fields are populated by the IoC container.
type Server struct {
	// Config is bound from ${spring.scheduler}.
	Config Config `value:"${spring.scheduler}"`

	// Jobs are all beans exported as Job — the units of work the application
	// registered with NewJob.
	Jobs []Job `autowire:"?"`

	// Lockers are all lock.Locker beans, keyed by bean name, so a job's `lock`
	// config key can reference one by name for multi-replica de-duplication.
	Lockers map[string]lock.Locker `autowire:"?"`

	sched scheduling.Scheduler
}

// Run wires the configured jobs, then blocks until the application shuts down.
// It validates and builds every task before signalling readiness, so a
// misconfiguration (unknown job, bad cron, missing locker) fails startup rather
// than surfacing on a later fire. Scheduling begins only after the application
// is ready, so jobs never race application startup.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	s.sched = scheduling.NewScheduler(scheduling.WithObserver(s.observe))

	if err := s.build(); err != nil {
		return err
	}

	<-sig.TriggerAndWait()

	if err := s.sched.Start(ctx); err != nil {
		return err
	}
	log.Infof(ctx, log.TagAppDef, "scheduler started with %d job(s)", len(s.Config.Jobs))

	<-ctx.Done()
	return nil
}

// Stop halts scheduling and drains in-flight runs, bounded by the configured
// drain timeout. It is called during graceful shutdown.
func (s *Server) Stop() error {
	if s.sched == nil {
		return nil
	}
	ctx := context.Background()
	if s.Config.DrainTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Config.DrainTimeout)
		defer cancel()
	}
	if err := s.sched.Stop(ctx); err != nil {
		log.Warnf(context.Background(), log.TagAppDef, "scheduler drain timed out: %v", err)
		return err
	}
	return nil
}

// build translates the bound configuration and registered Job beans into
// scheduled tasks. It fails fast on any misconfiguration.
func (s *Server) build() error {
	jobs := make(map[string]Job, len(s.Jobs))
	for _, j := range s.Jobs {
		if _, dup := jobs[j.JobName()]; dup {
			return errutil.Explain(nil, "scheduler: duplicate job bean named %q", j.JobName())
		}
		jobs[j.JobName()] = j
	}

	// Warn about jobs registered in code but never scheduled by config — they are
	// dead weight and almost always an oversight.
	for name := range jobs {
		if _, ok := s.Config.Jobs[name]; !ok {
			log.Warnf(context.Background(), log.TagAppDef,
				"scheduler: job bean %q has no ${spring.scheduler.jobs.%s} entry; it will not run", name, name)
		}
	}

	for name, jc := range s.Config.Jobs {
		job, ok := jobs[name]
		if !ok {
			return errutil.Explain(nil,
				"scheduler: job %q is configured but no Job bean of that name is registered", name)
		}

		trigger, err := jc.trigger(name)
		if err != nil {
			return err
		}
		policy, err := jc.policy(name)
		if err != nil {
			return err
		}

		opts := []scheduling.Option{scheduling.WithConcurrencyPolicy(policy)}
		if jc.Timeout > 0 {
			opts = append(opts, scheduling.WithTimeout(jc.Timeout))
		}
		if jc.Lock != "" {
			locker, ok := s.Lockers[jc.Lock]
			if !ok {
				return errutil.Explain(nil,
					"scheduler: job %q references lock %q but no lock.Locker bean of that name is registered", name, jc.Lock)
			}
			key := jc.LockKey
			if key == "" {
				key = name
			}
			adapter := lockerAdapter{l: locker, opts: lockTTLOption(jc.LockTTL)}
			opts = append(opts, scheduling.WithLock(adapter, key))
		}

		if _, err := s.sched.Schedule(name, trigger, job.Run, opts...); err != nil {
			return errutil.Explain(err, "scheduler: failed to schedule job %q", name)
		}
	}
	return nil
}

// observe logs each fire outcome. It is the seam where metrics/tracing could be
// added later (e.g. an otel-backed observer); for now it bridges into go-spring
// log so operators see runs, skips and failures.
func (s *Server) observe(ev scheduling.Event) {
	ctx := context.Background()
	switch {
	case ev.Skipped:
		log.Debugf(ctx, log.TagAppDef, "scheduler: job %q skipped (%s)", ev.Name, ev.Reason)
	case ev.Err != nil:
		log.Errorf(ctx, log.TagAppDef, "scheduler: job %q failed after %s: %v", ev.Name, ev.Duration, ev.Err)
	default:
		log.Debugf(ctx, log.TagAppDef, "scheduler: job %q ran in %s", ev.Name, ev.Duration)
	}
}
