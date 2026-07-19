# endpoint Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`endpoint` is a stdlib (zero-dependency foundation) seam that lets one
starter contribute an HTTP path to another starter's management server
without either importing the other. It sits alongside `health.Indicator`
and is used by the same pattern.

## 1. Responsibilities & Boundaries

- **Does:** define one tiny interface — a `Path() string` and an embedded
  `http.Handler` — that beans exported as `Endpoint` implement.
- **Refuses:**
  - No collector, no mux, no serving code. That belongs to whoever owns
    the management server (typically `starter-actuator`).
  - No path taxonomy or reserved names. The contributor and the collector
    agree by convention; conflicts are a wiring bug the operator resolves.

## 2. Key Abstractions / Seams

- **Interface-based collection under the DI container's `Export`
  contract.** Go-Spring's container matches beans to injection points by
  concrete type plus the interfaces they explicitly `Export`. Placing this
  interface in stdlib means the contributor (e.g. `starter-otel` exposing
  `/metrics`) and the collector (`starter-actuator`) both depend on stdlib
  only. Neither imports the other, honouring `starter/DESIGN.md` §3.
- **`Path()` is a value, not a wiring detail.** The contributor owns the
  path; the collector's job is purely to mount whatever it is told. This
  keeps the interface stable even as more paths (`/build-info`,
  `/threaddump`, ...) accrete.

## 3. Constraints

- Implementations must be safe for concurrent use (the management server
  serves probes concurrently).
- `Path()` should be a static, stable value for the life of the bean.
- Do not overlap the actuator's own paths (`/health`, `/readiness`,
  `/info`); no runtime enforcement, so this is a review-time discipline.

## 4. Trade-offs / Alternatives Rejected

- **Interface over registration function.** A `RegisterEndpoint(path,
  handler)` global would work too, but it inverts control (the contributor
  runs code at init and must know the collector exists). Beans are how
  Go-Spring already wires optional contributions, and lazy bean
  instantiation means an unused endpoint costs nothing.
- **Two-method interface, not a struct.** Keeping the surface as an
  interface (rather than a `type Endpoint struct { Path string; Handler
  http.Handler }`) lets a contributor add operational methods (health
  metadata, versioning) to the same object later without a package split.
