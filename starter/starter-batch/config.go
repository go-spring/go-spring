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

import "time"

// Config binds ${spring.batch}. It declares the drain-timeout for graceful
// shutdown, an optional named repository, and the set of jobs that participate
// in startup / launch. A configured job with no matching [JobDefinition] bean
// (or a JobConfig referencing an unknown repo) is a fail-fast startup error, so
// a typo surfaces at boot rather than when someone tries to launch the job.
type Config struct {
	// Repository names a [batch.JobRepository] bean to use as the progress
	// store. When empty the runner picks the sole repo bean if there is one, or
	// falls back to [batch.NewMemoryRepository] (in-process only — not durable
	// across restarts). Setting this to a name that does not exist is a
	// fail-fast startup error.
	Repository string `value:"${repository:=}"`

	// DrainTimeout bounds how long Stop waits for in-flight launches to finish
	// during graceful shutdown before giving up. It is a safety net on top of
	// the framework-level app.shutdown.timeout.
	DrainTimeout time.Duration `value:"${drain-timeout:=30s}"`

	// Jobs maps a job name to its launch options. The name must match a
	// registered [JobDefinition] bean. A JobDefinition bean with no matching
	// entry is legal — it can still be launched manually via [Launcher.Launch]
	// (e.g. from a scheduler job).
	Jobs map[string]JobConfig `value:"${jobs:=}"`
}

// JobConfig declares one batch job's launch options.
//
// Setting RunOnStartup=true makes the job a *Cloud Task*: it is launched once
// when the application becomes ready. Leaving RunOnStartup=false means the job
// is only launched on demand — typically by a scheduler.Job that injects
// [*Launcher] and calls Launch, giving the same job a cron/fixed-rate trigger.
type JobConfig struct {
	// RunOnStartup, when true, launches the job once after the application is
	// ready. It is the Cloud Task shape: a one-shot run whose outcome is
	// recorded in the [batch.JobRepository]. Restart semantics still apply — a
	// prior incomplete run of the same (name, params) instance is resumed.
	RunOnStartup bool `value:"${run-on-startup:=false}"`

	// Params are the startup launch parameters. Together with the job name they
	// identify the job instance in the repository, so changing a parameter
	// creates a new instance rather than restarting the previous one.
	Params map[string]string `value:"${params:=}"`
}
