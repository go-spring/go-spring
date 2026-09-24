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
// registers a scheduler that drives every [Job] bean the application registers.
//
// This is a global / infrastructure-archetype starter (see starter/DESIGN.md
// §2.4): it opens no network port. Instead it exports a gs.Server so the
// scheduler participates in the server lifecycle — jobs start firing once the
// application is ready and, on SIGTERM, Stop drains the in-flight runs before
// the process exits (the drain the graceful-shutdown orchestration expects).
//
// A job is a bean of the concrete type scheduler.Job (no interface: nothing
// can implement it by accident, and navigation lands on the struct), built by
// scheduling.NewJob in the work's constructor — typically from a bound method
// of your own struct, so the job's dependencies are resolved by the container.
// The schedule is declared there too, next to the work it describes:
//
//	func NewCleanupJob(db *gormcore.DB) *scheduler.Job {
//	    svc := &CleanupService{db: db}
//	    return scheduler.NewJob("cleanup", svc.Cleanup, scheduling.FixedRate(5*time.Minute))
//	}
//
//	gs.Provide(NewCleanupJob)

// Triggers, execution options and the lock are all cloud types (see
// [go-spring.org/cloud/scheduling] and [go-spring.org/cloud/lock]):
// multi-replica de-duplication attaches a lock to the Job with
// WithLock taking the injected lock.Locker bean directly; only the
// replica that wins the lock runs the fire.
//
// Only one process-level knob comes from configuration:
// `spring.scheduler.enabled`. The shutdown drain is bounded by the
// orchestrator's kill deadline, not a config value. A job's cadence is part of what the job
// is, so it is not something an operator retunes from a config file — for
// ops-managed schedules use a job platform (e.g. starter-xxl-job) instead.
package StarterScheduler

import (
	"context"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register the scheduler as a gs.Server under a distinct name so it coexists
	// with any HTTP/RPC server the app also runs. Enabled by default, but only
	// active once at least one Job bean is present — an app that imports the
	// starter without registering jobs pays nothing.
	gs.Provide(&Server{}).
		Name("schedulerServer").
		Condition(gs.OnProperty("spring.scheduler.enabled").HavingValue("true").MatchIfMissing()).
		Condition(gs.OnBean[*scheduling.Job]()).
		Export(gs.As[gs.Server]())
}

// Server drives the scheduled jobs and plugs the scheduler into the Go-Spring
// server lifecycle. Its exported fields are populated by the IoC container.
type Server struct {
	// Jobs are all beans of the cloud package's scheduling.Job type — the
	// units of work the application registered, each carrying its own
	// schedule, options and lock.
	Jobs []*scheduling.Job `autowire:"?"`

	sched *scheduling.Scheduler
}

// Run wires the configured jobs, then blocks until the application shuts down.
// It validates and builds every task before signalling readiness, so a
// misconfiguration (unknown job, bad cron, missing locker) fails startup rather
// than surfacing on a later fire. Scheduling begins only after the application
// is ready, so jobs never race application startup.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	log.Debugf(context.Background(), log.TagAppDef, "scheduler starting with %d job(s)", len(s.Jobs))

	s.sched = scheduling.NewScheduler()
	if err := s.build(); err != nil {
		return err
	}

	<-sig.TriggerAndWait()

	if err := s.sched.Start(ctx); err != nil {
		return err
	}
	log.Infof(ctx, log.TagAppDef, "scheduler started with %d job(s)", len(s.Jobs))

	<-ctx.Done()
	return nil
}

// Stop halts scheduling and drains in-flight runs. It is called during
// graceful shutdown with the framework's shutdown context; the drain is
// bounded by whatever that context carries — in production the orchestrator
// (K8s terminationGracePeriod, systemd TimeoutStopSec) is the real deadline
// and force-kills the process past it, so the starter adds no second knob.
func (s *Server) Stop(ctx context.Context) error {
	if s.sched == nil {
		return nil
	}
	if err := s.sched.Stop(ctx); err != nil {
		log.Warnf(ctx, log.TagAppDef, "scheduler drain timed out: %v", err)
		return err
	}
	return nil
}

// build turns the registered Job beans into scheduled tasks. Each Job was
// validated by scheduling.NewJob at construction; what remains here is only
// what a job cannot know about itself: that its name is unique on this
// scheduler. A job's lock references a locker it already holds (bridged by
// WithLock at construction), so there is nothing left to resolve.
func (s *Server) build() error {
	for _, j := range s.Jobs {
		if _, err := s.sched.Schedule(j); err != nil {
			return errutil.Explain(err, "scheduler: failed to schedule job %q", j.Name())
		}
	}
	return nil
}
