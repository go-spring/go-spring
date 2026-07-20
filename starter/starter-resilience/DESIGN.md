# starter-resilience Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-resilience` is a **global / infrastructure** starter (see
[starter/DESIGN.md](../DESIGN.md) §2.4) that registers
[alibaba/sentinel-golang][sentinel] as the recommended driver for
`spring/resilience`. It registers no bean and opens no port; a blank
import is enough for any adapter to select `driver=sentinel`.

[sentinel]: https://github.com/alibaba/sentinel-golang

## 1. Responsibilities & Boundaries

- **In scope:** call `sentinel.InitDefault()` at import time and
  `resilience.RegisterDriver("sentinel", ...)`. Translate a
  backend-neutral `resilience.Policy` into sentinel rules per resource.
- **Out of scope:** deciding *where* resilience is applied — that is the
  adapter's job. `spring/resilience` ships three seams
  (`NewRoundTripper` for HTTP clients, `NewDialer` for connection dial,
  `NewHandler` for HTTP inbound admission); this starter never chooses
  between them.

## 2. Key Decisions

- **No single universal per-request seam.** Every client library has a
  different hook (oauth2 → `http.RoundTripper`, go-redis → `redis.Hook`,
  gorm → plugin callback, MQ → call-site helper). `spring/resilience`
  keeps a neutral `Executor.Execute(ctx, resource, fn)` and lets each
  adapter bridge to its own shape. This starter provides the *engine*,
  not the *seam*.
- **`Policy` → sentinel rule mapping is lazy per resource.** Rules are
  loaded on the first `Entry` for a given resource, since sentinel keys
  everything by resource name. Concurrent first-touch is guarded by a
  `sync.Mutex` + `loaded` map.
- **`MaxRetries` and `Timeout` are applied *outside* sentinel `Entry`.**
  Sentinel models neither, so the executor wraps them. `ctx.Err()`
  breaks the retry loop early so a cancelled request does not exhaust
  the budget.
- **Block reasons map to neutral sentinels.** `BlockTypeCircuitBreaking
  → ErrCircuitOpen`, `BlockTypeIsolation → ErrBulkheadFull`, default
  `→ ErrRateLimited`. Callers depend only on `spring/resilience`; the
  sentinel dependency is a starter-side detail.

## 3. Constraints

- **Init at import time.** `sentinel.InitDefault()` panics on failure so
  a misconfigured environment surfaces at boot, not first use.
- **Do not `go mod tidy`.** Internal deps (spring, stdlib) resolve
  through `go.work`; tidy would 404 on the proxy.
- **Sentinel version is pinned to v1.0.4** — later minors have
  reshuffled the flow / circuitbreaker rule struct fields;
  regressions upstream should be caught here rather than at every
  adapter.

## 4. Zero-dependency fallback

`spring/resilience` ships a built-in `default` driver (token bucket +
consecutive-failure breaker + retry + timeout, zero third-party
dependencies) so the framework works out of the box and tests do not
need to pull sentinel. This starter's value shows up on production
traffic where sentinel's adaptive flow control and tunable breakers
matter.

## 5. Trade-offs / Alternatives Rejected

- **Making `spring/resilience` depend on sentinel — rejected.** The
  four-layer rule keeps the foundation zero-dep; this starter is one
  concrete implementation, not the abstraction.
- **A single dialer / RoundTripper seam for every library — rejected.**
  `LiveDialer` is the only truly universal seam, but only covers
  connection establishment; per-request hooks live where each library
  chose to put them.
- **`otelsarama`-style upstream wrapper for MQ — rejected.** The
  wrappers are deprecated / locked to specific client releases;
  call-site span helpers age better (documented in the sibling MQ
  observability decision).
