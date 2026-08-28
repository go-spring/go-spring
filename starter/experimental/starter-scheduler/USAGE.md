# starter-scheduler Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `job.go`) and the runnable
[example/](example/) (smoke: `example/check.sh`, no docker) plus
[example-otel/](example-otel/) (docker-gated Jaeger). **Trigger semantics — cron parsing,
fixed-rate/fixed-delay, concurrency policies — come from
[go-spring.org/cloud/experimental/scheduling](../../../cloud/experimental/scheduling)**;
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

    "go-spring.org/cloud/experimental/lock"
    "go-spring.org/spring/gs"

    scheduler "go-spring.org/starter-scheduler"
)

func main() {
    // One Job bean per unit of work. Provide names the bean after the job and
    // exports it as Job, so the scheduler collects it and matches it to its
    // ${spring.scheduler.jobs.<name>} config entry.
    scheduler.Provide("tick", func(ctx context.Context) error { // fixed-rate
        return nil
    })
    scheduler.Provide("delay", func(ctx context.Context) error { // fixed-delay
        time.Sleep(50 * time.Millisecond) // never overlaps by construction
        return nil
    })
    scheduler.Provide("beat", func(ctx context.Context) error { // cron, 5-field
        return nil
    })
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return nil // guarded by the lock below: only the holder runs
    })

    // Cross-replica dedup: reference a lock.Locker bean BY BEAN NAME. In
    // production it comes from starter-lock-{redis,etcd,consul}; here an
    // in-process one stands in.
    ml := lock.NewMemoryLocker()
    gs.Provide(ml).Name("memory").Export(gs.As[lock.Locker]()).
        Destroy(func(l lock.Locker) { _ = ml.Close() })

    gs.Run()
}
```

**conf/app.properties** — the complete, commented surface (copied from
`example/conf/app.properties`):

```properties
# Enabled by default; the line documents the knob.
spring.scheduler.enabled=true

# Bound on graceful-shutdown drain of in-flight runs.
spring.scheduler.drain-timeout=5s

# Fixed-rate: fires every 200ms measured from each scheduled fire time;
# overlapping runs subject to <job>.concurrency.
spring.scheduler.jobs.tick.fixed-rate=200ms

# Fixed-delay: fires 200ms after the previous run ENDS; never overlaps.
spring.scheduler.jobs.delay.fixed-delay=200ms

# Cron: standard 5-field expression (minute hour dom month dow), every minute
# here. NOTE: 6-field-with-seconds expressions are rejected by ParseCron.
spring.scheduler.jobs.beat.cron=* * * * *

# The lock-guarded job: `lock` names a lock.Locker bean; the acquired key is
# `lock-key` (default: the job name); the lease is `lock-ttl`, auto-renewed
# while held.
spring.scheduler.jobs.cleanup.fixed-rate=200ms
spring.scheduler.jobs.cleanup.lock=memory
spring.scheduler.jobs.cleanup.lock-ttl=5s
```

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
own pipeline, per the protocol/component-starter convention). Outcome/duration logs come
from the log observer (`observe` in starter.go); skipped fires are log-only (no span, since
no run happened). example-otel additionally proves the app's starter-otel pipeline end to
end and exposes a metrics port for your own instrumentation — per-job scheduler metrics are
still a gap (see §6).

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
  │   ├─ build() — FULL validation here, BEFORE readiness:
  │   │    duplicate job bean → error
  │   │    job bean without a config entry → WARNING only (never runs)
  │   │    config entry without a Job bean → error
  │   │    trigger exclusivity (exactly one of cron/fixed-rate/fixed-delay,
  │   │      job.go:93-110) / bad cron / bad concurrency → error
  │   │    lock references a missing locker bean → error
  │   ├─ <-sig.TriggerAndWait() → readiness flips AFTER build succeeds
  │   └─ sched.Start(ctx) — "Scheduling begins only after the application is
  │        ready, so jobs never race application startup" (starter.go:85-86)
  └─ on SIGTERM: StopContext wraps ctx with drain-timeout and drains in-flight
       runs via sched.Stop(ctx); a timeout logs "scheduler drain timed out"
```

