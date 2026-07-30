# httputil

[English](README.md) | [中文](README_CN.md)

`httputil` derives OpenTelemetry HTTP semantic-convention attribute *values* from
an inbound HTTP request, so server starters (gin, echo, fiber, ...) don't each
reimplement the same mappings.

The functions return plain Go types (string/int), not `attribute.KeyValue`, so the
package stays free of any OTel dependency - callers wrap the values with
`attribute.String`/`attribute.Int` as needed.

The conventions followed are the OTel HTTP semconv stable since v1.27.0:
`url.scheme`, `network.protocol.version`, `server.address`, `server.port`.

## Usage

```go
import (
    "go-spring.org/stdlib/httputil"
    "go.opentelemetry.io/otel/attribute"
)

r := c.Request
scheme := httputil.Scheme(r)                          // "https" / "http"
proto := httputil.ProtocolVersion(r.Proto)            // "1.1" / "2" / "3"
addr, port := httputil.ServerAddrPort(r.Host, scheme) // host, 0 if default port

attrs := []attribute.KeyValue{
    attribute.String("url.scheme", scheme),
    attribute.String("network.protocol.version", proto),
    attribute.String("server.address", addr),
}
if port != 0 {
    attrs = append(attrs, attribute.Int("server.port", port))
}
```

### API

| Function | Returns | Semconv attribute |
|---|---|---|
| `Scheme(r *http.Request) string` | `"https"` (TLS) / `"http"` | `url.scheme` |
| `ProtocolVersion(proto string) string` | `"1.0"`/`"1.1"`/`"2"`/`"3"` | `network.protocol.version` |
| `ServerAddrPort(host, scheme string) (string, int)` | host, port (0 if default) | `server.address`, `server.port` |
| `FlattenHeader(h http.Header) string` | `"K: V; K: V"` | (log convenience, not a semconv attribute) |

See [DESIGN.md](DESIGN.md) for why the functions return values (not OTel types)
and why most take plain strings rather than `*http.Request`.
