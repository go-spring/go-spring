# starter-lock-etcd Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-etcd` is a Contributor-archetype starter (`starter/DESIGN.md`
§2.3) in the integration layer. It contributes named `lock.Locker` beans
backed by etcd concurrency sessions.

## 1. Responsibilities & Boundaries

- Binds `spring.lock.<name>` entries to etcd-backed `lock.Locker` beans,
  one per entry, registered under the config name and exported as
  `lock.Locker`.
- The locker uses `concurrency.NewSession(WithTTL)` + `concurrency.NewMutex`;
  keepalive is handled by the session, so no manual renew goroutine is
  needed.
- Optionally configures TLS to the etcd cluster via the shared
  `starter.TLSConfig` (`Enabled`, `CertFile`, `KeyFile`, `CAFile`,
  `ServerName`, `InsecureSkipVerify`), off by default.

## 2. Key Abstractions & Seams

- **Seam is the bean type.** Like `starter-lock-redis`, there is no
  package-level driver string; switching backend is a blank-import change.
- **Session-per-acquisition.** Each `Acquire` opens a fresh
  `concurrency.Session` so its `Done()` channel maps 1:1 to the acquired
  lock's `Lost()` — different holds are independent.
- **`ttlSeconds` normalization.** etcd rejects sub-second session TTLs; the
  config helper rounds up to whole seconds with a minimum of one to keep
  the abstraction's TTL contract intact.

## 3. Constraints

- **`Endpoints` is required.** An empty list is rejected at boot; there is
  no localhost fallback so a misconfigured cluster address never boots
  silently.
- **Session keepalive handles renewal.** The lock does not run a manual
  renewal goroutine — etcd sessions already keepalive under the hood.
  Interpreting `RenewInterval` here would fight with the session's own
  behavior; the value is only used for consul/redis.
- **TLS config uses the shared block.** The `TLS` field is
  `starter.TLSConfig` from `stdlib/starter`; `Enabled` gates it, then
  `CertFile`/`KeyFile` (mutual TLS) and `CAFile` (server verification)
  match every other Go-Spring starter.
- **No `go mod tidy` against the proxy.** `stdlib/lock` is workspace-local;
  tidy would 404.

## 4. Trade-offs / Alternatives Rejected

- **Reusing an application-provided `clientv3.Client` — rejected for this
  starter.** Unlike the redis backend (which routinely reuses the app's
  Redis client), an etcd cluster used *only* for coordination is common
  enough that a self-contained config is friendlier; the lock owns its
  own client and closes it on destroy.
- **Native etcd election API — rejected.** `lock.Election` sits on top of
  the same `Locker` for every backend so users get uniform semantics; using
  etcd's `concurrency.Election` here would fragment the abstraction (see
  memory `project_task65_lock.md`).