### 2.2 The asymmetric validation (verified in source)

There is **no** `expr` validation on `Config`/`JobConfig` (config.go has none). All validation
lives in `Server.Run → build()` (starter.go:134-191), i.e. at Runner time — but still before
readiness, so a misconfigured job fails startup rather than surfacing on a later fire. The
asymmetry is in the **two directions** of the name coupling:

- config entry → no Job bean: hard **error** ("job %q is configured but no Job bean of that
  name is registered", starter.go:154-157);
- Job bean → no config entry: **warning only** — "job bean %q has no
  ${spring.scheduler.jobs.%s} entry; it will not run" (starter.go:144-150).

So a typo on the config side fails fast; a typo on the bean side (or a forgotten config entry)
produces a silently dead job with one warning line.

### 2.3 Named-lock cross-replica semantics (verified in source)

1. Every `lock.Locker` bean in the container is collected into `Server.Lockers`, keyed by
   **bean name** (`autowire:"?"` map, starter.go:76-77). Contributing backends:
   `starter-lock-redis` / `-etcd` / `-consul` (or your own bean, as in the example).
2. A job opts in with `lock: <bean-name>`; `build()` resolves it and **fails fast** if the
   bean does not exist (starter.go:172-177).
3. The acquired key is `lock-key`, **defaulting to the job name** — "so two jobs sharing a
   locker do not collide" (config.go:66-68). Across replicas: same locker + same key = the
   same distributed lock.
4. `lock-ttl` becomes `lock.WithTTL` (job.go:156-161); the lock package **auto-renews the
   lease while the job holds it**, so a run longer than the TTL keeps the lease
   (config.go:70-73, job.go:139-142). Set TTL above a typical run anyway so the first lease
   window is comfortable.
5. Each fire calls `TryAcquire` (lockerAdapter, job.go:148-154): only the replica that wins
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
| `spring.scheduler.drain-timeout` | duration | 30s | Bounds Stop's drain of in-flight runs (starter.go:120-124). | Too low → runs abandoned mid-flight on deploy (no retry — unlike a queue). |
| `jobs.<n>.cron` | string | — | 5-field cron (`ParseCron` rejects anything else, scheduling/cron.go:81-83; `@macros` supported). ⚠ exactly one of cron/fixed-rate/fixed-delay — 0 or >1 is a build-time error (job.go:93-110). | Bad/ambiguous trigger → startup error before readiness. |
| `jobs.<n>.fixed-rate` | duration | 0 | Fires every interval from each scheduled fire time; overlaps governed by `concurrency`. | — |
| `jobs.<n>.fixed-delay` | duration | 0 | Fires this long after the previous run ends; never overlaps; `concurrency` ignored. | — |
| `jobs.<n>.timeout` | duration | 0 (off) | `>0` cancels the run ctx after it elapses. | 0 → a hung job blocks nothing but runs forever (and, if locked, holds the lease). |
| `jobs.<n>.concurrency` | string | skip | `skip` \| `queue` \| `replace` (case-insensitive, job.go:126-137). | Any other value → startup error. |
| `jobs.<n>.lock` | string | empty | Bean name of a `lock.Locker`; each fire TryAcquires — only the holder runs. ⚠ must match a locker bean's name exactly (fail-fast otherwise, starter.go:172-177). | Unknown name → startup error. |
| `jobs.<n>.lock-key` | string | job name | The key acquired on the locker — the cross-replica coordination point. ⚠ two jobs intentionally serialized must share locker AND key. | Accidental key sharing across jobs → they serialize against each other. |
| `jobs.<n>.lock-ttl` | duration | 30s | Lease duration, auto-renewed while held (job.go:139-161). | Below a typical run + renewal jitter → lease lost mid-run, second replica starts. |

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
# configure spring.lock.redis... bean "redis" + jobs.cleanup.lock=redis, then:
go run . & go run . & wait
# exactly one process logs "job \"cleanup\" ran"; the other logs nothing for it
```

### 4.3 Failure and skip observability

Make a job return an error and another a slow run with `concurrency=skip` + `fixed-rate=100ms`:

```
scheduler: job "bad" failed after 10ms: boom          # Error level
scheduler: job "slow" skipped (previous run still in flight)   # Debug, reason string
```

There is no retry: a failed run is logged and the next fire is per schedule. Wrap your own
retry (or push the work into asynq) if you need one.

### 4.4 Drain on shutdown

Start a 3s job, `kill -TERM` mid-run: shutdown waits up to `drain-timeout`; set it to 1s to
watch `scheduler drain timed out: context deadline exceeded` (Warn).

### 4.5 Validation drills (fail fast)

```bash
# 1) job configured but never registered:
spring.scheduler.jobs.ghost.fixed-rate=1s            # → startup error naming "ghost"
# 2) two triggers on one job:
spring.scheduler.jobs.tick.fixed-rate=1s
spring.scheduler.jobs.tick.fixed-delay=1s            # → "sets more than one of ..."
# 3) unknown locker:
spring.scheduler.jobs.tick.lock=nope                 # → "references lock \"nope\" ..."
# 4) 6-field cron:
spring.scheduler.jobs.beat.cron=0 */5 * * * *        # → "must have 5 fields, got 6"
# 5) bean without entry (the silent direction):
scheduler.Provide("orphan", fn)  # no jobs.orphan.* → one Warn line, never runs
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Job never fires, only a Warn at startup | Bean registered but no `jobs.<name>.*` entry (the warning-only direction, starter.go:144-150) | Add the config entry; check name spelling both sides. |
| Startup error "configured but no Job bean" | Config entry with no `scheduler.Provide` | Register the bean or drop the entry. |
| "sets more than one of cron/fixed-rate/fixed-delay" / "must set exactly one" | Trigger exclusivity (job.go:93-110) | Keep exactly one trigger key per job. |
| "must have 5 fields" on cron | Seconds-style 6-field expression | Use 5-field (`* * * * *`). |
| "references lock %q but no lock.Locker bean" | `lock` value isn't a locker bean name | Match the starter-lock bean name exactly (starter.go:172-177). |
| Both replicas run the "locked" job | Locker is per-process (memory) or `lock-key` differs between deployments | Use a shared backend and the same key; key defaults to the job name. |
| Runs abandoned on deploy | `drain-timeout` shorter than the longest run | Raise it; remember there is no re-run of drained jobs. |
| Two jobs mysteriously serialize | Same locker AND same `lock-key` | Give each job its own key (default already does). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 11 (2 global + 9 per-job) |
| Required | 0 keys, but exactly one trigger per job is enforced |
| Quickstart external deps | 0 (a lock backend only for cross-replica dedup) |
| "Watch out" entries | 4 |

Design suspects (kept from the prior edition, plus new findings):

- Job ↔ config coupling by name across two sources is validated asymmetrically: config→bean
  is an error, bean→config only a warning — a typo on the bean side yields a silently dead job.
- ~~The observe seam is log-only~~ Fixed: each job run now opens a span on the GLOBAL otel
  pipeline (`scheduler.job <name>` via otel.Tracer, starter.go instrument) — no-op tracer
  unless starter-otel (or any SDK provider) is present; the log observer is unchanged and
  still the only signal for skipped fires. Runs emit span + existing log; no metrics yet.
- ~~The package doc comment's cron example is 6-field~~ Fixed: the example is now 5-field
  (`*/5 * * * *`), matching ParseCron (scheduling/cron.go:81-83).
- No `Init`-phase validation: all job validation happens in `Run/build()`; an error surfaces
  at Runner time (still pre-readiness, but later than the bind-time validation other starters
  do via `expr`).
- `drain-timeout` expiry abandons in-flight runs with no re-run or handoff — acceptable for
  idempotent jobs, undocumented as a policy for others.
