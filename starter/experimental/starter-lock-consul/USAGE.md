# starter-lock-consul Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`, `consullock.go`, `observe.go`), the shared
abstraction [cloud/lock](../../../cloud/lock), and the self-asserting
[example/](example) (`example/check.sh`). Consul's own semantics (sessions, KV, blocking queries)
are [Consul docs](https://developer.hashicorp.com/consul/docs/dynamic-app-config/sessions) —
everything below is go-spring's increment.

**Activation**: any `spring.lock.<name>.*` property registers one Consul-backed `lock.Locker`
instance per `<name>` (blank-import one lock backend per binary — the `spring.lock` prefix is
shared by all four backends).

---

## 1. Complete worked project

A two-lock service (scheduled job + singleton worker) with leader election and observability.
File tree:

```
demo/
├── go.mod
├── main.go
├── worker.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/hashicorp/consul/api  latest
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-lock-consul latest
    go-spring.org/starter-actuator    latest   // optional: probes
    go-spring.org/starter-otel        latest   // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-lock-consul"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**worker.go** — the application's entire lock surface:

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Worker struct {
    // The bean under the instance name is already observe-wrapped by default
    // (trace span + metric + access log); observe.enabled=false opts out (§6).
    Jobs      lock.Locker `autowire:"jobs"`
    Singleton lock.Locker `autowire:"singleton"`
}

func init() {
    gs.Provide(&Worker{}).Export(gs.As[gs.Rooter]())
}

func (w *Worker) Init(ctx context.Context) {
    // Elected leader runs the singleton loop; the other replicas wait.
    e := lock.NewElection(lock.ElectionConfig{
        Locker:    w.Singleton,
        Key:       "singleton-worker",
        OnElected: func(context.Context) { log.Infof(ctx, log.TagAppDef, "elected leader") },
    })
    go func() { _ = e.Run(ctx) }()
}

func (w *Worker) RunOnce(ctx context.Context) {
    lk, ok, err := w.Jobs.TryAcquire(ctx, "nightly-sync")
    if err != nil {
        return // backend failure — do not run the job
    }
    if !ok {
        return // another replica owns it — skip
    }
    defer lk.Unlock(ctx)
    select {
    case <-lk.Lost():
        return // session invalidated mid-run — abort
    case <-time.After(5 * time.Second):
    }
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- lock: scheduled jobs ----------------------------------------------------
spring.lock.jobs.address=127.0.0.1:8500
spring.lock.jobs.ttl=15s
# Session TTL must sit in Consul's [10s, 86400s] window; out-of-range values
# are clamped per acquisition, not rejected.
spring.lock.jobs.key-prefix=demo/jobs/

# --- lock: singleton worker --------------------------------------------------
spring.lock.singleton.address=127.0.0.1:8500
spring.lock.singleton.key-prefix=demo/singleton/

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317

# Access-log verbosity of the observe-lock adapter (defaults shown).
```

**Verify** (with a local Consul agent, e.g. `example/docker-compose.yml`):

```bash
go run .                          # boots, elects a leader
curl -s localhost:8500/v1/kv/demo/jobs/?keys   # lock keys appear while held
```

The runnable [example/](example) exercises TryAcquire/TryAcquire-contended/idempotent-Unlock/
re-acquire and exits 0; `example/check.sh` wraps it in docker compose.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-lock-consul
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") per entry <name>:
             ├─ fail fast when address == ""            (boot error naming the instance)
             ├─ Provide newLocker  → bean "<name>"          (Export lock.Locker,
             │                                                           Destroy → Close)
             └─ newLocker wraps it with observe-lock unless observe.enabled=false
  ├─ config bind: ${spring.lock.<name>} → Config (value tags)
  ├─ newConsulLocker: api.NewClient (+TLS when tls.enabled); TTL default recorded
  ├─ bean wiring: consumers' autowire:"<name>" resolved (bean already observe-wrapped)
  └─ on SIGTERM: Destroy per bean — consulLocker.Close is a contract no-op
                 (api.Client has no Close; held handles own their sessions)
```

Each acquisition builds a **fresh `*api.Lock`** with its own Consul session
(`consullock.go buildLock`), so cancelling one handle never affects another.

### 2.2 Three-layer timing resolution (all lock backends)

TTL / renew / retry resolve through `lock.Resolve` (cloud/lock/resolve.go), higher
layer wins:

| Layer | Source | This backend |
|-------|--------|--------------|
| 1. per-call option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | all effective |
| 2. starter default | `spring.lock.<name>.ttl` etc. | **TTL only** — consul auto-renews the session and blocks internally in its own acquire loop, so no renew/retry keys exist |
| 3. package default | TTL `30s`, renew `TTL/3`, retry `100ms` | fill whatever is still unset |

The resolved TTL is then clamped into Consul's `[10s, 86400s]` session window per acquisition
(`consullock.go buildLock`) — so even a per-call `WithTTL(5*time.Second)` is silently raised to
10s rather than rejected.

### 2.3 One lock, layer by layer (acquire → hold → release)

`TryAcquire(ctx, "nightly-sync")`:

1. `lock.Resolve(defaults, opts...)` — TTL from config unless `WithTTL` overrides; fencing token
   generated (random 16-byte hex) unless `WithToken`.
2. `buildLock`: key = `key-prefix + "nightly-sync"` (default prefix `lock/`); session TTL string
   from the clamped value; `LockTryOnce=true`, `LockWaitTime=500ms` (bounds the single-shot
   round-trip; small but non-zero so the agent can answer).
3. `al.Lock(stopCh)` — nil leaderCh ⇒ `ok=false, err=nil` (ordinary contention); non-nil error ⇒
   backend failure. `Acquire` instead omits `LockTryOnce` and blocks in Consul's blocking-query
   loop until held, ctx-done (translated through a ctx→stopCh watcher goroutine) or error.
4. Held: **Consul itself auto-renews the session** behind `api.Lock`; there is no client-side
   renew goroutine. If the session is invalidated (agent restart, expiry), the leaderCh closes —
   that *is* `Lost()`.
5. `Unlock`: `api.Lock.Unlock` (deletes the KV entry) then best-effort `Destroy` of the session.
   Idempotent; a second call returns nil. `api.ErrLockNotHeld` is treated as a benign
   "already released" — Consul cannot distinguish "we released" from "somebody else did", so this
   backend never returns `lock.ErrNotHeld` from Unlock.

---

## 3. Per-key behavior reference

All keys live under `spring.lock.<name>` (exact-match, no relaxed forms).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `address` | string | — | **Required.** Consul agent endpoint, e.g. `127.0.0.1:8500`. Checked before bean registration. | Empty → boot fails naming the instance (`lock-consul: spring.lock.<n>.address is required`). |
| `scheme` | string | `http` | URL scheme only. `tls.enabled=true` forces `https` unless scheme is explicitly non-http. | TLS enabled + scheme left `http` still works (auto-https); expecting plaintext with tls block → surprised by https. |
| `token` | string | — | Consul ACL token for the API client. Distinct from the per-acquisition fencing token. | Wrong token surfaces as 403 on first Acquire, not at boot (client creation does not authenticate). |
| `ttl` | duration | `30s` | Session TTL; feeds layer 2 of §2.2. Clamped into `[10s, 86400s]` per acquisition. | `5s` silently becomes `10s`; `100000h` becomes `24h` — no warning. |
| `key-prefix` | string | `lock/` | Prepended to every lock key. Keeps apps sharing one Consul cluster disjoint. | Two apps with the same prefix contend on each other's locks. |
| `tls.enabled` | bool | `false` | Applies the shared `tlsconf` block (`server-name`, `ca-file`, `cert-file`, `key-file`, `insecure-skip-verify`) to the API client. | Enabled without material → client-creation error at boot (fail fast). |
| `observe.enabled` | bool | `true` | Wrap the primary `<name>` Locker bean with the observe-lock adapter (trace span + metric + access log). `false` = bare locker. | Migration: the `<name>-observed` bean no longer exists — inject `<name>`. |

⚠ There is **no** `renew-interval`/`retry-interval` key: Consul auto-renews the session and blocks
internally (§2.2 layer 2). Instance weight (`Weight=0` drain) is a registry/loadbalance concept
and does not apply to lock backends.

---

## 4. Verification & fault drills

### 4.1 Contended lock (two terminals)

```bash
# terminal 1 — manual mode keeps the app alive
go run ./example -manual
# inside the app (or via a second replica): acquire demo/jobs/nightly-sync
# terminal 2 — same key from a second process
```

`TryAcquire` returns `ok=false, err=nil` while the other holder owns the key; the KV entry
`<key-prefix><key>` is visible in the Consul UI/API for exactly the holder's session:

```bash
curl -s 'localhost:8500/v1/kv/demo/jobs/nightly-sync?raw'   # the fencing token
```

### 4.2 TTL expiry mid-hold (crash drill)

1. Configure a short TTL: `spring.lock.jobs.ttl=10s` (the clamp floor).
2. Acquire, then `kill -9` the holder (no Unlock).
3. Consul invalidates the session after ~TTL; within a few seconds the KV entry is released and a
   waiting `Acquire` in the replica wins.
4. In the killed process's twin (before it died), `Lost()` fires via the leaderCh closing — a
   live critical section selects on it and aborts.

### 4.3 Observing the locker (on by default)

Inject `autowire:"jobs"` and generate lock traffic with starter-otel configured:

- Spans named `acquire` / `try_acquire` with attributes `lock.system="consul"`,
  `lock.operation`, and `lock.key` (detailed level). A lost contention carries
  `lock.acquired=false`.
- Metric histogram `lock.operation.duration` with the same attributes — check your collector
  (Jaeger UI) after traffic.

```bash
grep -r 'lock.operation.duration' <otel-export-dump>   # metric presence
```

Without starter-otel the wrapper is a silent near-no-op (global providers are no-ops).

### 4.4 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up consul, run self-asserting example
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `spring.lock.<n>.address is required` | instance without address | Set the key or drop the instance. |
| Boot fails at client creation / TLS | bad `tls.*` material or unreachable scheme | Fix certs; creation is fail-fast. |
| `Acquire` returns 403-ish ACL error | wrong/missing `token` | ACL is checked on first use, not at boot — set `spring.lock.<n>.token`. |
| Locks expire sooner/later than configured | TTL clamped into `[10s, 86400s]` | Pick a TTL inside the window; sub-10s is impossible on this backend. |
| `Unlock` never returns `ErrNotHeld` even after takeover | Consul cannot attribute the release; `api.ErrLockNotHeld` is swallowed as benign | Need proof-of-takeover semantics → use the redis backend (Lua compare-and-DEL). |
| No `<name>-observed` bean anymore | removed 2026-08 | Inject `<name>` — it is observed by default; `observe.enabled=false` gives the bare locker. |
| No spans/metrics despite default wrap | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| Two replicas both "hold" the lock | same `key-prefix`/key across apps, or session TTL clamped up after a crash | Distinct prefixes; account for the clamp when computing failover time. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (incl. tls/observer) | 14 |
| Required | 1 (`address`) |
| Quickstart external deps | 1 (Consul) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger):

- RESOLVED (2026-08): the primary `<name>` bean is now observed by default (transparent wrap
  in `newLocker`); the separate `<name>-observed` bean was removed — migration: inject `<name>`.
- RESOLVED: the wrap decision (`wrapIfObserved`, default on + opt-out) is covered by `observe_test.go`.
- TTL clamping is silent — a caller asking for `5s` gets `10s` with no log line; a clamp warning
  would surface mis-tuned failover budgets.
- Timing knobs are TTL-only by necessity, yet the key table reads differently from the redis
  backend — acceptable, documented per backend above.
