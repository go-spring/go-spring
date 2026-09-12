# starter-nats Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-nats` is a Client-archetype starter (`starter/DESIGN.md` §2.2)
with a small twist: the injected bean is a wrapper around
`*nats.Conn`, not the raw connection type, because NATS ships two APIs
(core + JetStream) that share one connection.

## 1. Responsibilities & Boundaries

- Binds each `spring.nats.instances.<name>` entry to a `*Conn` bean via
  `gs.Group`; there is no default single-instance bean (client starters
  in this repo are multi-instance only, see `project_starter_capability_backlog`).
- `Conn` embeds `*nats.Conn` so callers keep `Publish`/`Subscribe`/
  `Request` directly on the bean; `JetStream` is non-nil only when
  `jetstream.enabled=true` and is derived from the same connection.
- Bridges async connection events (async errors, disconnect, reconnect,
  close) into go-spring's `log` so they land beside app logs.
- Applies an optional resilience executor (rate-limit + circuit breaker)
  as opt-in `PublishGuarded` / `RequestGuarded` helpers; plain
  `Publish`/`Request` stay untouched — NATS exposes no reject-capable
  middleware, so guarding lives at the call site.

## 2. Key Abstractions & Seams

- **Bean = wrapper, not raw `*nats.Conn`.** The wrapper carries the
  optional JetStream context and the resilience executor without
  forcing callers to pick two beans and reason about their relationship.
- **`Healthy()` reflects live state.** The wrapper reports
  `Conn.IsConnected()` so an actuator `health.Indicator` (or K8s
  readiness) sees the auto-reconnecting client's actual state, not a
  stale boot-time success.
- **`destroy = Drain`, not `Close`.** `Drain` lets in-flight
  subscriptions finish and closes the connection when done, matching
  the graceful-shutdown contract of the framework.
- **Resource key is per instance, not per subject.** The resilience
  executor's `resource` string is the connection bean name so limiter /
  breaker state is scoped per connection rather than per subject.

## 3. Constraints

- **JetStream requires the same connection.** With
  `jetstream.enabled=true` the JetStream context is built via
  `jetstream.New(nc)`; if that fails the raw `nc` is closed and boot
  fails. There is no second connection.
- **TLS knobs are additive.** `tls.enabled=true` triggers `nats.Secure`;
  `insecure-skip-verify`, `ca-file`, and `cert-file/key-file` layer on
  top for mTLS or CA overrides.
- **Auth is mutually orthogonal.** Username/password, token, creds file,
  and NKey seed can be set independently; each maps to a distinct
  `nats.Option`.
- **`MaxReconnects=-1` = infinite.** Client-side reconnection is the
  reliability mechanism; there is no external supervisor.

## 4. Trade-offs / Alternatives Rejected

- **Exposing `*nats.Conn` directly and a separate `JetStream` bean —
  rejected.** It forces callers to autowire both and pick the right one
  per call site; the wrapper hides that split.
- **Middleware-based guard around `Publish` — rejected.** NATS has no
  reject-capable middleware seam; wrapping `Publish` would silently
  change semantics. Opt-in `PublishGuarded` is the honest surface.
