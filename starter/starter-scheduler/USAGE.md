# starter-scheduler Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `job.go`) and the runnable
[example/](example/) (smoke: `example/check.sh`, no docker) plus
[example-otel/](example-otel/) (docker-gated Jaeger). **Trigger semantics — cron parsing,
fixed-rate/fixed-delay, concurrency policies — come from
[go-spring.org/cloud/scheduling](../../cloud/scheduling)**;
this starter wires them into the gs lifecycle and adds per-job cross-replica locks.

**Activation**: the scheduler bean `schedulerServer` is provided when `spring.scheduler.enabled`
is true (**default on**, `MatchIfMissing`) **and** at least one `Job` bean exists
(starter.go:58-62) — an app importing the starter without registering jobs pays nothing. This is
a global/infrastructure starter: it opens no network port; it exports a `gs.Server` so jobs
participate in the server lifecycle.

---

## 1. Complete worked project

Four jobs — fixed-rate, fixed-delay, cron, and a lock-guarded fixed-rate — exactly
[example/example.go](example/example.go). File tree:

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod** (see `example/go.mod`):

```
module demo

require (
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-scheduler latest
    // optional, for cross-replica locks (example uses an in-process locker):
    go-spring.org/starter-lock-redis latest
)
```

**main.go**:

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/spring/gs"

    scheduler "go-spring.org/starter-scheduler"
)

func main() {
    // One Job bean per unit of work. Provide names the bean after the job,
    // exports it as Job so the scheduler collects it, and takes the schedule as
    // an option — the cadence is read here, beside the work it describes.
    scheduler.Provide("tick", func(ctx context.Context) error { // fixed-rate
        return nil
    }, scheduler.Every(200*time.Millisecond))
    scheduler.Provide("delay", func(ctx context.Context) error { // fixed-delay
        time.Sleep(50 * time.Millisecond) // never overlaps by construction
        return nil
    }, scheduler.After(200*time.Millisecond))
    scheduler.Provide("beat", func(ctx context.Context) error { // cron, 5-field
        return nil
    }, scheduler.Cron("* * * * *"))
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return nil // guarded by the lock below: only the holder runs
    }, scheduler.Every(200*time.Millisecond),
        scheduler.WithLock("memory"), scheduler.WithLockTTL(5*time.Second))

    // Cross-replica dedup: reference a lock.Locker bean BY BEAN NAME. In
    // production it comes from starter-lock-{redis,etcd,consul}; here an
    // in-process one stands in.
    ml := lock.NewMemoryLocker()
    gs.Provide(ml).Name("memory").Export(gs.As[lock.Locker]()).
        Destroy(func(l lock.Locker) { _ = ml.Close() })

    gs.Run()
}
```

**conf/app.properties** — the complete surface (copied from
`example/conf/app.properties`). Schedules are not here: each job declares its own
trigger at registration.

```properties
# Enabled by default; the line documents the knob.
spring.scheduler.enabled=true

# Bound on graceful-shutdown drain of in-flight runs.
spring.scheduler.drain-timeout=5s
```

In the snippet above: `Every(200ms)` is a fixed-rate trigger — fires every 200ms
measured from each scheduled fire time, with overlapping runs subject to
`WithConcurrency`. `After(200ms)` fires 200ms after the previous run *ends* and
never overlaps. `Cron("* * * * *")` is a standard 5-field expression (minute hour
dom month dow); 6-field-with-seconds expressions are rejected by `ParseCron`, and
an unparsable one panics at registration. `WithLock("memory")` names a
`lock.Locker` bean, `WithLockKey` (default: the job name) is the key acquired on
it, and `WithLockTTL` is the lease, auto-renewed while held.

**Verify**:

```bash
go run .                    # "fires: tick(fixed-rate)=N delay(fixed-delay)=M locked(lock)=K"
                            # then "starter-scheduler smoke test passed"
