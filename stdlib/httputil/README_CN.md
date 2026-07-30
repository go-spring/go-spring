# httputil

[English](README.md) | [中文](README_CN.md)

`httputil` 从入站 HTTP 请求派生 OpenTelemetry HTTP 语义约定的属性**值**，使各 server starter（gin、echo、fiber……）无需各自重复实现相同映射。

函数返回纯 Go 类型（string/int），不是 `attribute.KeyValue`，因此本包不依赖任何 OTel 组件--调用方按需用 `attribute.String`/`attribute.Int` 包装。

遵循的约定是自 v1.27.0 起稳定的 OTel HTTP semconv：`url.scheme`、`network.protocol.version`、`server.address`、`server.port`。

## 用法

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

### API

| 函数 | 返回 | Semconv 属性 |
|---|---|---|
| `Scheme(r *http.Request) string` | `"https"`（TLS）/ `"http"` | `url.scheme` |
| `ProtocolVersion(proto string) string` | `"1.0"`/`"1.1"`/`"2"`/`"3"` | `network.protocol.version` |
| `ServerAddrPort(host, scheme string) (string, int)` | host、port（默认端口时 0） | `server.address`、`server.port` |
| `FlattenHeader(h http.Header) string` | `"K: V; K: V"` |（日志便捷工具，非 semconv 属性）|

函数为何返回值（而非 OTel 类型）、多数为何接收纯字符串而非 `*http.Request`，见 [DESIGN_CN.md](DESIGN_CN.md)。
