# httpx
[English](README.md) | [中文](README_CN.md)

`httpx` 是 Go-Spring 声明式 HTTP 客户端(对标 OpenFeign / `@HttpExchange`)背后的
运行时装配器。Go 无运行时代理,调用点由 `gs-http-gen` 生成,生成的客户端只持有一个
`*http.Client`,`httpx.NewTransport` 负责把这个 client 的 `http.RoundTripper` 装
配起来。

## 特性

- 唯一缝隙:`http.RoundTripper`——与 `resilience`、`otelhttp` 复用同一缝隙。
- 内置可观测:底层传输自动包 `otelhttp`(client span + trace 传播),启用
  resilience 时执行器默认包 fault + observe(outcome 分维指标、按
  `Observability` 门控的访问日志)。未注册 OTel SDK 时均为 no-op。
- 服务发现 + 负载均衡:配置 `ServiceName` 时接 discovery `Resolver` +
  `loadbalance.Pool`(round-robin / least-conn / consistent-hash / weighted /
  zone-aware),可选离群剔除。
- 直连模式:只填 `Addr` 即把每次请求重写到该主机,生成客户端的 `Target` 可留空。
- 默认走治理:未显式给执行器时,自动从集中治理中心按 `Resource`(由
  ServiceName/Addr 推导)解析执行器——进程级 `govern.*` 规则 + 热更新生效,并对
  error-rate 熔断施加 `min-requests=5` 下限。
- TLS:`tls.*` 证书面(客户端证书对、CA bundle、server name、insecure 逃生舱)
  直接构建进底层传输。
- 可选 `resilience` 执行器包住整条链,重试会重新进入负载均衡挑一个新端点,熔断按
  逻辑服务名归键。
- 装配期即校验 discovery 后端 / 负载策略,配置错立即失败。

## 用法

```go
import "go-spring.org/cloud/httpx"

rt, closeFn, err := httpx.NewTransport(httpx.Config{
    ServiceName: "user-svc",     // 直连模式留空
    Discovery:   "redis",
    Balancer:    "round_robin",
})
if err != nil {
    log.Fatal(err)
}
defer closeFn()

client := &http.Client{Transport: rt}
```

直连模式:填 `Addr`、`ServiceName` 留空,`httpx` 会把每次请求的 host 重写为
`Addr`。`Base`(可选)是裸底层传输——TLS 定制 clone、自定义 dialer——trace 由
`httpx` 自己叠在其上。

Bean 化封装见 `starter/starter-http-client`。
