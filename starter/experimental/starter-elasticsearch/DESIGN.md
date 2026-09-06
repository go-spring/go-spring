# starter-elasticsearch Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-elasticsearch` is a Client-archetype starter (`starter/DESIGN.md`
§2.2) that provisions `elastic/go-elasticsearch/v8` clients. It has three
non-obvious decisions worth pinning: an optional Driver-bean seam, a nil-op
`destroy`, and discovery integration at startup only.

## 1. Responsibilities & Boundaries

- Binds each `${spring.elasticsearch}` entry to an
  `*elasticsearch.Client` bean via `gs.Group`. No single-instance
  default.
- Runs a one-shot `Info` health check at construction so a bad address,
  bad cert, or bad credential surfaces at boot instead of on first use.
- Optionally resolves node addresses from `cloud/discovery` when
  `service-name` is set; otherwise uses the static `Addresses` (or
  `CloudID`).

## 2. Key Abstractions & Seams

- **Optional Driver-bean seam.** The starter does not construct the ES
  client directly; client assembly is owned by a `Driver` (interface),
  and a company/umbrella starter may provide its own `Driver` as an
  optional container bean. When no such bean is present the starter
  falls back to its bundled `DefaultDriver`. A bean is process-wide, so
  a custom driver delegates to `DefaultDriver` for per-instance
  behavior; it lets APM/OTel-wrapped transports plug in without changing
  the starter's public API.
- **`destroy = nil` (v8 client has no `Close`).** The v8 client's
  transport uses `net/http` with idle-connection reuse — there is
  nothing to close. `destroyClient` returns `nil` only so `gs.Group`'s
  destroy slot stays populated for symmetry with other clients
  (`project_es_starter_smoke`).
- **Discovery is startup-only.** `service-name` is resolved once, at
  boot, to a fixed `Addresses` list. The v8 client does its own
  round-robin over that list but does not re-resolve. This is
  deliberate: ES nodes churn on days, not seconds, and the client
  itself sniffs cluster state.
- **`HealthCheck` always passes a context.** The transport's OTel
  instrumentation panics on a nil parent context, so `client.Info` is
  called with `WithContext(context.Background())` explicitly.

## 3. Constraints

- **`Addresses` OR `CloudID` OR `ServiceName`.** Exactly one of the
  three provisioning modes is used; empty across all is rejected via
  the health check (which cannot reach anything).
- **`DiscoveryScheme` decides scheme for endpoints.** Endpoints from
  `cloud/discovery` carry `host:port` only; the client needs
  `scheme://host:port`, so `discovery-scheme` (`http` / `https`) is
  applied when building addresses.
- **Smoke pulls ES 8.13 image (cached).** Readiness may take up to
  120 s on first boot (`project_es_starter_smoke`); tests wait
  accordingly.

## 4. Trade-offs / Alternatives Rejected

- **Sniffing / re-resolving at runtime — rejected.** The v8 client
  already handles connection pooling and dead-node backoff across the
  seed list; adding a re-resolve loop would fight the client's own
  strategy.
- **Wrapping transport with `otelelasticsearch` in the starter —
  rejected.** Transport wrapping is done by the driver's `CreateClient`
  (the bundled `DefaultDriver` installs a dynamic transport whose
  observe+resilience behavior `Init` swaps in); the base transport stays
  plain net/http so an app that does not import otel does not pay for it.
