# httputil

[English](README.md) | [中文](README_CN.md)

`httputil` 提供几个 HTTP 辅助函数，用于取出 scheme、协议版本、地址与端口，并把请求头展平为字符串。

## 使用方式

```go
import (
    "go-spring.org/stdlib/httputil"
    "go.opentelemetry.io/otel/attribute"
)

r := c.Request
scheme := httputil.Scheme(r)                          // "https" / "http"
proto := httputil.ProtocolVersion(r.Proto)            // "1.1" / "2" / "3"
addr, port := httputil.ServerAddrPort(r.Host, scheme) // host, 默认端口时为 0

attrs := []attribute.KeyValue{
    attribute.String("url.scheme", scheme),
    attribute.String("network.protocol.version", proto),
    attribute.String("server.address", addr),
}
if port != 0 {
    attrs = append(attrs, attribute.Int("server.port", port))
}
```

### API 列表

| 函数 | 返回 | Semconv 属性 |
|---|---|---|
| `Scheme(r *http.Request) string` | `"https"`（TLS）/ `"http"` | `url.scheme` |
| `ProtocolVersion(proto string) string` | `"1.0"`/`"1.1"`/`"2"`/`"3"` | `network.protocol.version` |
| `ServerAddrPort(host, scheme string) (string, int)` | host、port（默认端口时 0） | `server.address`、`server.port` |
| `FlattenHeader(h http.Header) string` | `"K: V; K: V"` |（日志便捷工具，非 semconv 属性）|

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
