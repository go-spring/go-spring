# starter-batch-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim is verified against
the starter source (`starter.go`, `config.go`, `redisrepo.go`) and the runnable smoke example
[starter-batch/example](../starter-batch/example) (`example/check.sh`, docker-gated). Batch
semantics themselves (JobExecution / StepExecution, chunk model, restart-from-last-chunk) belong to
`go-spring.org/cloud/experimental/batch` — this document covers the Redis backend and the
go-spring wiring only.

**Activation**: any `spring.batch-repository.instances.*` key. Blank-importing the package registers one
`batch.JobRepository` per entry (`starter.go:51-70`); each entry reuses a `*redis.Client` bean
published by starter-go-redis. The starter holds no connection of its own — it is a Contributor
archetype (starter.go:23-28): swapping Redis for a SQL backend is a blank-import change.

---

## 1. Complete worked project

A chunk-oriented reconciliation job that copies 10 000 rows into Redis with a durable checkpoint,
plus a progress-query service. This is the shape the smoke example runs; the crash/resume drill in
§4 reuses it verbatim. File tree:

```
demo/
├── go.mod
├── main.go
├── job.go
├── conf/
│   └── app.properties
└── docker-compose.yml     # redis on 127.0.0.1:6379
```

**go.mod** (deps that matter):

```
require (
    github.com/redis/go-redis/v9   latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-batch    latest
    go-spring.org/starter-batch-redis latest
    go-spring.org/starter-go-redis latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-batch"
    _ "go-spring.org/starter-batch-redis"   // contributes the Redis JobRepository
    _ "go-spring.org/starter-go-redis"      // contributes *redis.Client beans
)

func main() { gs.Run() }
```

**job.go** — one JobDefinition plus a progress-query bean:

```go
package main

import (
    "context"

    "github.com/redis/go-redis/v9"
    "go-spring.org/cloud/experimental/batch"
    "go-spring.org/spring/gs"
    StarterBatch "go-spring.org/starter-batch"
)

// reconcileJob needs a live client, so it cannot be a plain
// batch.Provide(name, steps...) — it autowires the client and builds its
// ChunkStep in Build (same shape as the smoke example's job).
type reconcileJob struct {
    Client *redis.Client `autowire:"cache"`
}

func (j *reconcileJob) JobName() string { return "reconcile" }

func (j *reconcileJob) Build() (*batch.Job, error) {
    step := &batch.ChunkStep[int, int]{
        Name:      "load",
        Reader:    &seqReader{n: 10000},          // implements batch.Checkpointer
        Processor: batch.Passthrough[int](),
        Writer:    batch.WriterFunc[int](j.writeChunk),
        ChunkSize: 100,
    }
    return &batch.Job{Name: "reconcile", Steps: []batch.Step{step}}, nil
}

// Progress exposes the same repository the runner uses, for a /jobs endpoint.
type Progress struct {
    Repo batch.JobRepository `autowire:"main"`
}

func init() {
    gs.Provide(&reconcileJob{}).
        Name("reconcile").
        Export(gs.As[StarterBatch.JobDefinition]())
    gs.Provide(&Progress{}) // inject where you need status queries
}
```

(seqReader and writeChunk are application code — see the smoke example for a complete
checkpoint-carrying reader: `../starter-batch/example/example.go:117-145`.)

**conf/app.properties** — the complete surface:

```properties
# --- redis client (starter-go-redis namespace) --------------------------------
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# --- this starter: one repository per spring.batch-repository.instances.<name> entry ----
spring.batch-repository.instances.main.client=cache
spring.batch-repository.instances.main.key-prefix=demo:batch:
spring.batch-repository.instances.main.ttl=24h

# --- batch runner (starter-batch namespace) — picks the repo up by name -------
spring.batch.repository=main
spring.batch.drain-timeout=30s
spring.batch.jobs.reconcile.run-on-startup=true
spring.batch.jobs.reconcile.params.date=2026-08-28
```

**Verify**:

```bash
docker compose up -d
go run .
# after "batch runner started (1 job definition(s), 1 run-on-startup)":
grep -c 'job "reconcile" finished' <log>      # 1 line, status=COMPLETED
redis-cli --scan --pattern 'demo:batch:*'     # job:<sha1>, steps:<id>, seq keys
```

External dependency: one Redis (the only one). No registry, no database.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-go-redis + starter-batch + starter-batch-redis
  ├─ go-redis: gs.Group("${spring.go-redis}") → *redis.Client bean "cache"
  ├─ batch-redis: gs.Module(OnProperty("spring.batch-repository"), BindEach)
  │     └─ per entry <name>: Provide(newRedisRepository, ValueArg(c), TagArg(<client>))
  │                        .Name(<name>).Export(batch.JobRepository)   starter.go:64-67
  └─ batch: Launcher + Server beans, gated OnBean[JobDefinition]()

