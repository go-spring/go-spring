# httputil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `httputil` derives OpenTelemetry HTTP
semantic-convention attribute *values* from an inbound HTTP request, so server
starters (gin, echo, fiber, ...) don't each reimplement the same mappings.

## 1. Responsibilities & Boundaries

- Derive the values for OTel HTTP semconv (stable since v1.27.0) attributes from
  a request: `url.scheme` ([Scheme]), `network.protocol.version`
  ([ProtocolVersion]), `server.address`+`server.port` ([ServerAddrPort]).
- Return plain Go types (string/int), **not** `attribute.KeyValue`. This keeps the
  package free of any OTel dependency - callers wrap the values themselves.
- Not a request-parsing, routing, or middleware library. Only the semconv-value
  derivations that every server starter duplicates belong here.

## 2. Key Seams

- **Value-only, no OTel types**: the functions compute strings/ints. A starter
  that hasn't imported OTel can still use them; one that has wraps them with
  `attribute.String`/`attribute.Int`. The semconv *key names* stay in each
  starter (they're an OTel concern), only the *value derivation* is shared.
- **Decoupled from *http.Request where possible**: [ProtocolVersion] and
  [ServerAddrPort] take plain strings, not a request, so non-gin transports (gRPC,
  HTTP/3 via quic) that obtain the proto or host elsewhere can reuse them.
  [Scheme] takes the request because it reads `r.TLS`.

## 3. Constraints

- Follows semconv v1.27.0: `server.port` is dropped when it's the scheme default
  (80/443), since the convention makes it conditionally required only when
  non-default.
- No OTel dependency; only `net`, `net/http`, `strconv`.
