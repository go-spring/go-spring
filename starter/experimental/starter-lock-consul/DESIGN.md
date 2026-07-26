# starter-lock-consul Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-consul` is a Contributor-archetype starter (`starter/DESIGN.md`
§2.3) in the integration layer. It contributes named `lock.Locker` beans
backed by Consul sessions and `api.Lock`.

## 1. Responsibilities & Boundaries

- Binds `spring.lock.<name>` entries to Consul-backed `lock.Locker` beans,
  one per entry, registered under the config name and exported as
  `lock.Locker`.
- Owns its own `api.Client` per instance (Consul does not expose a shared
  client the way `starter-go-redis` does); constructs it at boot with the
  configured Address/Scheme/Token/TLS.
- `TryAcquire` maps to `LockTryOnce`; `Acquire` uses the blocking `Lock()`
  call; `Lost()` is the channel `Lock()` returns.

## 2. Key Abstractions & Seams

- **Seam is the bean type.** No package-level driver string; switching
  backend is a blank-import change.
- **TTL is Consul-clamped.** Consul sessions require the TTL in
  `[10s, 86400s]`; values outside that window are clamped at startup so a
  smaller-than-10s config still boots (with the effective TTL used at
  runtime), rather than crashing on session creation.
- **`api.ErrLockNotHeld` swallowed on Unlock.** The abstraction guarantees
  `Unlock` is idempotent; a "lock already released" error therefore does
  not surface as a caller-facing error.

## 3. Constraints

- **`Address` is required.** No localhost fallback; a missing address is
  rejected at boot.
- **`Scheme=https` is not the same as TLS.** `Scheme` only chooses the URL
  scheme; a functional TLS setup requires `TLS.Enabled=true` and the
  supporting cert/CA fields. Setting only `Scheme=https` fails to dial.
- **KeyPrefix defaults to `lock/`.** Multiple apps sharing one Consul
  cluster differentiate their key spaces via prefix rather than colliding
  on flat keys.
- **No `go mod tidy` against the proxy.** `spring/lock` is workspace-local.

## 4. Trade-offs / Alternatives Rejected

- **Custom blocking loop instead of `api.Lock` — rejected.** `api.Lock`
  already handles session creation, keepalive, and the "waiter released"
  channel, and matches how idiomatic Consul users implement locks; a hand-
  rolled loop would duplicate all of that.
- **Consul-native leader election API — rejected.** `lock.Election` sits on
  top of the shared `Locker` abstraction for every backend, so the choice
  between Consul/etcd/Redis/K8s does not change the caller-facing API.