gs.Run()
  ├─ config bind: ${spring.batch-repository.instances.<name>} → Config (client / key-prefix / ttl)
  ├─ fail-fast #1: client=="" → boot error "instance %q missing required property"
  │                (starter.go:57-60 — no silent default client)
  ├─ wiring: TagArg(c.Client) injects the named *redis.Client (the seam that ties the
  │           repository to a redis instance; starter.go:62-64)
  ├─ Launcher.Init: pickRepository resolves spring.batch.repository (launcher.go:164-185);
  │           unknown name → fail-fast; 0 repos → memory fallback (logged);
  │           >1 repo and no name → fail-fast "name one explicitly"
  ├─ Server.Run: waits for ready signal, fires run-on-startup jobs in background
  │           goroutines (starter.go(starter-batch):123-161)
  └─ SIGTERM: Stop drains in-flight launches up to spring.batch.drain-timeout
```

Timing note for the restart guarantee: the repository writes are synchronous Redis round-trips
inside the chunk commit path — a committed chunk is durable *before* the engine reads the next one,
which is what makes crash-resume exact (see §2.2).

### 2.2 One launch, layer by layer (and a crash)

`Launcher.Launch(ctx, "reconcile", params)` — the same call the runner uses for run-on-startup and
a scheduler job would use (`launcher.go:44-46`):

1. `ObtainExecution(name, params)` — Redis `GET job:<instanceKey>` where instanceKey =
   `SHA1(name \0 k=v \0 ...)` over sorted params (`redisrepo.go:95-111`, same algorithm as the
   in-memory backend so both agree on "same instance").
2. Stored execution exists and `Status != COMPLETED` → returned with restart=true: the engine
   resumes it (`redisrepo.go:138-146`). COMPLETED or missing → `INCR seq` mints a fresh
   monotonically-unique ID `<sha8>-<n>`, SET writes the new JobExecution, restart=false
   (`redisrepo.go:148-166`).
3. Per step: the reader re-opens at its saved checkpoint; each chunk commit lands as one
   `HSET steps:<jobExecutionID> <stepName> <JSON StepExecution>` — atomic and idempotent per
   step name, TTL refreshed on every save (`redisrepo.go:197-210`).
4. **Crash here** (`kill -9` / os.Exit): committed chunks + step checkpoint survive in Redis.
5. Next `Launch` of the same (name, params) → step 2 takes the restart branch, the reader opens
   past the checkpoint, no committed chunk is reprocessed (proven by the smoke example:
   ReadCount == WriteCount == total after resume).
6. Status transitions each rewrite `job:<instanceKey>` via the single JSON+SET+EXPIRE path
   (`redisrepo.go:181-191`), so a crash between transitions leaves a coherent snapshot.

### 2.3 Key layout in Redis

With `key-prefix=P` (`redisrepo.go:41-46`):

| Key | Type | Content |
|-----|------|---------|
| `Pjob:<instanceKey>` | string | JSON JobExecution (status, times, params) |
| `Psteps:<jobExecutionID>` | hash | field per stepName → JSON StepExecution (counts + checkpoint envelope) |
| `Pseq` | counter | INCR source of execution IDs |

All three get `EXPIRE ttl` on every write when `ttl > 0`. The steps of one execution share a
single hash deliberately: Save/Find/List are one round-trip each (`redisrepo.go:73-79`).

---

## 3. Per-key behavior reference

### 3.1 This starter — `spring.batch-repository.instances.<name>.*` (3 keys)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `<name>.client` | string | — | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.instances.<client>`; injected by `gs.TagArg` (starter.go:64). ⚠ coupled: the go-redis instance must exist under exactly this name. | Empty → boot fails "missing required property" (fail-fast, starter.go:57-60); wrong name → container error on bean lookup. |
| `<name>.key-prefix` | string | "" | Prepended to `job:`/`steps:`/`seq` keys. Use to keep multiple apps sharing one Redis disjoint. ⚠ changing it orphans prior history — restart-resume then starts fresh. | Colliding prefixes between apps → cross-app resume corruption. |
| `<name>.ttl` | duration | 0 | `>0` → EXPIRE applied and **refreshed** on every write, so long-running steps stay alive (`redisrepo.go:113-121`). 0 keeps records forever (open-ended restart windows). | Too small → records expire mid-run: a restart after expiry reprocesses from scratch. |

### 3.2 Load-bearing keys owned by the runner (`spring.batch.*`, starter-batch)

| Key | Default | Behavior | Misconfiguration consequence |
|-----|---------|----------|------------------------------|
| `spring.batch.repository` | "" | Names this repository for the runner. Empty + exactly one repo bean → implicit pick; empty + several → fail-fast (`launcher.go:164-185`). | Typo → boot fails "no batch.JobRepository bean of that name". |
| `spring.batch.drain-timeout` | 30s | SIGTERM drain bound for startup launches. | 0 waits forever; too small abandons launches mid-chunk (resume still safe). |
| `spring.batch.jobs.<job>.run-on-startup` | false | Launch `<job>` once at readiness. | Job configured but no JobDefinition bean → boot fails (`launcher.go:97-103`). |
| `spring.batch.jobs.<job>.params.*` | — | Instance identity: changing a param **creates a new instance**, not a restart of the old one (`config.go(starter-batch):58-61`). | Retrying with a new date → old incomplete instance stays incomplete forever (with ttl=0, also leaks keys). |