./example/check.sh          # scripted marker assert
# watch the fire log:
grep 'scheduler: job' <log>
```

### 1.1 Observability variant (example-otel)

[example-otel/](example-otel/main.go) adds `starter-otel` and exports traces via OTLP/gRPC to
Jaeger (`docker-compose.yml`: jaeger all-in-one with `COLLECTOR_OTLP_ENABLED`, :4317/:16686),
config in `example-otel/conf/app.properties`:

```properties
spring.observability.enable=true
spring.observability.service-name=scheduler-otel-example
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

```bash
cd example-otel && docker compose up -d && go run .
# then: curl -s 'http://127.0.0.1:16686/api/traces?service=scheduler-otel-example&limit=1'
# the example itself asserts "OK: traces found in Jaeger ..." before exiting
```

Scope note (verified against source): each job run opens a span on the GLOBAL otel pipeline
(`scheduler.job <name>` with the job-name attribute, `instrument` in starter.go — no-op
tracer unless starter-otel or any SDK provider is installed; this starter never builds its
own pipeline, per the protocol/component-starter convention). Spans carry the job name and
a `status` attribute (`ok|error|panic`); skipped fires emit no span — there is no run to
trace, and they reach the metrics and logs instead.

The same observer emits three metrics per fire (observe.go):

| Instrument | Type | Attributes | Answers |
|------------|------|------------|---------|
| `scheduling.runs` | counter | `job`, `status` | how many fires ended each way |
| `scheduling.run.duration` | histogram (s) | `job`, `status` | how long runs take |
| `scheduling.lag` | histogram (s) | `job` | how late runs start vs their planned instant |

`status` is one vocabulary covering every ending: `ok`, `error`, `panic`,
`skipped_policy`, `skipped_lock` — a fire a concurrency policy dropped, versus one another
replica was running. The job name is a metric dimension because it is fixed in code, unlike
a lock key, whose cardinality is unbounded.

Each fire also writes one log line (observe.go) — the per-fire access log, on its own tag
`_app_scheduler_access` (`log.RegisterAppTag("scheduler", "access")`) so it can be selected
apart from application logs. The lifecycle lines (starting / started / drain) stay on the
default app tag. The line carries the same `job` and `status` the metrics just recorded,
plus `reason` on a skip and `duration_ms` / `error` on a run — so `scheduling.runs{job,status}`
and `scheduling.run.duration{job,status}` join the line that explains them. Failures and
panics are Error level, a skip and a successful run are Debug.

`lag` is the scheduler's own health signal: it is not drift — the next fire stays anchored
on the planned instant — but a rising lag means runs are starting later and later, which
points at a saturated or stalled process and which no other instrument in the framework
would show. The Prometheus endpoint renders these with dots as underscores and a `_total`
suffix on the counter (`scheduling_runs_total`). example-otel asserts all three appear on
`:9090/metrics` after its run, alongside the trace check.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-scheduler
  └─ init: gs.Provide(&Server{}).Name("schedulerServer")
        .Condition(OnProperty("spring.scheduler.enabled").HavingValue("true").MatchIfMissing())
        .Condition(OnBean[Job]())
        .Export(gs.As[gs.Server]())                                 [starter.go:58-62]
gs.Run()
  ├─ config bind: ${spring.scheduler} → Server.Config (NO expr validation —
  │  see §2.2 asymmetry), plus field injection:
  │    Jobs    []Job                 `autowire:"?"`   (all Job beans)
  │    Lockers map[string]lock.Locker `autowire:"?"`  (all locker beans, by name)
  ├─ Rooter Init phase: your Provide()d jobs and lockers are beans already;
  │    nothing scheduler-side runs here
  ├─ Runner phase: Server.Run (starter.go:87-105)
  │   ├─ build() — validates what a job cannot know about itself, BEFORE readiness:
  │   │    duplicate job bean → error
  │   │    lock references a missing locker bean → error
  │   │    (the trigger and its options were already settled at registration:
  │   │      missing/duplicated trigger, non-positive duration or bad cron
  │   │      panic in Provide/NewJob, i.e. during startup — see §2.2)
  │   ├─ <-sig.TriggerAndWait() → readiness flips AFTER build succeeds
  │   └─ sched.Start(ctx) — "Scheduling begins only after the application is
  │        ready, so jobs never race application startup" (starter.go:85-86)
  └─ on SIGTERM: Stop wraps ctx with drain-timeout and drains in-flight
       runs via sched.Stop(ctx); a timeout logs "scheduler drain timed out"
