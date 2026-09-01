# lock Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`lock` is the framework-agnostic abstraction for distributed locking and
leader election. Backends live in starters (`starter-lock-redis`,
`starter-lock-etcd`, `starter-lock-consul`, `starter-lock-k8s`); the bundled
`MemoryLocker` keeps stdlib self-testing without dependencies.

## 1. Responsibilities & Boundaries

- Answer "may I take this named lock right now?" with a handle that surfaces
  a fencing token and a `Lost()` channel.
- Provide a unified `Election` on top of any `Locker` so leader election is
  the same code across backends — deliberately not delegated to
  etcd/consul-native election, so the abstraction stays the single source of
  truth.
- Not a stampede-mitigation cache, not a queue, not a two-phase commit.
  `Unlock` is idempotent; contention is either signalled via `TryAcquire`
  returning `ok=false` or as an ordinary `Acquire` blocking retry.

## 2. Key Abstractions & Seams

- `Locker` = seam. Unlike `discovery` and `resilience` there is **no global
  string-keyed driver registry**, because a lock backend needs a live client
  (Redis conn, etcd client, ...) rather than a declarative policy. The seam
  is the `Locker` bean type: each starter builds its client and exports one
  `Locker`; switching backend = blank-import swap, no change to business code.
- `Options` (functional) normalizes TTL / RenewInterval / RetryInterval /
  Token. `RenewInterval` special values: `0` → TTL/3; negative → auto-renew
  disabled. Timing comes from three layers, resolved once by `Resolve` so every
  backend composes them identically:
  **per-acquisition `Option` > starter `Defaults` > package default.** Backends
  build a `Defaults` from their config and call `Resolve` (not `Apply`) at the
  top of `Acquire`/`TryAcquire`, so a starter-level `ttl` is an *overridable
  default* and a per-call `WithTTL` always wins. A backend that exposes no
  starter-level knobs passes a zero `Defaults` (pure per-call + package default,
  identical to `Apply`); this is the K8s backend's chosen shape.
- `Lock.Lost()` is the mandatory contract for lease-based backends; a critical
  section must select on it to abort when the lease is gone.
- `Election` is built on `Locker.Acquire` + `Lock.Lost()`: acquire the shared
  key, run `OnElected` with a term context, watch `Lost()`, cancel and
  recampaign.

## 3. Constraints (do not break)

- **`Unlock` is idempotent**. Only return `ErrNotHeld` when the backend can
  **prove** the lock was taken over by someone else; releasing an already-
  released or expired lock returns nil.
- **Fencing token** is a required non-empty string; `Apply` fills a random
  16-byte hex when unset. Downstream storage can use it to reject writes
  from a stale holder.
- **Lease lifecycle**: TTL is the max blast radius of a crashed holder;
  auto-renew keeps the lease alive; `Lost()` fires on renewal failure or lease
  expiry. Do not "keep the lock indefinitely" — pick TTL carefully.
- **`Election.OnElected` receives a term context** that is cancelled on loss;
  it must honour it. `Election.Run` cancels first, then waits for the leader
  goroutine, then unlocks — always in that order — before recampaigning.
- **`NewElection` panics on missing Locker/Key** because a misconfigured
  election can never elect anyone; fail-fast at construction.
- **Timing precedence is uniform across backends**: per-acquisition `Option` >
  starter `Defaults` > package default, resolved by `Resolve`. A backend must
  never apply a starter-level timing value as a fixed override that ignores a
  per-call `Option` — that breaks "switch backend, behaviour unchanged", the
  whole point of the seam. (`RenewInterval` keeps its sign semantics through the
  layering: a caller's explicit negative disables renew and is preserved.)

## 4. Trade-offs / Alternatives Rejected

- **No driver registry**. A live client cannot live in a package-global map
  across tests and restarts; that pattern is right for `resilience`
  (declarative) but wrong here.
- **No redsync / etcd-election / consul-native leader**. `Election` is one
  path over `Locker`, so the same behaviour is easy to reason about across
  backends and a K8s-Lease backend can be added later behind the same seam.
- **No re-entrant locks**. Every `Acquire` returns a new fencing token; a
  holder that recurses must build re-entrance on top (or not need it —
  distributed re-entrance is a footgun).
- **Kubernetes Lease backend exists** (`starter-lock-k8s`). It maps each lock
  to a `coordination.k8s.io/Lease` and needs no external middleware beyond the
  in-cluster control plane; by design it exposes no starter-level timing config,
  running pure per-call `Options` (a zero `Defaults`) so its knobs match the
  other backends.
- **The OTel tracing wrapper is duplicated per backend, not extracted into this
  package.** The shared instrumentation lives in-package: `WrapLocker` in
  observe.go wraps any `Locker` with the observe kit's three signals (trace
  span + duration/in-flight metric + access log, `lock.*` conventions), so
  each backend starter installs one wrapper with its `lock.system` value
  instead of copying an `observability.go` per backend (the copy era ended
  2026-08-31; it was what let the redis backend bind the wrong bean into its
  traced wrapper, fixed 2026-08-08).
