# httpclt Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`httpclt` is the runtime the code generator (`gs-http-gen`) targets. It lives in
`stdlib` and imports only `net/http`, `stdlib/jsonflow` and a couple of standard
packages, so a generated client can stay stdlib-only and never leak a starter
dependency into caller code.

## 1. Responsibilities and Boundaries

- Hold the per-request `Metadata` shape the generator emits, drive it through
  request construction, dispatch and streaming decode.
- Provide the two hooks (`Metadata.Client` and the package-level `DoRequest`
  variable) that let a caller — a starter, a test, or plain application code —
  swap the entire HTTP hop without touching the generated call site.
- Refuse to know about service discovery, load balancing, resilience or trace
  propagation. All of that belongs to whatever `*http.Client` is injected via
  `Metadata.Client` (see `spring/httpx`).

## 2. Key Abstractions and Seams

- **`Metadata`** — the declarative call description. It intentionally includes
  a `Client *http.Client` field: this is the single seam through which a
  generated client picks up service discovery + LB + resilience + otel, without
  changing anything in the generated code.
- **`DoRequest` package variable** — a test/observability seam. Replacing it
  short-circuits the whole HTTP hop.
- **`QueryStringer` / `EncodeForm` / `ResponseObject` interfaces** — the two
  encoding pluggables (`QueryForm`, `EncodeForm`) and the one decoding
  pluggable (`DecodeJSON`) that generated types implement, so `httpclt` never
  performs runtime reflection on business structs.

## 3. Constraints

- The runtime is generator-driven: fields on `Metadata` are the contract with
  `gs-http-gen` output. Renaming a field is a break of that contract.
- `Metadata.Client == nil` must silently fall back to `http.DefaultClient`,
  because the generator emits a struct field the application may leave unset in
  tests or simple scripts.
- Streaming JSON is the default: `ObjectResponse` / `JSONResponse` decode
  incrementally through `jsonflow`, so the runtime does not buffer the full
  response body in memory.

## 4. Trade-offs and Alternatives Rejected

- **No client construction inside httpclt.** The transport, timeouts, cookie
  jar and TLS config all live in the `*http.Client` the caller injects. This
  is what makes the client-side integrations (`starter-http-client`, tests,
  contract stubs) pluggable without changing generated code.
- **No reflection over business types.** Interface methods
  (`QueryForm`, `EncodeForm`, `DecodeJSON`) keep the runtime fast and dependency-
  free at the cost of code generation carrying its weight on the other side.
