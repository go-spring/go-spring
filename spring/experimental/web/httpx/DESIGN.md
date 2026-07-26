# httpx Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`httpx` supplies the transport that generated declarative HTTP clients run on.
It sits in the `stdlib` zero-dependency layer: the assembler and every seam it
composes (`discovery`, `loadbalance`, `resilience`) are stdlib abstractions, so
the generated code and this package never import a concrete starter.

## 1. Responsibilities and Boundaries

- Assemble one `http.RoundTripper` from a declarative `Config`; return a close
  function that releases the `discovery` watch and the `resilience` executor.
- Address the request in one of three modes, chosen by config: through
  discovery + a load-balancing pool, pinned to a direct `Addr`, or transparently
  passing through whatever host the generated client set.
- Refuse to touch anything above the transport layer. Cookie jars, retry-body
  buffering, request rewriting for tracing headers — none of that lives here;
  they belong in the client or a `Base` transport a starter injects.

## 2. Key Abstractions and Seams

- **`http.RoundTripper`** — the single seam. `otelhttp`, `resilience` and
  `httpx` all speak it, so instrumentation and protection layer around the
  balancer without any custom glue.
- **`Config.ServiceName` vs `Config.Addr`** — the two addressing modes collapse
  to the same shape internally (`balancedTransport` vs `fixedHostTransport`),
  so the layers above them run identical code.
- **`Base`** — the underlying transport (typically `otelhttp.NewTransport(...)`
  supplied by `starter-http-client`). Keeping observability injected here is
  what lets stdlib stay otel-free.

## 3. Constraints

- Resilience wraps the whole chain, **outside** the balancer. A retry therefore
  re-picks a fresh endpoint (needed for failover) and the breaker keys on the
  logical service name (the request `Host` set by the generated client), not on
  the physical endpoint. Never move the resilience wrapper below the balancer.
- `balancedTransport.RoundTrip` clones the request before rewriting `URL.Host`
  and `Host`: `net/http` may retry and the resilience layer above reuses the
  original request across attempts. Mutating the caller's request in place is
  a correctness bug.
- `NewTransport` fails fast on unknown discovery / balancer names. Delayed
  discovery of a wiring mistake at first request time is worse than a bootup
  error.

## 4. Trade-offs and Alternatives Rejected

- **No YAML / reflection / annotations.** Declarative behavior is produced by
  `gs-http-gen`; runtime magic is deliberately avoided because Go has no
  proxy mechanism and reflection-based clients pay a cost on every call.
- **No cross-cutting logging / metrics inside httpx.** Observability is a
  starter concern (via `Base`). This preserves stdlib's zero-dependency rule
  and lets one project swap otel for a different backend without touching the
  assembler.
- **No pluggable "chain" API.** The four-stage order (resilience → balancer →
  otel-base → net/http) is deliberate; exposing it as a user-composable chain
  would invite putting resilience below the balancer and losing failover on
  retry.
