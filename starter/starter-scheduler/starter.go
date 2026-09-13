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
// A job is registered with its schedule, next to the work it describes:
//
//	scheduler.Provide("cleanup", svc.Cleanup, scheduler.Every(5*time.Minute))
//	scheduler.Provide("nightly", svc.Prune,   scheduler.Cron("0 3 * * *"))
//
// The three trigger kinds — [Every], [After] and [Cron] — wrap the triggers in
// [go-spring.org/cloud/scheduling]. Multi-replica de-duplication is opt-in per
// job via [WithLock], which names a lock.Locker bean contributed by
// starter-lock-{redis,etcd,consul}; only the replica that wins the lock runs the
// fire. See [go-spring.org/cloud/lock].
//
// Only process-level knobs come from configuration: `spring.scheduler.enabled`
// and `spring.scheduler.drain-timeout`. A job's cadence is part of what the job
// is, so it is not something an operator retunes from a config file — for
// ops-managed schedules use a job platform (e.g. starter-xxl-job) instead.
package StarterScheduler

import (
	"context"

	"go-spring.org/cloud/lock"
	"go-spring.org/cloud/scheduling"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName is the otel scope the job spans and metrics ride. Both go
// to the GLOBAL pipeline installed by starter-otel (or any SDK provider); this
// starter never builds its own, so without an SDK they are no-ops.
const instrumentationName = "go-spring.org/starter-scheduler"

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
	// registered with Provide or NewJob, each carrying its own schedule.
	Jobs []Job `autowire:"?"`

	// Lockers are all lock.Locker beans, keyed by bean name, so a job's `lock`
	// config key can reference one by name for multi-replica de-duplication.
	Lockers map[string]lock.Locker `autowire:"?"`

	sched       scheduling.Scheduler
	instruments instruments // built at wiring time in Run
}

// Run wires the configured jobs, then blocks until the application shuts down.
// It validates and builds every task before signalling readiness, so a
// misconfiguration (unknown job, bad cron, missing locker) fails startup rather
// than surfacing on a later fire. Scheduling begins only after the application
// is ready, so jobs never race application startup.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	log.Debugf(context.Background(), log.TagAppDef, "scheduler starting with %d job(s)", len(s.Jobs))

	// Resolve the instruments here, at wiring time rather than package init, so an
	// SDK installed later than this package's init still receives the records.
	s.instruments = newInstruments()
	s.sched = scheduling.NewScheduler(scheduling.WithObserver(s.observe))

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

// Stop halts scheduling and drains in-flight runs, bounded by the
// configured drain timeout. It is called during graceful shutdown with the
// framework's shutdown context, which is threaded into the scheduler drain.
func (s *Server) Stop(ctx context.Context) error {
	if s.sched == nil {
		return nil
	}
	if s.Config.DrainTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Config.DrainTimeout)
		defer cancel()
	}
	if err := s.sched.Stop(ctx); err != nil {
		log.Warnf(ctx, log.TagAppDef, "scheduler drain timed out: %v", err)
		return err
	}
	return nil
}

// build turns the registered Job beans into scheduled tasks. Each job carries its
// own trigger and options, so this validates the two things a job cannot know
// about itself: that its name is unique, and that the lock bean it names exists.
// Everything else was already checked when the job was registered.
func (s *Server) build() error {
	seen := make(map[string]bool, len(s.Jobs))
	for _, j := range s.Jobs {
		name := j.JobName()
		if seen[name] {
			return errutil.Explain(nil, "scheduler: duplicate job bean named %q", name)
		}
		seen[name] = true

		spec := j.Spec()
		opts := []scheduling.Option{scheduling.WithConcurrencyPolicy(spec.Concurrency)}
		if spec.Timeout > 0 {
			opts = append(opts, scheduling.WithTimeout(spec.Timeout))
		}
		if spec.Lock != "" {
			locker, ok := s.Lockers[spec.Lock]
			if !ok {
				return errutil.Explain(nil,
					"scheduler: job %q references lock %q but no lock.Locker bean of that name is registered", name, spec.Lock)
			}
			key := spec.LockKey
			if key == "" {
				key = name
			}
			adapter := lockerAdapter{l: locker, opts: lockTTLOption(spec.LockTTL)}
			opts = append(opts, scheduling.WithLock(adapter, key))
		}

		if _, err := s.sched.Schedule(name, j.Trigger(), s.instrument(name, j.Run), opts...); err != nil {
			return errutil.Explain(err, "scheduler: failed to schedule job %q", name)
		}
	}
	return nil
}

// instrument wraps a job's Run so each execution opens a span on the GLOBAL
// otel pipeline (the repo convention for protocol/component starters: never
// build a local provider here). The tracer comes from otel.Tracer, which is the
// no-op implementation unless starter-otel (or any SDK-based provider) has
// installed a global TracerProvider — so without otel the wrap is a zero-cost
// pass-through. The span carries the job name and the run's outcome (same
// vocabulary as the metrics, see observe.go); the error status and duration come
// from the run's result. Skipped fires emit no span — there is no run to trace,
// and the observer reports them as metrics and logs instead.
func (s *Server) instrument(name string, run scheduling.Job) scheduling.Job {
	return func(ctx context.Context) error {
		ctx, span := otel.Tracer(instrumentationName).Start(ctx, "scheduler.job "+name,
			trace.WithAttributes(
				attribute.String("scheduler.job.name", name),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		err := run(ctx)
		span.SetAttributes(attribute.String("scheduling.outcome", outcomeOfRun(err)))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
		return err
	}
}

// observe receives every fire — run or skipped — and hands it to [Server.record],
// which turns it into metrics and a log line. It must not block.
func (s *Server) observe(ev scheduling.Event) {
	s.record(ev)
}
