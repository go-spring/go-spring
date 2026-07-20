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

package StarterBatch

import (
	"fmt"

	"go-spring.org/spring/gs"
	"go-spring.org/spring/batch"
)

var _ JobDefinition = (*staticJob)(nil)

// JobDefinition is the seam between the application and the batch runner: the
// application registers one JobDefinition bean per job, and the starter matches
// each to a ${spring.batch.jobs.<name>} entry by JobName to learn its startup /
// launch options. Because a [batch.Job] holds generic [batch.ChunkStep]s, a
// plain "job func" cannot express the whole graph — the app hands the runner a
// *builder* bean and the runner calls Build to materialise the concrete Job.
//
// This mirrors the "app owns the work, starter owns the runner" split used by
// starter-scheduler (see [go-spring.org/starter-scheduler].Job), adjusted for
// the fact that steps are typed.
//
// Register a JobDefinition with [Provide]:
//
//	batch.Provide("reconcile", reconcileStep)
type JobDefinition interface {
	// JobName is the name that ties this job to its config entry. It must be
	// unique across all JobDefinition beans in the container.
	JobName() string
	// Build materialises the concrete [*batch.Job]. It is called once at
	// startup (fail-fast wiring) and again by [Launcher.Launch]; a builder that
	// captures shared state should treat repeated Build calls as returning
	// equivalent jobs.
	Build() (*batch.Job, error)
}

// NewJob adapts a name and a set of [batch.Step]s into a [JobDefinition] bean.
// It panics on an empty name, since a nameless job cannot be matched to config.
// An empty step list is legal (Build returns a job with no steps), but almost
// always a mistake — the runner will log a warning at wiring time.
//
// NewJob returns the [JobDefinition] value only; callers who use it directly
// must register it with the container themselves, naming the bean and exporting
// it as JobDefinition so the runner can collect it:
//
//	gs.Provide(batch.NewJob("reconcile", reconcileStep)).
//	    Name("reconcile").Export(gs.As[batch.JobDefinition]())
//
// Prefer [Provide], which does exactly that in one call.
func NewJob(name string, steps ...batch.Step) JobDefinition {
	if name == "" {
		panic("batch: job definition name must not be empty")
	}
	return &staticJob{name: name, steps: steps}
}

// Provide registers a batch job in one call: it wraps steps in a
// [JobDefinition] bean, names the bean after the job, and exports it as
// JobDefinition so the runner collects it and matches it to its
// ${spring.batch.jobs.<name>} config entry. This is the idiomatic way an
// application declares a batch job:
//
//	batch.Provide("reconcile", reconcileStep)
//
// For advanced cases that need a dynamic step graph (e.g. steps that depend on
// other beans), implement [JobDefinition] on your own type and register it with
// gs.Provide directly, exporting as JobDefinition.
func Provide(name string, steps ...batch.Step) {
	gs.Provide(NewJob(name, steps...)).
		Name(name).
		Export(gs.As[JobDefinition]())
}

// staticJob is a JobDefinition whose steps are fixed at registration time. It
// is the shape [NewJob] returns; applications with dynamic job graphs supply
// their own JobDefinition implementation.
type staticJob struct {
	name  string
	steps []batch.Step
}

func (j *staticJob) JobName() string { return j.name }

func (j *staticJob) Build() (*batch.Job, error) {
	if j.name == "" {
		return nil, fmt.Errorf("batch: job definition has empty name")
	}
	// Copy the slice so a caller mutating the returned job does not affect the
	// definition's own step list.
	steps := make([]batch.Step, len(j.steps))
	copy(steps, j.steps)
	return &batch.Job{Name: j.name, Steps: steps}, nil
}
