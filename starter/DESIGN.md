# Go-Spring Starter Design Guide

[English](DESIGN.md) | [中文](DESIGN_CN.md)

This document captures the design constraints that every official Go-Spring
starter follows. It is meant to keep the family consistent and to guide anyone
adding a new starter. For the domain-based catalog of what exists today, see
[README.md](README.md).

A starter is an *integration module*: it wires one third-party service or
framework into the Go-Spring IoC container and server lifecycle, and nothing
more. Business logic, deployment scaffolding, and cross-starter abstractions do
not belong here.

## 1. Module Layout

- **One starter, one Go module.** There is no `go.mod` at the repo root; each
  starter owns its own module and dependency graph. This keeps a Redis app from
  pulling in Kafka's transitive dependencies.
- **Fixed file skeleton.** A starter directory contains `starter.go` (bean
  registration + lifecycle), `config.go` (the bound `Config` struct and any
  driver registry), `README.md` / `README_CN.md`, and an `example/` module that
  exists for smoke tests and integration only — no `build.sh` / `bootstrap.sh` /
  deployment scaffolding, only `check.sh` / `gen.sh` / source.
- **Apache License header** on every source file (see
  [../LICENSE_HEADER](../LICENSE_HEADER)).
- **Internal deps resolve through `go.work`, never `require`.** Modules in this
  workspace depend on each other via the workspace file; adding a `require` on an
  internal module sends `go mod tidy` to the proxy and 404s.

## 2. The Four Archetypes

Every starter falls into exactly one of four shapes. Its shape dictates its
lifecycle, its port behavior, and how the application consumes it.

### 2.1 Server starters (own a listener)

Web (`gin`, `echo`, `hertz`, ...) and RPC (`grpc`, `kitex`, `thrift`,
`dubbo`, ...) starters own a network listener and plug into the Go-Spring server
lifecycle by exporting a `gs.Server` bean.

- **Each server binds its own port.** A server starter listens on a distinct
  address from its own `Config` (e.g. `${spring.grpc.server}` → `addr:=:9494`).
  Two server starters in one process must not share a port; the application
  assigns non-conflicting addresses. Contributor starters (§2.3) deliberately do
  *not* open a port — they mount onto a server the app already runs.
- **Listen early, serve on the ready signal.** `Run(ctx, sig)` binds the
  listener immediately so a port conflict fails startup, then blocks on
  `<-sig.TriggerAndWait()` before `Serve`. This guarantees the socket is bound
  before Go-Spring reports readiness, but no traffic is served until all beans
  are wired.
- **Graceful shutdown in `Stop()`.** HTTP servers call `Shutdown`; RPC servers
  call `GracefulStop`.
- **The app owns routes, the starter owns the server.** The application supplies
  a register function bean (`RouterRegister`, `ServiceRegister`,
  `HandlerRegister`, ...); the starter creates and configures the engine and its
  transport. Registration is the seam.
- **Enabled-by-default toggle.** `gs.OnProperty("spring.<x>.server.enabled").
  HavingValue("true").MatchIfMissing()` gates registration, and it is usually
  further conditioned on the register bean being present
  (`Condition(gs.OnBean[...])`).

### 2.2 Client starters (driver mode + multi-instance)

Database, cache, and message-queue clients (`go-redis`, `gorm-*`, `mongodb`,
`kafka`, `nats`, ...) connect *out* to an external service.

- **Multi-instance only, via `gs.Group` / `gs.Module`.** Client starters do
  **not** register a default singleton bean. They bind a
  `map[string]Config` under the prefix and register one named bean per entry.
  Reason: the default-singleton + multi-instance dual registration was
  error-prone and the conditional singleton semantics were opaque. The
  application selects an instance by name (`autowire:"a"`), and adding a second
  instance is a pure-config change.
- **Address is required — fail fast.** A client must never silently fall back to
  `localhost`. Fields default to empty (`${addr:=}`), and the constructor rejects
  a config with no address (and, where discovery applies, no service name) at
  startup via `errutil.Explain`. go-spring's `expr:` tag validates one field at a
  time, so an "addr OR service-name" rule lives in the constructor, not a tag.
