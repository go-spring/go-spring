# starter-lock-redis Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-redis` is a Contributor-archetype starter (`starter/DESIGN.md`
§2.3) in the integration layer. It contributes named `lock.Locker` beans
backed by Redis; it opens no listener and holds no connection of its own,
reusing a `*redis.Client` produced by `starter-go-redis`.

## 1. Responsibilities & Boundaries

- Binds `spring.lock.<name>` entries to Redis-backed `lock.Locker` beans,
  one per entry, registered under the config name and exported as
  `lock.Locker`.
- The `Locker` implements Redis SETNX + Lua-scripted release / renew and
  therefore does not require any additional library (no `redsync`).
- Explicitly does **not** own the Redis client's lifecycle: the destroy hook
  stops the background renewal goroutines it started; the `*redis.Client`
  itself is closed by `starter-go-redis`.

## 2. Key Abstractions & Seams

- **Seam is the bean type, not a driver string.** `stdlib/lock` deliberately
  has **no** package-level string driver registry, unlike
  `stdlib/discovery` or `stdlib/resilience`. Locks need a *live* backend
  handle (`*redis.Client`), not a declarative policy string; the switch from
  Redis to etcd/consul/k8s is therefore a blank-import swap that changes
  which starter registers the `lock.Locker` bean.
- **Instance-to-client wiring via `TagArg`.** The `Config.Client` field
  names the `*redis.Client` bean under `spring.go-redis.<Client>`. The
  starter wires the two by calling `gs.TagArg(c.Client)` on the provide
  builder — the seam that ties one `Locker` to a specific Redis instance.
- **Shared prefix across lock backends.** All lock starters bind under
  `spring.lock.<name>` (`starter/DESIGN.md` §3), so business code injects
  `lock.Locker` by name and never changes when the backend changes.

## 3. Constraints

- **`Client` is required.** An empty `spring.lock.<name>.client` is rejected
  at boot via `errutil.Explain`. Silently defaulting to some arbitrary
  Redis instance would hide a misconfiguration until the first `Acquire`,
  potentially in production.
- **Destroy stops renewal, not the client.** `destroyLocker` calls
  `Locker.Close()`, which stops the background renewal goroutine per held
  lock. It never calls `Close` on the injected `*redis.Client` — that
  connection may still be in use by other beans.
- **`Locker` API is per abstraction, not per backend.** Everything
  configurable (TTL, RenewInterval, RetryInterval, Token) flows through
  `lock.Option` values; the config only carries defaults. `KeyPrefix`
  scopes the key space so multiple apps can share one Redis cluster.

## 4. Trade-offs / Alternatives Rejected

- **`redsync` — rejected.** Hand-rolled `SET NX PX` + compare-and-DEL Lua is
  ~50 lines and keeps the dependency surface identical to `starter-go-redis`.
- **Auto-detect a `*redis.Client` bean — rejected.** Explicit `client=`
  keeps the mapping obvious; auto-detect would break as soon as an app
  ran more than one Redis instance.
- **Bundling Redis config into `spring.lock.<name>` — rejected.** Reusing
  the existing `*redis.Client` bean means switching topologies
  (share-a-cluster / dedicated-cluster) is a config-only change on the
  Redis side.
