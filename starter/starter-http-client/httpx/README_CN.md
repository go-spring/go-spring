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
- 默认走治理:未显式给执行器时,直接从集中治理中心按 `Resource`(由
  ServiceName/Addr 推导)取执行器——进程级 `spring.governance.*` 规则 + 热更新生效。
- TLS:`tls.*` 证书面(客户端证书对、CA bundle、server name、insecure 逃生舱)
  直接构建进底层传输。
- 可选 `resilience` 执行器包住整条链,重试会重新进入负载均衡挑一个新端点,熔断按
  逻辑服务名归键。
- 装配期即校验 discovery 后端 / 负载策略,配置错立即失败。

## 用法

```go
import "go-spring.org/starter-http-client/httpx"

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

## 设计说明

* **fault 注入在最内层，observe 记录最终结果。** 启用 resilience 时执行器包成
  `observe(fault(raw))`：注入的故障位于 observe *之内*，因此看起来与真实下游失败一致，
  而 observe 度量最终结果。
* **`WrapExec` 是逃生舱。** 需要自定义顺序或额外一层的调用方，通过它替换默认的
  `observe(fault(...))` 包装。
* **传输层以上的事不在这里做。** Cookie jar、为重试缓冲请求体、注入追踪头——都不在
  `httpx`；请放到 client 或你传入的 `Base` 传输层里。
* **改写 host 前先克隆请求。** net/http 可能重试，上层 resilience 也会跨尝试复用原
  请求，因此 `balancedTransport` 在改写 `URL.Host` / `Host` 前会先克隆；就地改写
  调用方请求是正确性 bug。
* **四段顺序是固定的。** `resilience → balancer → otel-base → net/http` 是有意固定
  的，没有用户可拼装的 chain API——改变顺序会诱使把 resilience 放到 balancer 之下，
  重试时失去 failover。