```

### 2.2 Where a job is validated (verified in source)

Validation is split by what each layer can know:

- **At registration** — `Provide`/`NewJob` panic, i.e. during startup, on a missing or
  duplicated trigger, a non-positive `Every`/`After` duration, or an unparsable cron
  expression (job.go). There is **no** `expr` validation and no `JobConfig` to bind
  (config.go binds only `drain-timeout`): the schedule never travels through config, so the
  name-keyed coupling between a `jobs.<name>.*` entry and a bean — and its asymmetry, where
  config→bean was an error but bean→config only a warning, so a bean-side typo left a job
  that silently never fired — no longer exists.
- **At `Server.Run → build()`** (starter.go) — the two things a job cannot know about itself:
  a duplicate job name, and a `WithLock` naming a locker bean that is not in the container.
  Both are errors before readiness, so a misconfigured job fails startup rather than
  surfacing on a later fire.

### 2.3 Named-lock cross-replica semantics (verified in source)

1. Every `lock.Locker` bean in the container is collected into `Server.Lockers`, keyed by
   **bean name** (`autowire:"?"` map, starter.go). Contributing backends:
   `starter-lock-redis` / `-etcd` / `-consul` (or your own bean, as in the example).
2. A job opts in with `WithLock("<bean-name>")` — a name, not the bean, because the bean
   does not exist yet when the job is registered. `build()` resolves it and **fails fast** if
   the bean does not exist (starter.go).
3. The acquired key is `WithLockKey`, **defaulting to the job name** — "so two jobs sharing a
   locker do not collide" (job.go). Across replicas: same locker + same key = the same
   distributed lock.
4. `WithLockTTL` becomes `lock.WithTTL` (job.go); the lock package **auto-renews the lease
   while the job holds it**, so a run longer than the TTL keeps the lease. Zero — the
   default — keeps the locker's own default, so set a TTL above a typical run only if that
   default is too tight for this job.
5. Each fire calls `TryAcquire` (lockerAdapter, job.go): only the replica that wins
   runs the fire; the others skip (logged at Debug as "skipped").

### 2.4 One fire, layer by layer

1. The scheduling trigger (cron spec / fixed-rate timer / fixed-delay timer) fires.
2. Concurrency policy applies (`skip` default / `queue` / `replace`; no effect on
   fixed-delay, which never overlaps by construction).
3. If a lock is configured: `TryAcquire(key)` — loss ⇒ skip with reason.
4. If `timeout > 0`: the run ctx is wrapped with a timeout.
5. `Job.Run(ctx)` executes; the Observer seam fires with `{Name, Duration, Err, Skipped,
   Reason}` and the starter logs it — Debug for ran/skipped, Error for failures
   (starter.go:196-206).

---

## 3. Per-key behavior reference

Prefix `spring.scheduler.*` (single global scheduler — no multi-instance). The Server's
`Config` field binds `${spring.scheduler}` directly (starter.go:68-69), so these are the
absolute keys.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.scheduler.enabled` | bool | true (MatchIfMissing) | Master switch; false removes the bean even if jobs exist. | Set false by accident → all jobs silently stop. |
| `spring.scheduler.drain-timeout` | duration | 30s | Bounds Stop's drain of in-flight runs (starter.go). | Too low → runs abandoned mid-flight on deploy (no retry — unlike a queue). |

That is the whole config surface. Everything else about a job is a registration
option, so this second table is a reference for the `JobOption`s rather than for
config keys:

