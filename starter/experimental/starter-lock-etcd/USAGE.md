# starter-lock-etcd Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`, `lock.go`, `observe.go`), the shared
abstraction [cloud/lock](../../../cloud/lock), and the self-asserting
[example/](example) (`example/check.sh`). etcd's own semantics (leases, concurrency package) are
[etcd docs](https://etcd.io/docs/latest/dev-guide/api_concurrency_reference/) — everything below
is go-spring's increment.

**Activation**: any `spring.lock.instances.<name>.*` property registers one etcd-backed `lock.Locker`
instance per `<name>` (blank-import one lock backend per binary — the `spring.lock` prefix is
shared by all four backends).

---

## 1. Complete worked project

A batch service holding a long-running lock with a startup readiness probe and observability.
File tree:

```
demo/
├── go.mod
├── main.go
├── batch.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go.etcd.io/etcd/client/v3     latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-lock-etcd latest
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
    _ "go-spring.org/starter-lock-etcd"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**batch.go** — the application's entire lock surface:

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Batch struct {
    // The bean under the instance name is already observe-wrapped by default
    // (trace span + metric + access log); observe.enabled=false opts out (§6).
    Locker lock.Locker `autowire:"main"`
}

func init() {
    gs.Provide(&Batch{}).Export(gs.As[gs.Rooter]())
}

func (b *Batch) Run(ctx context.Context) {
    lk, err := b.Locker.Acquire(ctx, "batch-run") // blocks while contended
    if err != nil {
        return
    }
    defer lk.Unlock(ctx)
    select {
    case <-lk.Lost(): // lease expired / connectivity gone — abort the batch
        log.Warnf(ctx, log.TagAppDef, "lock lost mid-run, aborting")
    case <-time.After(30 * time.Second):
        log.Infof(ctx, log.TagAppDef, "batch finished")
    }
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- etcd lock ----------------------------------------------------------------
spring.lock.instances.main.endpoints=127.0.0.1:2379
spring.lock.instances.main.ttl=10s
spring.lock.instances.main.key-prefix=/starter-lock-etcd/
# dial-timeout bounds both the initial connection and the startup readiness
# probe — an unreachable cluster fails boot instead of the first Acquire.
# spring.lock.instances.main.dial-timeout=5s

# --- observability (starter-otel) --------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317

# Access-log verbosity of the observe-lock adapter (defaults shown).
```

**Verify** (with a local etcd, e.g. `example/docker-compose.yml`):

```bash
go run .                          # boots; unreachable etcd aborts boot (probe)
ETCDCTL_API=3 etcdctl get --prefix /starter-lock-etcd/   # lock key while held
```

The runnable [example/](example) exercises TryAcquire(+WithTTL)/contention/idempotent-Unlock/
re-acquire and exits 0; `example/check.sh` wraps it in docker compose.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-lock-etcd
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") per entry <name>:
             ├─ fail fast when endpoints is empty        (boot error naming the instance)
             ├─ Provide newLocker  → bean "<name>"            (Export lock.Locker,
             │                                                           Destroy → Close)
             └─ newLocker wraps it with observe-lock unless observe.enabled=false
  ├─ newEtcdLocker:
  │    clientv3.New (DialTimeout, optional TLS via tlsconf.Build)
  │    + readiness probe: cli.Status(endpoints[0]) within DialTimeout —
  │      proves credentials/TLS work at boot; failure closes the client and aborts
  ├─ bean wiring: consumers' autowire:"<name>" resolved (bean already observe-wrapped)
  └─ on SIGTERM: Destroy → Close closes the shared *clientv3.Client.
                 Locks already handed out own their own sessions and stay valid
                 until Unlock — or their leases expire when the process dies.
```

Each acquisition opens a **fresh `concurrency.Session`** (its own lease with automatic keepalive,
`lock.go newMutex`), so one hold's lease and `Lost()` channel are isolated from any other
outstanding hold.

### 2.2 Three-layer timing resolution (all lock backends)

TTL / renew / retry resolve through `lock.Resolve` (cloud/lock/resolve.go), higher
layer wins:

| Layer | Source | This backend |
|-------|--------|--------------|
| 1. per-call option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | all effective |
| 2. starter default | `spring.lock.instances.<name>.ttl` etc. | **TTL only** — etcd's concurrency package keeps each session's lease alive itself, so no renew/retry keys exist |
| 3. package default | TTL `30s`, renew `TTL/3`, retry `100ms` | fill whatever is still unset |

The resolved TTL is converted to whole seconds, rounding sub-second values **up** with a 1s floor
(`lock.go ttlSeconds`), because etcd leases use integer TTLs.

### 2.3 One lock, layer by layer (acquire → hold → release)

`TryAcquire(ctx, "report")`:

1. `lock.Resolve(defaults, opts...)` — TTL from config unless `WithTTL` overrides (the example
   passes `WithTTL(10s)`); fencing token generated unless `WithToken`.
2. `newMutex`: `concurrency.NewSession(client, WithTTL(ttlSeconds(o.TTL)))` — the session's lease
   auto-keepalive runs inside client-go; then `concurrency.NewMutex(sess, keyPrefix+key)`.
3. `mu.TryLock(ctx)` — contention surfaces as the sentinel `concurrency.ErrLocked`, translated to
   `ok=false, err=nil`; other errors are backend failures. `Acquire` calls `mu.Lock(ctx)` and
   blocks until the mutex is held (etcd watch-driven) or ctx ends; the fresh session is closed on
   failure.
4. Held: etcd keeps the lease alive automatically (keepalive stream). The handle spawns one
   goroutine that fans `session.Done()` (lease expired / connectivity gone) and the voluntary
   `released` signal into the single channel returned by `Lost()` — it exits as soon as either
   fires, so no goroutine outlives the hold.
5. `Unlock`: best-effort `mutex.Unlock` + `session.Close` (closing the session releases the lease
   and fires `Lost()`). Idempotent. Unlock-after-lease-loss errors are swallowed except
   non-canceled backend errors, matching the abstraction's "already released = success" contract.
   `Key()` returns the caller-facing key without the prefix.

---

## 3. Per-key behavior reference

All keys live under `spring.lock.instances.<name>` (exact-match, no relaxed forms).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoints` | []string | — | **Required.** Cluster node addresses; checked before bean registration. | Empty → boot fails naming the instance. |
| `username` / `password` | string | — | etcd auth credentials; empty for anonymous clusters. | Wrong credentials fail the startup `Status` probe (not merely client creation). |
| `dial-timeout` | duration | `5s` | Bounds the initial connection **and** the readiness probe budget. | Too low → boot failures on slow networks; too high → slow fail-fast. |
| `ttl` | duration | `30s` | Lease TTL per lock; feeds layer 2 of §2.2. Sub-second values round **up** to 1s. | `500ms` silently becomes `1s`. |
| `key-prefix` | string | `/lock/` | Prepended to every lock key; trailing slashes preserved. | Shared prefix across apps → mutual contention. |
| `tls.enabled` | bool | `false` | Applies the shared `tlsconf` block (`server-name`, `ca-file`, `cert-file`, `key-file`, `insecure-skip-verify`) via `tlsconf.Build`. | Bad material → boot failure at client creation. |
| `observe.enabled` | bool | `true` | Wrap the primary `<name>` Locker bean with the observe-lock adapter (trace span + metric + access log). `false` = bare locker. | Migration: the `<name>-observed` bean no longer exists — inject `<name>`. |

⚠ There is **no** `renew-interval`/`retry-interval` key: etcd's concurrency package keeps each
session's lease alive automatically (§2.2 layer 2). Instance weight (`Weight=0` drain) is a
registry/loadbalance concept and does not apply to lock backends.

---

## 4. Verification & fault drills

### 4.1 Contended lock

Run two replicas against the same etcd. Replica A's `Acquire` returns immediately on a free key;
replica B's `Acquire` blocks; B's `TryAcquire` returns `ok=false, err=nil`:

```bash
ETCDCTL_API=3 etcdctl get --prefix /starter-lock-etcd/ --keys-only   # holder's key
```

The mutex key appears while held (etcd concurrency MVCC key) and disappears on Unlock.

### 4.2 TTL expiry mid-hold (crash drill)

1. Acquire with a short TTL: `lock.WithTTL(5 * time.Second)` (or set `spring.lock.instances.main.ttl=5s`).
2. `kill -9` the holder — no Unlock, no keepalive.
3. The lease expires after ~TTL; `session.Done()` closes in the dead process (moot) and the key
   vanishes — a waiting `Acquire` in replica B wins within the watch latency.
4. Drill the live path too: pause the holder (SIGSTOP) instead of killing — keepalive stops while
   the session object lives; after TTL the lease expires and `Lost()` fires in the paused process
   once it resumes, so a long critical section aborts instead of writing unguarded.

### 4.3 Startup readiness probe

Point `endpoints` at a dead port: boot fails with `lock-etcd: startup probe failed for ...` and
closes the client — a misconfigured app never reaches `Acquire`.

### 4.4 Observing the locker (on by default)

Inject `autowire:"main"` and generate lock traffic with starter-otel configured:

- Spans named `acquire` / `try_acquire` with attributes `lock.system="etcd"`, `lock.operation`,
  `lock.key` (detailed level); a lost contention carries `lock.acquired=false`.
- Metric histogram `lock.operation.duration` with the same attributes — check your collector
  after traffic.

Without starter-otel the wrapper is a silent near-no-op.

### 4.5 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up etcd, run self-asserting example
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `endpoints is required for instance "<n>"` | instance without endpoints | Set the key or drop the instance. |
| Boot fails `startup probe failed` | unreachable cluster, wrong credentials or TLS material | The `Status` call proves all three at boot — fix connectivity/auth. |
| Lock lost mid-run with healthy process | keepalive stream broken (network partition, etcd quorum loss) | etcd requires client→server traffic for keepalive; check connectivity and quorum, then shrink TTL to bound the exposure. |
| Sub-second TTL behaves as 1s | whole-second lease rounding (`ttlSeconds` rounds up, 1s floor) | Pick whole-second TTLs; `500ms` is not representable. |
| `TryAcquire` returns `ok=false` with no error | ordinary contention (`ErrLocked` translated) | Expected; use `Acquire` to wait. |
| No `<name>-observed` bean anymore | removed 2026-08 | Inject `<name>` — it is observed by default; `observe.enabled=false` gives the bare locker. |
| No spans/metrics despite default wrap | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| Lock survives process death longer than TTL | TTL configured larger than assumed, or rounding | Verify with `etcdctl lease list` / timing; remember `ttl` is lease TTL, not retry cadence. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (incl. tls/observer) | 15 |
| Required | 1 (`endpoints`) |
| Quickstart external deps | 1 (etcd) |
| "Watch out" entries | 3 |

Design suspects (for the audit ledger):

- RESOLVED (2026-08): the primary `<name>` bean is now observed by default (transparent wrap
  in `newLocker`); the separate `<name>-observed` bean was removed — migration: inject `<name>`.
- RESOLVED: the wrap decision (`wrapIfObserved`, default on + opt-out) is covered by `observe_test.go`.
- No native etcd fencing token — `Token()` reflects the lock-package token, which downstream
  resources cannot verify against etcd state (documented limitation, worth a note in any fencing
  design).
