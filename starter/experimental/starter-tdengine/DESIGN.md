# starter-tdengine Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A Client-archetype starter (`starter/DESIGN.md` §2.2) for TDengine over the
official driver-go v3 taosWS connector. The two load-bearing decisions are
the wire protocol and the seam at which resilience/observability attach.

## 1. Responsibilities & Boundaries

- **Owns**: bean lifecycle for the `spring.tdengine.instances.<name>` group, DSN
  parsing into a taosWS connector, the guarded `*sql.DB` pool, the startup
  ping probe and per-instance indicator, the per-statement guard.
- **Does not own**: SQL building for time-series idioms (super tables,
  schemaless writes stay raw SQL through `database/sql`), STMT parameter
  binding (not modeled by `database/sql`; use the SDK directly if needed),
  and connection-level load balancing (the DSN names one taosAdapter).

## 2. Key Abstractions & Seams

- **websocket (taosWS) wire** — driver-go v3 offers three connectors: the
  CGO `taosSql` (needs the native libtaos client installed), the REST
  `taosRestful` (weaker throughput/features), and the pure-Go websocket
  `taosWS`. The starter ships taosWS: zero-install, full feature set; the
  REST connector remains available as a custom driver.
- **`Client` wrapper = embedded `*sql.DB` + slot** — database/sql offers no
  transport or callback to swap after construction, so NewClient wraps
  the taosWS connector in `guardedConnector`/`guardedConn`: every pooled
  connection consults a `clientSlot` on each `ExecContext`/`QueryContext`.
  NewClient creates the slot and installs it on every pooled connection; it
  also computes the executor (from the injected `cloud.ClientParams`
  bundle the wiring passes) and installs it on the slot, so the client is
  built complete. On each statement the connection declares the statement's
  identity on the context, then routes it through the executor. The
  resilience executor is the single emitter of the
  span, the metrics and the access log — the starter only declares. This is
  the database/sql
  analog of starter-gorm's callback chain and the HTTP starters'
  RoundTripper adapters.
- **health** — `PingContext`, matching the gorm family's probe shape.

## 3. Constraints

- driver-go v3.8.2 requires a TDengine server ≥ 3.3.6.0 on the websocket
  path; older servers fail the startup ping (when `ping=true`) with the driver's
  version error, or surface it on first use otherwise.
- TDengine has no transactions; the wrapper's `Begin` delegates to the
  driver, which reports that.
- The DSN is the driver's unified format (`user:pass@ws(host:port)/db`) —
  deliberately not decomposed into host/port/user fields, keeping the
  driver's parameter surface (`?readTimeout=...` etc.) pass-through.

## 4. Trade-offs / Alternatives Rejected

- **taosWS vs taosSql (CGO)**: the CGO driver would force a native client
  install on every build/deploy host and complicate cross-compilation; the
  websocket driver pays a small protocol overhead for a zero-dependency
  build — the right default for a framework starter.
- **`*sql.DB` vs the native `taosws.Conn` API**: the native API exposes
  schemaless STMT writes `database/sql` cannot model, but a `*sql.DB` bean
  gives pooling, context, and the entire `database/sql` toolchain for free;
  the native path stays reachable through a custom driver.
- **Guard at driver.Conn vs helper methods**: guarding `ExecContext`/
  `QueryContext` covers every caller (including ORMs layered on the pool)
  without remembering to use a `Guarded*` helper — the strongest argument
  for the slightly deeper wrap.
