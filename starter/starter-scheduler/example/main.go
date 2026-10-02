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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/cloud/scheduling"
	"go-spring.org/spring/gs"

	// Blank-import the scheduler starter: it registers a gs.Server that
	// collects every *scheduling.Job bean and drives it. No network port is
	// opened — it is a global/infrastructure starter.
	_ "go-spring.org/starter-scheduler"
)

// Fire counters, incremented by the jobs so the smoke test can assert that each
// trigger kind actually fired.
var (
	tickCount   atomic.Int64 // fixed-rate job
	delayCount  atomic.Int64 // fixed-delay job
	lockedCount atomic.Int64 // fixed-rate job guarded by a lock
	reportCount atomic.Int64 // struct job with dependency injection
)

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

// The no-dependency jobs: each constructor returns a cloud scheduling.Job —
// the only job type there is. Trivial work is an inline closure; a struct is
// only worth it when the work carries state or dependencies (see reportWork);
// passed as a bound method value; the trigger and options are cloud types
// under their real names.
func NewTickJob() (*scheduling.Job, error) {
	return scheduling.NewJob("tick", scheduling.FixedRate(200*time.Millisecond),
		func(context.Context) error { tickCount.Add(1); return nil })
}

func NewDelayJob() (*scheduling.Job, error) {
	return scheduling.NewJob("delay", scheduling.FixedDelay(200*time.Millisecond),
		func(context.Context) error {
			delayCount.Add(1)
			time.Sleep(50 * time.Millisecond) // simulate work; fixed-delay never overlaps
			return nil
		})
}

func NewBeatJob() (*scheduling.Job, error) {
	tr, err := scheduling.ParseCron("* * * * *")
	if err != nil {
		return nil, err
	}
	// cron every minute — wired but too slow to fire in the smoke window
	return scheduling.NewJob("beat", tr, func(context.Context) error { return nil })
}

// The locked job's lock: the WithLock option takes the injected lock.Locker
// bean directly — no adapter, no extra type to learn — alongside the other
// job options. The TTL is the lease duration, auto-renewed while held.
func NewLockedJob(lk lock.Locker) (*scheduling.Job, error) {
	return scheduling.NewJob("locked", scheduling.FixedRate(200*time.Millisecond),
		func(context.Context) error { lockedCount.Add(1); return nil },
		scheduling.WithLock(lk, "locked", 5*time.Second))
}

// ExampleConfig is the struct job's own configuration, bound from
// ${spring.example.*} in conf/app.properties. Note what deliberately does NOT
// live here: the job's cadence — a schedule is part of what the job is, so it
// is declared in code (ReportJob.Trigger), not retuned from a config file.
type ExampleConfig struct {
	ReportEnabled bool  `value:"${report-enabled:=false}"`
	ReportLimit   int64 `value:"${report-limit:=3}"`
}

// The dependency-carrying job: the work and its state live in a plain struct
// whose constructor the container resolves — the same ctor-injection every
// other bean uses. Here the injected dependency is bound config; a
// *gormcore.DB or a client bean would ride along the same way. The constructor
// hands the struct's Run method (a bound method value) to scheduler.NewJob.
type reportWork struct {
	limit int64
}

func NewReportJob(cfg ExampleConfig) (*scheduling.Job, error) {
	w := &reportWork{limit: cfg.ReportLimit}
	return scheduling.NewJob("report", scheduling.FixedRate(200*time.Millisecond), w.run,
		scheduling.WithTimeout(100*time.Millisecond))
}

func (w *reportWork) run(ctx context.Context) error {
	n := reportCount.Add(1)
	if n%w.limit == 0 {
		// Every w.limit-th fire does the "real" work; the config value is real
		// input, not just wiring decoration.
		fmt.Printf("report job: aggregated %d fires\n", n)
	}
	return nil
}

func main() {
	flag.Parse()

	// Every job is a bean of the cloud type *scheduling.Job, registered with
	// gs.Provide so the container resolves its constructor's dependencies. No
	// Export is needed: the starter collects beans of exactly this type. The
	// job constructors are cloud types.
	// No .Name here: the scheduler collects Job beans by type, and the job's
	// identity (task name, default lock key, metric label) is the name inside
	// NewJob — a bean name would be a second, unused name for the same thing.
	gs.Provide(NewTickJob)
	gs.Provide(NewDelayJob)
	gs.Provide(NewBeatJob)   // constructor returns (job, err): bad cron fails wiring
	gs.Provide(NewLockedJob) // injects the "memory" lock.Locker bean below

	// Dependency injection at work: gs.Provide takes the constructor (config is
	// injected, bound with TagArg onto the ctor param), and the chain adds a
	// Condition — here on the very config key that enables it.
	gs.Provide(NewReportJob, gs.TagArg("${spring.example}")).
		Condition(gs.OnProperty("spring.example.report-enabled").HavingValue("true"))

	// An in-process lock.Locker named "memory" so the "locked" job's WithLock
	// reference resolves. In production this bean would be contributed by
	// starter-lock-{redis,etcd,consul} for cross-replica dedup.
	gs.Provide(lock.NewMemoryLocker()).Name("memory").Export(gs.As[lock.Locker]()).
		Destroy(func(l *lock.MemoryLocker) { _ = l.Close() })

	if !*manual {
		// Give readiness a moment to trigger and the scheduler to fire a few times.
		// This must run alongside gs.Run() below, not before it: the container — and
		// so the scheduler — has not started until gs.Run() is called.
		go func() {
			time.Sleep(1500 * time.Millisecond)
			runTest()
		}()
	} else {

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest() {
	tick := tickCount.Load()
	delay := delayCount.Load()
	locked := lockedCount.Load()
	report := reportCount.Load()

	fmt.Printf("fires: tick(fixed-rate)=%d delay(fixed-delay)=%d locked(lock)=%d report(struct+DI)=%d\n",
		tick, delay, locked, report)

	if tick < 3 {
		fmt.Println("ERROR: fixed-rate job did not fire enough times")
		os.Exit(1)
	}
	if delay < 2 {
		fmt.Println("ERROR: fixed-delay job did not fire enough times")
		os.Exit(1)
	}
	if locked < 1 {
		fmt.Println("ERROR: locked job never fired (lock wiring broken)")
		os.Exit(1)
	}
	if report < 1 {
		fmt.Println("ERROR: struct job never fired (bean/config injection broken)")
		os.Exit(1)
	}

	fmt.Println("starter-scheduler smoke test passed")
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init pins the working directory to this source file's directory so relative
// config paths resolve regardless of how the binary is invoked.
// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