- **Driver pattern for pluggable backends.** A client exposes a `Driver`
  interface plus a package-level registry (`RegisterDriver`, panic on
  dup/empty/nil). `DefaultDriver` ships built in; a company can register its own
  driver and select it with `${driver:=...}` without forking the starter. This
  is also the seam through which service discovery is injected (the driver builds
  the dialer). Optional capabilities go on *separate* interfaces (e.g. go-redis's
  `ClusterDriver`) so existing custom drivers keep compiling.
- **Startup connection check.** Where the client library allows it, the
  constructor performs a bounded probe (e.g. Redis `PING` with `DialTimeout`) so
  a misconfiguration surfaces at boot, not on first request.
- **Every instance has a `Destroy`.** Each bean registers a destructor that
  `Close()`s the connection and stops any background goroutine or discovery
  watch behind it. Missing destroy hooks were a known gap and are now required.

### 2.3 Contributor starters (no port of their own)

WebSocket (`websocket`, `websocket-coder`), middleware (`lua-filter`), and
authorization (`casbin`, `oauth2-client`) starters contribute a configured bean
that the application mounts onto infrastructure it already runs.

- **They open no listener.** A WebSocket starter contributes a
  `*websocket.Upgrader` / `*websocket.AcceptOptions`; the app upgrades
  connections on its existing HTTP server. This is why WebSocket lives apart from
  the server archetype.
- **The bean type is the seam.** Switching between two implementations of the
  same capability is a one-line blank-import change; see the shared-prefix rule
  in §3.

### 2.4 Global / infrastructure starters

`otel` (observability core) and `pprof` (diagnostics) install process-wide
facilities.

- **`starter-otel`** builds shared Tracer/Meter providers and installs them as
  OTel globals; client starters instrument against those globals so that when
  otel is absent the hooks are no-ops (zero-config opt-in).
- **`starter-pprof`** runs a *dedicated* HTTP server on its own port for runtime
  profiles, kept off the application's main port on purpose.

## 3. Cross-Cutting Constraints

- **Config prefix is per capability, not per implementation.** Two starters that
  implement the *same* capability share one prefix — `starter-websocket` and
  `starter-websocket-coder` both use `spring.websocket`; `starter-kafka`
  (franz-go) and `starter-kafka-sarama` both use `spring.kafka`. Users pick one
  implementation; switching is a blank-import swap with zero config migration.
  Do not split the prefix per implementation for cosmetic isolation — isolation
  already comes from module separation and distinct bean types.
- **Fail-fast over silent defaults.** Required inputs (addresses, credentials,
  mode-specific fields) are validated at startup with a clear `errutil.Explain`
  message rather than defaulted to something that half-works.
- **Production capabilities are part of the wrapper.** Health/readiness,
  startup connection validation, TLS, and destroy hooks are considered part of
  what a starter must provide, not optional extras. TLS is a nested
  `TLSConfig` (`enabled` + cert/key/CA), off by default.
- **Duplication is currently tolerated over premature abstraction.** Common
  capabilities (health, TLS, fail-fast) are intentionally written once per
  module rather than extracted into a shared package. A consolidation pass may
  come later; until then, do not build cross-starter helper packages.
- **Service discovery is client-side only.** Client starters can resolve a
  `ServiceName` to live endpoints through `stdlib/discovery` (`LiveDialer`
  injected via the driver's dialer hook). RPC *provider* registration is
  deliberately out of scope — it is too framework-specific to unify. When
  `ServiceName` is empty the client dials the address directly, unchanged.
- **Observability is central-define, edge-bridge.** The starter emits through
  the OTel globals or bridges the library's internal logs into go-spring `log`
  via a `SetLogger` hook; it must also add a go-spring `FileLogger` sink or the
  console output is lost.

## 4. Adding a New Starter — Checklist

1. Pick the archetype (§2); it fixes your lifecycle and port behavior.
2. Own module, standard file skeleton, license headers.
3. Choose the config prefix by *capability* (reuse an existing one if you are a
   second implementation).
4. Client? → `gs.Group` multi-instance, driver registry, required address with
   fail-fast, startup probe, per-instance `Destroy`.
5. Server? → own port, listen-early/serve-on-ready, graceful `Stop`,
   app-supplied register bean, enabled-by-default toggle.
6. Add health, TLS, and destroy where the underlying library supports them.
7. Ship a bilingual README pair and an `example/` with `check.sh` only (no
   deployment scaffolding).
8. Resolve internal deps through `go.work`, never `require`.