| Option | Type | Default | Behavior / interactions | Misconfiguration consequence |
|--------|------|---------|-------------------------|------------------------------|
| `Every(d)` | duration | — | Fixed-rate: fires every `d` from each scheduled fire time; overlaps governed by `WithConcurrency`. | `d <= 0` → panic at registration. |
| `After(d)` | duration | — | Fixed-delay: fires `d` after the previous run ends; never overlaps; `WithConcurrency` ignored. | `d <= 0` → panic at registration. |
| `Cron(expr)` | string | — | 5-field cron (`ParseCron` rejects anything else; `@macros` supported). | Unparsable → panic at registration. A 6-field-with-seconds expression is the usual mistake. |
| — | — | — | A job must declare **exactly one** of the three triggers above. | None, or two → panic at registration. |
| `WithTimeout(d)` | duration | 0 (off) | `>0` cancels the run ctx after it elapses. | 0 → a hung job blocks nothing but runs forever (and, if locked, holds the lease). |
| `WithConcurrency(p)` | `scheduling.ConcurrencyPolicy` | `Skip` | `Skip` \| `Queue` \| `Replace` — a typed value, so no invalid one is expressible. | — |
| `WithLock(bean)` | string | empty | Bean name of a `lock.Locker`; each fire TryAcquires — only the holder runs. ⚠ must match a locker bean's name exactly (fail-fast otherwise, in `build()`). | Unknown name → startup error before readiness. |
| `WithLockKey(k)` | string | job name | The key acquired on the locker — the cross-replica coordination point. ⚠ two jobs intentionally serialized must share locker AND key. | Accidental key sharing across jobs → they serialize against each other. |
| `WithLockTTL(d)` | duration | locker's default | Lease duration, auto-renewed while held. | Below a typical run + renewal jitter → lease lost mid-run, second replica starts. |

---

## 4. Verification & fault drills

### 4.1 Jobs execute

```bash
go run .                       # fires: tick(...)≥3 delay(...)≥2 locked(...)≥1
./example/check.sh             # asserts the marker "starter-scheduler smoke test passed"
grep 'scheduler: job' <log>    # "job \"tick\" ran in ...", "job \"locked\" ran in ..."
```

### 4.2 Lock de-duplication (cross-replica drill)

Run the example twice concurrently (same machine stands in for two replicas) with a
shared-backend locker — with the in-process `memory` locker each process dedups only itself,
so for a real drill swap in starter-lock-redis:

```bash
docker run -d -p 6379:6379 redis:7
# configure spring.lock.instances.redis... bean "redis" + WithLock("redis") in example.go, then:
go run . & go run . & wait
# exactly one process logs "job \"cleanup\" ran"; the other logs nothing for it
```

### 4.3 Failure and skip observability

Make a job return an error, and another a slow run with `scheduler.Every(100*time.Millisecond)`
and the default `Skip` policy:

```
scheduler: job "bad" failed after 10ms: boom          # Error level
scheduler: job "slow" skipped (previous run still in flight)   # Debug, reason string
```

A panicking job is reported the same way, at Error level, with `panicked` rather than
`failed` — its error wraps `scheduling.ErrJobPanicked`, so `errors.Is` separates a panic
from a run that merely returned an error.

The same fires are counted in metrics (see §1.1): `scheduling.runs{status="error"}` climbs
for the failing job, and the two skip kinds are told apart — `skipped_policy` means the job
is not keeping up, while `skipped_lock` is routine when several replicas share a schedule.

These lines are on the `_app_scheduler_access` tag, so they can be routed independently of
the application's own logs (`logger.scheduler_access.tag=_app_scheduler_access`); the
lifecycle lines stay on the default app tag.

There is no retry: a failed run is logged and the next fire is per schedule. Wrap your own
retry (or push the work into asynq) if you need one.

### 4.4 Drain on shutdown

