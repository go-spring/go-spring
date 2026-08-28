# starter-batch Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `job.go`, `launcher.go`, `config.go`) and the
self-asserting [example/](example/) (`example/check.sh` — docker Redis + two-phase
crash/resume smoke). **Batch semantics (chunk steps, restart checkpoints, job execution
model) live in [cloud/experimental/batch](../../../cloud/experimental/batch)** — only the
go-spring wiring is covered here.

**Activation**: blank import + at least one `JobDefinition` bean. Both starter beans gate on
`gs.OnBean[JobDefinition]()` (starter.go `init`), so importing the starter without jobs
costs nothing. `spring.batch.enabled=false` is an explicit opt-out.

---

## 1. Complete worked project

A chunk job that copies integers 1..10000 into a Redis set, with a durable (Redis)
repository, restart-on-crash semantics, and manual launch. File tree (this IS the example):

```
demo/
├── go.mod
├── main.go            // registers the JobDefinition + test runner
├── conf/app.properties
├── docker-compose.yml // Redis
└── check.sh           // two-phase crash/resume driver
```

**go.mod** (module deps that matter; siblings resolve through go.work):

```
require (
    github.com/redis/go-redis/v9 v9.21.0
    go-spring.org/cloud            v0.0.0
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-batch    latest
    go-spring.org/starter-batch-redis  latest   // durable JobRepository backend
    go-spring.org/starter-go-redis    latest   // *redis.Client bean
)
```

**main.go** (abridged from `example/example.go`, same structure):

```go
package main

import (
    _ "go-spring.org/starter-batch-redis"
    _ "go-spring.org/starter-go-redis"

    "go-spring.org/cloud/experimental/batch"
    "go-spring.org/spring/gs"
    StarterBatch "go-spring.org/starter-batch"
)

// reconcileJob is a JobDefinition whose writer needs a live *redis.Client,
// so it cannot use plain Provide(name, steps...) — it autowires the client
// and builds its ChunkStep in Build.
type reconcileJob struct {
    Client *redis.Client `autowire:"cache"`
}

func (j *reconcileJob) JobName() string { return "reconcile" }
func (j *reconcileJob) Build() (*batch.Job, error) {
    step := &batch.ChunkStep[int, int]{
        Name:      "load",
        Reader:    &seqReader{n: 10000},          // implements batch.Checkpointer
        Processor: batch.Passthrough[int](),
        Writer: batch.WriterFunc[int](func(ctx context.Context, items []int) error {
            return j.Client.SAdd(ctx, "demo:done", toAny(items)...).Err()
        }),
        ChunkSize: 100,
    }
    return &batch.Job{Name: "reconcile", Steps: []batch.Step{step}}, nil
}

// Runner injects the shared *Launcher — the same seam a scheduler.Job uses.
type Runner struct {
    Launcher *StarterBatch.Launcher `autowire:""`
    Client   *redis.Client          `autowire:"cache"`
}

func main() {
    // The Export is LOAD-BEARING: gs only wires root-reachable beans, and a
    // JobDefinition that nothing injects is collected via its export. Miss the
    // Export and the job silently never registers (see §2.1).
    gs.Provide(&reconcileJob{}).
        Name("reconcile").
        Export(gs.As[StarterBatch.JobDefinition]())
    gs.Provide(&Runner{}).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**conf/app.properties** — the complete, commented surface (from the example):

```properties
# A Redis client managed by starter-go-redis. Both the batch repository
# backend and the example's result set reuse this client by name.
spring.go-redis.cache.addr=127.0.0.1:6379

# A Redis-backed batch.JobRepository named "main", reusing the redis client
# above. This is what makes the step checkpoint durable across a crash.
spring.batch-repository.main.client=cache
spring.batch-repository.main.key-prefix=starter-batch:example:

# Tell the batch runner to use the "main" repository as its progress store.
spring.batch.repository=main
```

**Verify** (isomorphic to the example's check.sh):

```bash
docker compose up -d && sleep-for-6379
PHASE=1 go run . ; echo "rc=$?"     # crash run: exits non-zero after ~half committed
PHASE=2 go run .                    # resumes; prints "starter-batch smoke test passed"
docker exec starter-batch-redis redis-cli SCARD starter-batch:example:done   # 10000
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-batch
  ├─ init(): gs.Provide(&Launcher{}).Name("batchLauncher")
  │            .Init((*Launcher).Init)
  │            .Condition(enabled, gs.OnBean[JobDefinition]())
  ├─ init(): gs.Provide(&Server{}).Name("batchServer")
  │            .Condition(enabled, gs.OnBean[JobDefinition]())
  │            .Export(gs.As[gs.Server]())
