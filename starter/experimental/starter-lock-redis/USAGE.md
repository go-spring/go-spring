# starter-lock-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`, `redislock.go`, `observe.go`), the shared
abstraction [cloud/lock](../../../cloud/lock), and the self-asserting
[example/](example) (`example/check.sh`). Redis semantics (SET NX PX, scripting, expiry) are
[Redis docs](https://redis.io/docs/latest/commands/set/) — everything below is go-spring's
increment.

**Activation**: any `spring.lock.<name>.*` property registers one Redis-backed `lock.Locker`
instance per `<name>`; each reuses the `*redis.Client` bean named by its `client` field
(provided by starter-go-redis under `spring.go-redis.<client>`). Blank-import one lock backend
per binary — the `spring.lock` prefix is shared by all four backends.

---

## 1. Complete worked project

A scheduled-job service locking over the app's existing Redis, with observability. File tree:

```
demo/
├── go.mod
├── main.go
├── jobs.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/redis/go-redis/v9  latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-go-redis latest
    go-spring.org/starter-lock-redis latest
    go-spring.org/starter-actuator latest   // optional: probes
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**jobs.go** — the application's entire lock surface:

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Jobs struct {
    // The bean under the instance name is already observe-wrapped by default
    // (trace span + metric + access log); observe.enabled=false opts out (§6).
    Lock lock.Locker `autowire:"jobs"`
}

func init() {
    gs.Provide(&Jobs{}).Export(gs.As[gs.Rooter]())
}

func (j *Jobs) Run(ctx context.Context) {
    // Acquire blocks while contended, retrying every retry-interval.
    lk, err := j.Lock.Acquire(ctx, "nightly-sync", lock.WithTTL(10*time.Second))
    if err != nil {
        return
    }
    defer lk.Unlock(ctx)
    select {
    case <-lk.Lost(): // renew proved takeover — abort the job
        log.Warnf(ctx, log.TagAppDef, "lock lost mid-run, aborting")
    case <-time.After(3 * time.Second):
        log.Infof(ctx, log.TagAppDef, "job finished")
    }
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- redis client (owned by starter-go-redis; the lock reuses it by name) ----
spring.go-redis.cache.addr=127.0.0.1:6379

# --- locker -------------------------------------------------------------------
spring.lock.jobs.client=cache          # required; fail-fast when empty
spring.lock.jobs.ttl=10s               # default lease TTL (per-call WithTTL wins)
spring.lock.jobs.renew-interval=0      # 0 -> ttl/3; negative disables auto-renew
spring.lock.jobs.retry-interval=100ms  # Acquire poll interval under contention
spring.lock.jobs.key-prefix=starter-lock-redis:example:

# --- observability (starter-otel) --------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317

# Access-log verbosity of the observe-lock adapter (defaults shown).
```

**Verify** (with a local Redis, e.g. `example/docker-compose.yml`):

```bash
go run .                                   # boots and runs the self-asserting flow
redis-cli keys 'starter-lock-redis:example:*'   # lock key visible while held
redis-cli ttl 'starter-lock-redis:example:demo' # ~TTL, refreshed by the renew loop
```

The runnable [example/](example) exercises TryAcquire/contention/idempotent-Unlock/re-acquire/
blocking Acquire and exits 0; `example/check.sh` wraps it in docker compose.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-go-redis + starter-lock-redis
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") per entry <name>:
             ├─ fail fast when client == ""       (boot error naming the instance)
             ├─ Provide newLocker → bean "<name>"           (Export lock.Locker,
             │              gs.ValueArg(c), gs.TagArg(c.Client)    Destroy → Close)
             └─ newLocker wraps it with observe-lock unless observe.enabled=false
gs.Run()
  ├─ config bind: ${spring.lock.<name>} → Config (value tags)
  ├─ newRedisLocker: no dialing — the *redis.Client bean is injected ready-made;
  │  this starter owns no connection
  ├─ bean wiring: consumers' autowire:"<name>" resolved (bean already observe-wrapped)
  └─ on SIGTERM: Destroy → Close closes the stop channel and WAITS for all renew
                 goroutines (wg.Wait). The *redis.Client lifecycle belongs to
                 starter-go-redis and is not touched.
```

The historical copy-paste bug (the observed bean bound the `*redis.Client` instead of the
Locker) is gone with the redesign: the whole second Provide was removed and observation is a
transparent default inside `newLocker`.

### 2.2 Three-layer timing resolution (all lock backends)

TTL / renew / retry resolve through `lock.Resolve` (cloud/lock/resolve.go), higher
layer wins — this starter feeds **all three** knobs:

| Layer | Source | Keys |
|-------|--------|------|
| 1. per-call option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | — |
| 2. starter default | `spring.lock.<name>.ttl` / `.renew-interval` / `.retry-interval` | all three exist here |
| 3. package default | TTL `30s`, renew `TTL/3`, retry `100ms` | fill whatever is still unset |

Special semantics that survive the layering:

- `renew-interval = 0` (the starter default) means "no opinion" → falls through to TTL/3.
- A **negative** `renew-interval` (per call or starter) is non-zero and therefore preserved: it
  **disables auto-renew** — the lock expires strictly after TTL regardless of work duration.
- Instance weight (`Weight=0` drain) is a registry/loadbalance concept and does not apply to lock
  backends.

### 2.3 One lock, layer by layer (acquire → renew → release)

`Acquire(ctx, "nightly-sync", WithTTL(10s))`:

1. `lock.Resolve(defaults, opts...)` — TTL 10s (per-call wins over config), renew = TTL/3 unless
   configured, retry = `retry-interval`; fencing token generated (random 16-byte hex) unless
   `WithToken`.
2. `TryAcquire`: one `SET key token NX PX <ttl>` (`redislock.go TryAcquire`).
   - Not set (`ok=false`) ⇒ contention.
   - Set ⇒ handle created; if renew interval > 0 a **renewLoop goroutine** starts.
   - In `Acquire`, contention sleeps `RetryInterval` and retries until ctx ends / locker closes.
3. Held / renew: every interval the renew Lua runs — `PEXPIRE` only when the value still equals
   the caller's token (compare-and-PEXPIRE). A transient Redis error is skipped and retried next
   tick (Redis remains the source of truth for expiry); a `0` result means the key is gone or
   re-owned ⇒ the handle's `lost` channel closes — `Lost()` fires and the critical section must
   abort.
4. `Unlock`: stops the renew loop first (no fresh PEXPIRE races the DEL), then runs the unlock
   Lua — a three-way compare-and-DEL returning `1` (deleted), `0` (already gone — no-op) or
   `-1` (owned by another token). `-1` maps to `lock.ErrNotHeld` — this is the only backend of
   the four that can *prove* takeover at Unlock. Idempotent; on a backend error the handle still
   fires `Lost()` so consumers unblock.
5. Single-node Redlock only: no multi-node quorum. For stronger guarantees use the etcd/consul
   backend (blank-import swap).

---

## 3. Per-key behavior reference

All keys live under `spring.lock.<name>` (exact-match, no relaxed forms).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `client` | string | — | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.<client>`; checked before bean registration. `TagArg(c.Client)` is the seam tying locker to redis instance. | Empty → boot fails naming the instance; typo → bean-wiring failure at boot. |
| `ttl` | duration | `30s` | Default lease TTL for acquisitions that pass no `WithTTL`; layer 2 of §2.2. | Too low + GC pause/network blip ⇒ lock lost mid-work; too high ⇒ slow failover after a crash. |
| `renew-interval` | duration | `0` | Lease refresh interval. `0` → `ttl/3`; **negative disables auto-renew** (expires strictly after TTL). | Disabling renew with a short TTL drops locks mid-work by design. |
| `retry-interval` | duration | `100ms` | `Acquire`'s poll interval while contended. | Too low hammers Redis; too high adds latency to failover. |
| `key-prefix` | string | — | Prepended to every key before it hits Redis; keeps key spaces of apps sharing one Redis disjoint. | Shared prefix across apps → mutual contention. |
| `observe.enabled` | bool | `true` | Wrap the primary `<name>` Locker bean with the observe-lock adapter (trace span + metric + access log). `false` = bare locker. | Migration: the `<name>-observed` bean no longer exists — inject `<name>`. |

⚠ This starter exposes all three timing knobs, unlike consul/etcd (TTL only) and k8s (none) —
Redis has no server-side session management, so renew/retry live client-side here.

---

## 4. Verification & fault drills

### 4.1 Contended lock

Two terminals against the same Redis; terminal 1's app holds
`starter-lock-redis:example:demo`:

```bash
redis-cli get 'starter-lock-redis:example:demo'   # the fencing token (random hex)
```

Terminal 2's `TryAcquire` returns `ok=false, err=nil`; its `Acquire` returns ~at terminal 1's
Unlock (poll every `retry-interval`).

### 4.2 TTL expiry mid-hold (renew-disabled drill)

1. Configure `spring.lock.jobs.ttl=3s` and `spring.lock.jobs.renew-interval=-1` (auto-renew off).
2. Acquire and hold; watch the key expire without any client action:

```bash
watch -n1 redis-cli ttl 'starter-lock-redis:example:demo'   # 3..2..1 → key gone
```

3. Another replica's `Acquire` wins immediately after expiry. In the original holder a
   `select { case <-lk.Lost(): }` fires only on its next renew attempt (disabled here) — which
   is exactly why long work needs renew enabled.

### 4.3 Renew loop (auto-renew drill)

With defaults (`renew-interval=0` → ttl/3), hold a lock with `ttl=10s` and watch the TTL being
refreshed every ~3.3s:

```bash
watch -n1 redis-cli ttl 'starter-lock-redis:example:demo'   # oscillates ~10 → ~7 → 10 ...
```

Kill the renew path by re-owning the key out of band (`redis-cli set <key> oops PX 99999`): the
next renew returns `0`, `Lost()` fires, and a later `Unlock` returns `ErrNotHeld` (script `-1`)
— the takeover proof unique to this backend.

### 4.4 Observing the locker (on by default)

Inject `autowire:"jobs"` and generate lock traffic with starter-otel configured:

- Spans named `acquire` / `try_acquire` with attributes `lock.system="redis"`,
  `lock.operation`, `lock.key` (detailed level); a lost contention carries
  `lock.acquired=false`.
- Metric histogram `lock.operation.duration` with the same attributes — check your collector
  after traffic.

Without starter-otel the wrapper is a silent near-no-op.

### 4.5 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up redis, run self-asserting example
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `lock-redis: instance "<n>" missing required property ...client` | instance without `client` | Set `spring.lock.<n>.client` to an existing `spring.go-redis.<name>`. |
| Boot fails wiring `*redis.Client` | `client` typo — no such redis bean | Fix the name to match a `spring.go-redis.<client>` entry. |
| Locks lost mid-work | renew disabled (`renew-interval < 0`) or TTL shorter than worst-case pause | Enable renew; raise TTL — renew only fires every interval, so TTL must survive one missed tick. |
| Failover slow after a crash | TTL (or renew interval × slack) too large | Lower TTL; crash failover waits out the remaining TTL. |
| `Unlock` returns `ErrNotHeld` | the key expired or was taken over before Unlock | Expected proof of takeover — check whether the critical section overran its lease. |
| No `<name>-observed` bean anymore | removed 2026-08 | Inject `<name>` — it is observed by default; `observe.enabled=false` gives the bare locker. |
| No spans/metrics despite default wrap | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| Redis restart loses all locks | keys are in-memory; single-node Redlock | By design; use etcd/consul backend for stronger durability (blank-import swap). |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (incl. observer) | 9 |
| Required | 1 (`client`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger):

- RESOLVED (2026-08): the primary `<name>` bean is now observed by default (transparent wrap
  in `newLocker`); the separate `<name>-observed` bean was removed — migration: inject `<name>`.
- RESOLVED: the wrap decision (`wrapIfObserved`, default on + opt-out) is covered by `observe_test.go`.
- Three timing knobs here vs zero/one on the other backends — justified by redis's lack of
  server-side session management, but worth a second look at whether consul/etcd could expose
  `retry-interval` too.
