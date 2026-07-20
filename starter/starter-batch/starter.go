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

// Package StarterBatch runs [go-spring.org/spring/batch] jobs as part of the
// Go-Spring application lifecycle. Blank-importing this package registers a
// batch runner that:
//
//   - collects every [JobDefinition] bean the application registered with
//     [Provide],
//   - materialises them at wiring time (fail-fast on config typos),
//   - launches any job with `spring.batch.jobs.<name>.run-on-startup=true` once
//     the application is ready — the "Spring Cloud Task" shape,
//   - exports a [*Launcher] bean so a scheduler job (or an HTTP handler) can
//     launch any registered job on demand — the shape you want for periodic
//     batches.
//
// This is a global / infrastructure-archetype starter (see starter/DESIGN.md
// §2.4): it opens no network port. Instead it exports a [gs.Server] so the
// runner participates in the server lifecycle — startup launches begin once the
// application is ready and, on SIGTERM, Stop drains the in-flight launches
// before the process exits (the drain the graceful-shutdown orchestration
// expects).
//
// The progress store — the [batch.JobRepository] seam — is a plain bean, so a
// durable backend is a separate integration module (e.g. starter-batch-redis)
// that contributes a [batch.JobRepository] bean. When no repository bean is
// present the runner falls back to [batch.NewMemoryRepository] (in-process
// only), which is fine for tests and single-process demos but does not survive
// a restart.
//
// Register jobs from the application:
//
//	batch.Provide("reconcile", reconcileStep)
//
// and declare which of them run on startup in configuration:
//
//	spring.batch.jobs.reconcile.run-on-startup=true
//	spring.batch.jobs.reconcile.params.date=2026-07-19
//
// For periodic batches, register a scheduler.Job that injects [*Launcher] and
// calls Launch; the schedule is declared under `spring.scheduler.jobs.*` and
// the batch run participates in the same drain as the scheduler.
package StarterBatch

import (
	"context"
	"sync"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/batch"
)

// enabled matches when the starter is not explicitly disabled. It is the
// baseline gate for both bean contributions so an app that sets
// spring.batch.enabled=false pays nothing.
var enabled = gs.OnProperty("spring.batch.enabled").HavingValue("true").MatchIfMissing()

func init() {
	// The Launcher bean. Its exported fields (Config, Defs, Repos) are
	// autowired by the container; Init resolves them into (repo, defs) at
	// wiring time, so a mis-named repo or a duplicate JobDefinition fails boot
	// rather than surfacing on a later launch.
	//
	// Both this and the Server bean gate on OnBean[JobDefinition]() so an app
	// that imports the starter without registering any jobs pays nothing.
	gs.Provide(&Launcher{}).
		Name("batchLauncher").
		Init((*Launcher).Init).
		Condition(enabled, gs.OnBean[JobDefinition]())

	// The Server drives run-on-startup jobs and participates in the graceful
	// drain. It autowires *Launcher so a startup launch and a manual launch go
	// through the exact same code path.
	gs.Provide(&Server{}).
		Name("batchServer").
		Condition(enabled, gs.OnBean[JobDefinition]()).
		Export(gs.As[gs.Server]())
}

// Server drives the run-on-startup batch jobs and plugs the runner into the
// Go-Spring server lifecycle. Its exported fields are populated by the IoC
// container.
type Server struct {
	// Config is bound from ${spring.batch}. It carries the drain timeout and
	// the per-job run-on-startup + params entries the Server iterates over.
	Config Config `value:"${spring.batch}"`

	// Launcher is the shared launcher — the same one manual callers inject —
	// used to fire startup launches. Sharing it means startup and manual
	// launches see the same (repo, defs) map.
	Launcher *Launcher `autowire:""`

	// wg tracks in-flight startup launches so Stop can drain them.
	wg sync.WaitGroup
	// cancel cancels every in-flight launch on Stop.
	cancel context.CancelFunc
}

// Run blocks until the application shuts down, having launched any run-on-
// startup job in the background once the application is ready. It does not
// perform wiring — the Launcher's Init already validated everything at boot, so
// Run has nothing left to fail on.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Fire startup launches ("Cloud Task" jobs) in the background so a
	// long-running one does not block the application from serving other work.
	// A scheduler.Job that calls Launcher.Launch is the other shape — those
	// runs are triggered by the scheduler, not here.
	launched := 0
	for name, jc := range s.Config.Jobs {
		if !jc.RunOnStartup {
			continue
		}
		jobName := name
		params := batch.Params{}
		for k, v := range jc.Params {
			params[k] = v
		}
		launched++
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			log.Infof(runCtx, log.TagAppDef, "batch: launching job %q on startup", jobName)
			je, err := s.Launcher.Launch(runCtx, jobName, params)
			if err != nil {
				log.Errorf(runCtx, log.TagAppDef, "batch: job %q failed: %v", jobName, err)
				return
			}
			log.Infof(runCtx, log.TagAppDef, "batch: job %q finished with status=%s", jobName, je.Status)
		}()
	}
	log.Infof(ctx, log.TagAppDef,
		"batch runner started (%d job definition(s), %d run-on-startup)", len(s.Launcher.Names()), launched)

	<-ctx.Done()
	return nil
}

// Stop cancels in-flight startup launches and waits for them to finish,
// bounded by the configured drain timeout. It is called during graceful
// shutdown. On-demand launches triggered through the shared Launcher after Run
// has returned are not tracked here — their lifetime is owned by their caller
// (e.g. the scheduler, which drains its own goroutines).
func (s *Server) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	timeout := s.Config.DrainTimeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		log.Warnf(context.Background(), log.TagAppDef,
			"batch: drain timed out after %s; abandoning in-flight launches", timeout)
		return nil
	}
}
