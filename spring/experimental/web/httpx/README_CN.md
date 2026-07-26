# httpx
[English](README.md) | [中文](README_CN.md)

`httpx` 是 Go-Spring 声明式 HTTP 客户端(对标 OpenFeign / `@HttpExchange`)背后的
运行时装配器。Go 无运行时代理,调用点由 `gs-http-gen` 生成,生成的客户端只持有一个
`*http.Client`,`httpx.NewTransport` 负责把这个 client 的 `http.RoundTripper` 装
配起来。

## 特性

- 唯一缝隙:`http.RoundTripper`——与 `resilience`、`otelhttp` 复用同一缝隙。
- 服务发现 + 负载均衡:配置 `ServiceName` 时接 `discovery.LiveDialer` +
  `loadbalance.Pool`(round-robin / least-conn / consistent-hash / weighted /
  zone-aware),可选离群剔除。
- 直连模式:只填 `Addr` 即把每次请求重写到该主机,生成客户端的 `Target` 可留空。
- 可选 `resilience` 执行器包住整条链,重试会重新进入负载均衡挑一个新端点,熔断按
  逻辑服务名归键。
- 装配期即校验 discovery 后端 / 负载策略,配置错立即失败。

## 用法

```go
import "go-spring.org/spring/web/httpx"

rt, closeFn, err := httpx.NewTransport(httpx.Config{
    ServiceName: "user-svc",     // 直连模式留空
    Discovery:   "redis",
    Balancer:    "round_robin",
    Base:        otelhttp.NewTransport(http.DefaultTransport),
})
if err != nil {
    log.Fatal(err)
}
defer closeFn()

client := &http.Client{Transport: rt}
```

直连模式:填 `Addr`、`ServiceName` 留空,`httpx` 会把每次请求的 host 重写为
`Addr`。链路追踪由外部注入 `Base`(starter 关心;`httpx` 本身不 import 任何可观测
库)。

Bean 化封装见 `starter/starter-http-client`。