⚠ Namespace split, by design: repositories bind under `spring.batch-repository.instances.<name>` because the
runner owns `spring.batch.*` for job/step/chunk config (`config.go:27-31`). `spring.batch.repository`
(singular, runner-side) references `spring.batch-repository.instances.<name>` (plural, this starter).

---

## 4. Verification & fault drills

All commands assume the worked project from §1 and a `redis-cli` pointed at the same Redis.

### 4.1 Keys and contents after a completed run

```bash
redis-cli --scan --pattern 'demo:batch:*'
# demo:batch:job:<40-hex>       → json .status == "COMPLETED"
redis-cli --type demo:batch:steps:<id>     # hash
redis-cli hget demo:batch:steps:<id> load  # json .readCount/.writeCount
redis-cli get demo:batch:seq               # monotonically growing
```

### 4.2 Crash-resume drill (the starter's core guarantee)

This is exactly what `starter-batch/example/check.sh:53-86` automates:

```bash
PHASE=1 go run .   # example's writer os.Exit(1)s after ~half committed; expect non-zero exit
redis-cli scard demo:done                  # ~half of the items
PHASE=2 go run .   # fresh process, same (name, params) → resumes; prints
                  # "Completed: read=10000 write=10000 SCARD=10000 (no item reprocessed)"
```

Or simply `cd ../starter-batch/example && ./check.sh` (docker-gated; skips gracefully without
docker).

### 4.3 TTL drill

Set `ttl=5s` and a long job: watch keys' TTL refresh while the job runs
(`redis-cli ttl demo:batch:job:<key>` → keeps resetting), then expire after completion.

### 4.4 Instance-identity drill

Run once with `params.date=2026-08-28`, then again with `params.date=2026-08-29`: `INCR seq`
increments — a *new* JobExecution, no resume. Re-run with the same date while the first is
incomplete → same `job:<instanceKey>`, restart branch taken.

### 4.5 Observability

The starter logs repository creation at Debug under tag `_app_def`
(`starter.go:61`): `creating batch redis repository name=… client=… keyPrefix=…`. Run/commit
logging (`batch:`, `_app_batch`) and the repository's health/metrics come from starter-batch and
starter-go-redis respectively — this starter contributes neither an indicator nor metrics.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "instance %q missing required property %q" | `spring.batch-repository.instances.<name>.client` empty | Set it; there is deliberately no default client (starter.go:53-60). |
| Boot fails "no batch.JobRepository bean of that name" | `spring.batch.repository` typo'd or the batch-repository entry didn't register | Check the OnProperty activation key spelling (`spring.batch-repository`, hyphenated). |
| Boot fails "N batch.JobRepository beans present but spring.batch.repository is empty" | several repository starters imported, none named | Name one via `spring.batch.repository` (launcher.go:183-185). |
| Restart reprocesses everything | key-prefix changed, ttl expired the records, or params changed (new instance) | Keep prefix/params stable; ttl=0 for open-ended restart windows. |
| Restart "resumes" a job that should have started fresh | prior run of the same (name, params) never reached COMPLETED | Change a param to mint a new instance, or delete `job:<instanceKey>`. |
| `decode job/step ...` errors at runtime | the JSON at `job:`/`steps:` was written by an incompatible schema version | Clear the affected prefix before upgrading across incompatible releases. |
| Steps hash grows without bound | ttl=0 (default) keeps every execution forever | Set a ttl to garbage-collect finished runs (`config.go:45-50`). |
| Slow chunk commits | one SET + one EXPIRE + one HSET round-trip per commit; latency-bound | Co-locate with Redis / raise ChunkSize (fewer commits per item); no pipelining today. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 3 per instance (+5 runner keys load-bearing here) |
| Required | 1 (`client`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger; entries 1-2 carried over from the previous audit):

1. No example of its own — correctness is exercised through starter-batch's example, whose
   check.sh runs the example twice against the same Redis to assert restart-resume.
2. Experimental: lives under `experimental/` (unreviewed marker, not a quality grade).
3. Reads are not pipelined: one commit = 3 sequential round-trips (SET, EXPIRE, HSET) — a
   pipeline/Lua-script commit would cut latency but complicate the code; nothing today needs it.
4. `seq` counter is per-prefix and never reset; IDs leak the total run count of the namespace.
5. TTL is instance-wide, not per-lifecycle: an EXPIRE applies equally to in-flight and completed
   records, relying on refresh-on-write to protect long steps.
