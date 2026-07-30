# httputil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`httputil` 从入站 HTTP 请求派生 OpenTelemetry HTTP 语义约定的属性**值**，使各 server starter（gin、echo、fiber……）无需各自重复实现相同映射。

## 1. 职责与边界

- 从请求派生 OTel HTTP semconv（v1.27.0 起稳定）属性值：`url.scheme`（[Scheme]）、`network.protocol.version`（[ProtocolVersion]）、`server.address`+`server.port`（[ServerAddrPort]）。
- 返回纯 Go 类型（string/int），**不是** `attribute.KeyValue`。这让本包不依赖任何 OTel 组件——调用方自行包装值。
- 不是请求解析、路由或中间件库。只有每个 server starter 都会重复的 semconv 值派生才放在这里。

## 2. 关键设计点

- **只产出值、不碰 OTel 类型**：函数计算字符串/整数。未引入 OTel 的 starter 也能用；引入了的用 `attribute.String`/`attribute.Int` 包装。semconv 的*键名*留在各 starter（属 OTel 关注点），只共享*值派生*。
- **尽量解耦 *http.Request**：[ProtocolVersion] 和 [ServerAddrPort] 接收纯字符串而非请求对象，使非 gin 传输（gRPC、经 quic 的 HTTP/3）从别处拿到 proto/host 时也能复用。[Scheme] 接收请求是因为要读 `r.TLS`。

## 3. 约束

- 遵循 semconv v1.27.0：`server.port` 为 scheme 默认端口（80/443）时归零，因约定仅在非默认时才要求该字段。
- 不依赖 OTel；仅用 `net`、`net/http`、`strconv`。
