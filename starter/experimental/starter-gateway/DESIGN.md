# starter-gateway Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-gateway` is a **Server-archetype** starter (see
[starter/DESIGN.md](../DESIGN.md) §2.1) that runs an independent API gateway
on its own port (default `:9440`). It lands the Spring Cloud Gateway
Route/Predicate/Filter model with Go idioms rather than a runtime DSL:
Predicates are `func(*http.Request) bool`, Filters are
`func(next http.Handler) http.Handler`, and a route is function composition.

## 1. Responsibilities & Boundaries

- **In scope:** route table binding + hot reload, `lb://` upstream via
  `spring/discovery` + `spring/loadbalance`, and a `FilterWrapper` seam
  through which pluggable filters (jwt-auth, lua) mount without a hard
  import.
- **Out of scope:** runtime DSL / rules engine, control-plane sync,
  L4/TCP proxying. Resilience (retry/circuit-breaker/rate-limit) is
  delegated to `spring/resilience`, not reimplemented.

## 2. Key Abstractions

- **RouteTable / GatewayServer.** Two beans, one bound to
  `${spring.gateway}`, the other to `${spring.gateway.server}`, using
  `value:"..."` tags on struct fields — **not** `gs.TagArg("${prefix}")`,
  which fails for aggregate structs with "property is not a simple value".
- **`FilterWrapper` seam.** A single-method local interface
  (`Wrap(next http.Handler) http.Handler`), collected as
  `map[string]FilterWrapper` with `autowire:"?"`. Contributors like
  `starter-security-jwt` and `starter-lua-filter` register beans that
  satisfy this shape; the gateway never imports them.
- **`lb://<service>`.** An upstream URL prefix that resolves through
  `discovery.NewLiveDialer` + `loadbalance.Pool.Pick` — the same
  client-side stack every other client starter uses. Mesh mode degrades it
  centrally, no per-gateway branching.

## 3. Constraints

- **Warmup, not eager compile.** `Wrappers map[string]FilterWrapper
  autowire:"?"` is only populated after construction, so route compilation
  is deferred to `GatewayServer.Run`'s `warmup()`; a bad config still fails
  at startup (same pattern as `starter-scheduler`), it just fires slightly
  later.
- **`gs.Server` bean must be `.Name("gatewayServer")`.** The container
  already has a default web-server bean named `__default__`; two `gs.Server`
  beans without distinct names collide as duplicates.
- **`gs.Dync[map]` default must be empty.** Write `${routes:=}`, never
  `${routes:={}}` — the bind layer rejects non-empty map defaults with
  "map can't have a non-empty default value".
- **Hot reload via map-pointer compare.** The refresh loop compares
  `reflect.ValueOf(m).Pointer()` against the last observed pointer; on
  change it recompiles. Compile failures preserve the previous table
  rather than serving a broken one.
- Internal deps resolve through `go.work`; `go mod tidy` is not run.

## 4. Trade-offs / Alternatives Rejected

- **No runtime DSL / rules engine.** Predicates and Filters are Go
  functions. A DSL adds a parser, a second execution model, and a
  hot-reload story we already get from Go binaries via
  `gs-http-gen`-style config refresh.
- **No hard import of jwt-auth / lua modules.** The `FilterWrapper` seam
  keeps the gateway independent of `starter-security-jwt` and
  `starter-lua-filter`, so an application chooses which filters to
  register by blank-importing them — the same way the WebSocket family
  swaps implementations.
- **`gs.Server` (own port) instead of a Contributor on the main web
  server.** A gateway is a distinct process concern with its own port,
  timeouts, and lifecycle; multiplexing it onto the application's business
  port makes rollout, TLS termination, and traffic isolation harder.