gs.Run()
  ├─ bind: Launcher.Config ← ${spring.batch} (value tags)
  ├─ autowire: Launcher.Defs ← []JobDefinition ("?"), Launcher.Repos ←
  │     map[string]batch.JobRepository ("?")
  ├─ Launcher.Init: pickRepository (named > sole bean > memory fallback);
  │     dedupe by JobName (duplicate = boot error); Build() each once
  │     (fail-fast on builder bugs); validate every spring.batch.jobs.<name>
  │     references a real definition
  ├─ Server.Run: blocks on sig.TriggerAndWait() — startup launches fire only
  │     AFTER the app is ready
  ├─ per run-on-startup job: goroutine + wg.Add → Launcher.Launch (same call
  │     a scheduler would make)
  └─ on SIGTERM: Server.StopContext — cancel(runCtx), wait wg bounded by
        drain-timeout, then return
```

**The Export requirement** (memory: gs wires only root-reachable beans): a `JobDefinition`
registered with `gs.Provide` that neither injects into anything nor exports the
`JobDefinition` interface is never collected — `Launcher.Defs` stays empty and the job is
silently unlaunchable (`Launch` would report "unknown job"). This is why `Provide`/the
example always chain `.Name(...).Export(gs.As[JobDefinition]())`. Suspect #1 in §6.

### 2.2 One job launch, step by step

`Launcher.Launch(ctx, "reconcile", params)` (launcher.go):

1. Resolve `l.defs["reconcile"]` — unknown name is a clear error, not a no-op.
2. `def.Build()` — rebuilds the Job for THIS run (Init's earlier Build was validation
   only; per-run state injection is the reason Build is called again, per source comment).
3. `job.Run(ctx, l.repo, params)` — the batch engine loads the prior execution of the same
   (name, params) instance from the repository; an incomplete one resumes from the reader's
   checkpoint (`seqReader.Open(cp)`), a finished/new one starts fresh.
4. Each chunk: reader fills up to `ChunkSize` items → processor → writer commits →
   checkpoint persisted. Counts (`ReadCount`/`WriteCount`) accumulate across the restart
   because they are loaded from the repository — exact equality with `total` after a resume
   proves no committed chunk was reprocessed.
5. The returned `*batch.JobExecution` carries `Status` (assert `batch.StatusCompleted`)
   and `FailureMsg`.

### 2.3 Drain semantics on shutdown

`Server.StopContext` (starter.go): cancels the launch context (in-flight chunk steps see
ctx cancellation in their Reader/Processor/Writer — writers that respect ctx stop between
commits, so the checkpoint stays consistent), then waits on the WaitGroup with
`drain-timeout`. Timeout → a Warn log "drain timed out ... abandoning in-flight launches"
and Stop returns anyway. ⚠ Only **startup** launches are tracked: on-demand `Launch` calls
made by your own scheduler/handlers after `Run` returned are owned by their caller (source
comment on `Stop`/`StopContext`).

### 2.4 Why a gs.Server and not a Runner

Per the package doc: the starter is a global/infrastructure-archetype starter — no port.
Exporting a `gs.Server` buys participation in the server lifecycle: startup launches begin
only after the ready signal, and Stop joins the graceful-shutdown drain the orchestration
already expects.

---

## 3. Per-key behavior reference

Under `spring.batch`:

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.batch.enabled` | bool | true (MatchIfMissing) | Opt-out gate for both beans; combined with OnBean[JobDefinition]. | false → no Launcher/Server even with jobs registered; jobs stay launchable only if you construct them yourself (you can't — no Launcher bean). |
| `spring.batch.repository` | string | — | Names a `batch.JobRepository` bean. Resolution order: named > sole repo bean > `batch.NewMemoryRepository()` (in-process, NOT durable). | ⚠ Name that matches no bean = fail-fast boot error; ⚠ multiple repo beans + empty key = fail-fast "disambiguate" error. |
| `spring.batch.drain-timeout` | duration | 30s | Bounds Stop's wait for in-flight startup launches; `<=0` waits forever. | Too small → "drain timed out ... abandoning" and possibly mid-chunk abort (checkpoint still consistent); too large → slow shutdown. |
| `spring.batch.jobs.<name>.run-on-startup` | bool | false | Launch once after the ready signal (Cloud Task shape). ⚠ `<name>` must match a JobDefinition bean's `JobName()` — else fail-fast boot error. | Typo → boot fails with "no JobDefinition bean of that name". |
| `spring.batch.jobs.<name>.params.<k>` | string | — | Startup launch params. (name, params) identify the job INSTANCE in the repository — changing a param creates a new instance rather than resuming the old one. | Expecting resume after editing params → actually a fresh run. |

Reconciled against `grep -rhoE 'value:"[^"]+"'` — the five keys above plus the internal
`${spring.batch}` struct bind; nothing else. (The example's `spring.go-redis.*` /
`spring.batch-repository.*` keys belong to their own starters.)

---

## 4. Verification & fault drills

### 4.1 Job launches and completes (example check.sh, both phases)

```bash
cd starter/experimental/starter-batch/example
docker compose up -d && ./check.sh
# PHASE 1 crashes mid-run (non-zero), PHASE 2 prints:
#   "PHASE 2: found <n> items already committed from the crashed run; resuming"
#   "Completed: read=10000 write=10000 SCARD=10000 (no item reprocessed)"
#   "starter-batch smoke test passed"
```

### 4.2 Crash / restart drill (durable repository)

The core guarantee: `PHASE=1 go run .` exits 1 after ~5000 commits; `PHASE=2 go run .`
resumes the SAME (name, params) instance from the last committed checkpoint. Verify
exactness directly:

```bash
docker exec starter-batch-redis redis-cli SCARD starter-batch:example:done   # == 10000, not more
```

Repeat with the memory repository (remove `spring.batch.repository` and the
starter-batch-redis import): PHASE 2 restarts from zero — demonstrating why durability
requires a backend.

### 4.3 Drain drill

Set `spring.batch.jobs.reconcile.run-on-startup=true` + a long-running job + a short
`spring.batch.drain-timeout=2s`; send SIGTERM while the job is mid-run: within ~2s the log
shows `batch: drain timed out after 2s; abandoning in-flight launches` (tag `_app_batch`).

### 4.4 Log tag

All launch/finish/drain lines use the registered tag `_app_batch`:

```properties
logger.batch.type=Logger
logger.batch.level=WARN
logger.batch.tag=_app_batch
```

Boot also logs `batch runner started (N job definition(s), M run-on-startup)` — an easy
wiring sanity check.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| "no JobDefinition bean of that name is registered" at boot | `spring.batch.jobs.<name>` typo, or job bean missing/not exported | Fix the name; ensure `.Export(gs.As[JobDefinition]())` on the Provide. |
| "unknown job" from Launch at runtime, but you registered it | Job bean not exported → not collected (root-reachability) | Same as above — the Export is load-bearing (§2.1). |
| "duplicate JobDefinition bean named %q" | Two definitions share a JobName | JobName must be unique across beans. |
| "%d batch.JobRepository beans present but spring.batch.repository is empty" | Two repo backends imported, none named | Set `spring.batch.repository=<bean name>`. |
| Restart reprocesses everything / job restarts from zero | Memory repository fallback (boot logs "using in-process NewMemoryRepository") | Import a durable backend (starter-batch-redis) and name it. |
| "batch: build job %q" at boot | Your Build() returned an error (bad step wiring) | Init Builds every definition once to surface this early. |
| Shutdown hangs | `drain-timeout<=0` (wait forever) with a stuck launch | Set a positive drain-timeout; make writers respect ctx. |
| Startup launch never fires | Job registered but `run-on-startup` unset (false) | Set `spring.batch.jobs.<name>.run-on-startup=true`, or launch via the Launcher seam. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 |
| Required | 0 |
| Quickstart external deps | 0 (Redis only for durability) |
| "Watch out" entries | 6 |

Design suspects (kept from the previous edition; for the audit ledger):

1. `JobDefinition` must be exported as a bean by the caller when self-implemented — easy to
   forget the `Export(gs.As[JobDefinition]())`, leaving the job silently uncollected.
2. Startup launches launched in background goroutines are drained by `Stop`, but on-demand
   `Launch` calls after `Run` returns are untracked — lifetime ownership split is subtle.
3. (New) `Launcher.Init` Builds each definition purely for validation and discards the
   result — Build runs twice per job lifecycle by design; a definition with side effects in
   Build pays them at boot.
4. (New) The batch runner's drain and the scheduler's drain are separate WaitGroups — a
   scheduler-triggered batch run is drained by the scheduler, a startup run by the batch
   Server; total shutdown time is their max, not their sum, but the split is invisible to
   the operator.
