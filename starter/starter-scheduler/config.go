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

import "time"

// Config binds ${spring.scheduler}. It declares the set of scheduled jobs and a
// drain timeout for graceful shutdown. Each entry under jobs is matched to a
// registered [Job] bean of the same name; a configured job with no matching bean
// (or a job bean with no configured trigger) is a fail-fast startup error.
type Config struct {
	// Jobs maps a job name to its trigger and options. The name must match a
	// registered Job bean (see NewJob).
	Jobs map[string]JobConfig `value:"${jobs:=}"`

	// DrainTimeout bounds how long Stop waits for in-flight runs to finish during
	// graceful shutdown before giving up. It is a safety net on top of the
	// framework-level app.shutdown.timeout.
	DrainTimeout time.Duration `value:"${drain-timeout:=30s}"`
}

// JobConfig declares one scheduled job's trigger and execution options. Exactly
// one of Cron, FixedRate or FixedDelay must be set; the starter fails fast
// otherwise, because a job with no trigger (or two) is a configuration mistake
// that would silently never fire the way the operator expects.
type JobConfig struct {
	// Cron is a standard 5-field cron expression (see
	// go-spring.org/stdlib/scheduling.ParseCron). Mutually exclusive with
	// FixedRate and FixedDelay.
	Cron string `value:"${cron:=}"`

	// FixedRate fires every interval measured from each scheduled fire time; runs
	// may overlap subject to Concurrency. Mutually exclusive with the others.
	FixedRate time.Duration `value:"${fixed-rate:=0}"`

	// FixedDelay fires this long after the previous run finishes; runs never
	// overlap. Mutually exclusive with the others.
	FixedDelay time.Duration `value:"${fixed-delay:=0}"`

	// Timeout, when positive, bounds a single run: the job's context is cancelled
	// after it elapses.
	Timeout time.Duration `value:"${timeout:=0}"`

	// Concurrency governs overlapping fixed-rate/cron runs: "skip" (default),
	// "queue" or "replace". It has no effect on fixed-delay jobs.
	Concurrency string `value:"${concurrency:=skip}"`

	// Lock names a lock.Locker bean (provided by starter-lock-{redis,etcd,consul})
	// that de-duplicates this job across replicas: each fire acquires the lock and
	// only the holder runs. Empty means run on every replica.
	Lock string `value:"${lock:=}"`

	// LockKey is the key acquired on the locker; it defaults to the job name when
	// empty, so two jobs sharing a locker do not collide.
	LockKey string `value:"${lock-key:=}"`

	// LockTTL is the lease duration for the acquired lock. It should exceed a
	// typical run so the lease is not lost mid-run; the lock package auto-renews
	// it while the job holds it.
	LockTTL time.Duration `value:"${lock-ttl:=30s}"`
}
