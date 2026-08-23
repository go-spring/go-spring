# httputil

[English](README.md) | [中文](README_CN.md)

`httputil` provides a few HTTP helper functions to extract the scheme, protocol version, address and port from a request, and to flatten a header into a string.

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

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