Start a 3s job, `kill -TERM` mid-run: shutdown waits up to `drain-timeout`; set it to 1s to
watch `scheduler drain timed out: context deadline exceeded` (Warn).

### 4.5 Validation drills (fail fast)

Each of these panics at registration — that is, during startup, before readiness:

```go
scheduler.Provide("orphan", fn)                          // → "has no trigger"
scheduler.Provide("tick", fn, scheduler.Every(0))        // → "requires a positive duration"
scheduler.Provide("beat", fn,
    scheduler.Every(time.Second), scheduler.Cron("* * * * *"))  // → "sets more than one trigger"
scheduler.Provide("beat", fn, scheduler.Cron("0 */5 * * * *"))  // → "must have 5 fields, got 6"
```

And these fail in `build()`, still before readiness:

```go
scheduler.Provide("tick", fn, scheduler.Every(time.Second))
scheduler.Provide("tick", fn, scheduler.Every(time.Second))     // → "duplicate job bean"
scheduler.Provide("tick", fn, scheduler.Every(time.Second),
    scheduler.WithLock("nope"))                                 // → "references lock \"nope\" ..."
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Panic at startup "has no trigger" | `Provide`/`NewJob` called without `Every`/`After`/`Cron` | Add exactly one trigger option. |
| "sets more than one trigger" | Two trigger options on one job | Keep one. |
| "must have 5 fields" on cron | Seconds-style 6-field expression | Use 5-field (`* * * * *`). |
| "references lock %q but no lock.Locker bean" | `WithLock` value isn't a locker bean name | Match the starter-lock bean name exactly. |
| "duplicate job bean named %q" | Two jobs registered under the same name | Rename one; the name is the task name and the default lock key. |
| Both replicas run the "locked" job | Locker is per-process (memory) or `WithLockKey` differs between deployments | Use a shared backend and the same key; key defaults to the job name. |
| Runs abandoned on deploy | `drain-timeout` shorter than the longest run | Raise it; remember there is no re-run of drained jobs. |
| Two jobs mysteriously serialize | Same locker AND same `WithLockKey` | Give each job its own key (default already does). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 2 (both process-level; job schedules are code) |
| Required | 0 keys; exactly one trigger per job is enforced at registration |
| Quickstart external deps | 0 (a lock backend only for cross-replica dedup) |
| "Watch out" entries | 3 |

Design suspects (kept from the prior edition, plus new findings):

- ~~Job ↔ config coupling by name across two sources, validated asymmetrically (config→bean
  an error, bean→config only a warning, so a bean-side typo yielded a silently dead job)~~
  Fixed: the schedule moved to registration, so there is no second source and no name lookup
  to get wrong. The whole `jobs.<name>.*` surface is gone.
- A job's cadence can no longer be retuned without a rebuild. Deliberate — an ops-tunable
  cadence is what a job platform (`starter-xxl-job`) is for — but it is the one thing the
  config surface previously bought.
- ~~The observe seam is log-only~~ Fixed: each job run opens a span on the GLOBAL otel
  pipeline (`scheduler.job <name>` via otel.Tracer, starter.go instrument), and every fire —
  run or skipped — feeds the three metrics in observe.go (`scheduling.runs`,
  `scheduling.run.duration`, `scheduling.lag`). All of it is a no-op unless starter-otel (or
  any SDK provider) is present. This needed one core addition: panics now wrap
  `ErrJobPanicked` (cloud/scheduling), so the observer can count them as their own status
  value instead of matching on an error string.
- ~~The package doc comment's cron example is 6-field~~ Fixed: the example is now 5-field
  (`*/5 * * * *`), matching ParseCron (scheduling/cron.go:81-83).
- No `Init`-phase validation: all job validation happens in `Run/build()`; an error surfaces
  at Runner time (still pre-readiness, but later than the bind-time validation other starters
  do via `expr`).
- `drain-timeout` expiry abandons in-flight runs with no re-run or handoff — acceptable for
  idempotent jobs, undocumented as a policy for others.
