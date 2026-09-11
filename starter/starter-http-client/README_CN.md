# starter-http-client

[English](README.md) | [中文](README_CN.md)

`starter-http-client` 是 Go-Spring **声明式 HTTP 客户端**的运行时部分——对标
Spring 的 OpenFeign / `@HttpExchange`。你在 IDL 中把远程服务声明为一个接口,用
[`gs-http-gen`](../../../gs/gs-http-gen) 生成调用代码,再设好 `Target` 即可调用。服务发现、负载均衡、韧性(限流/熔断/重试)
以及链路追踪透传都已替你接好,一次微服务调用不再需要为每个客户端手工拼装拨号器
和熔断器。

Go 没有运行时代理,所以与 Feign 不同,调用代码由代码生成在构建期产出,而非依赖
反射——同样的声明式体验,但没有运行时魔法。生成的代码只依赖 stdlib
(`net/http` + `httpclt`),本 starter 负责提供它运行所需的传输层。

## 安装

```bash
go get go-spring.org/starter-http-client
```

## 工作原理

```
 生成的 Client (gs-http-gen)          starter-http-client
 ┌───────────────────────────┐       ┌──────────────────────────────────────┐
 │ Greet(ctx, req)           │       │ *http.Client.Transport =              │
 │   HTTPClient ─────────────┼──────▶│   resilience → discovery+LB → trace  │
 └───────────────────────────┘       └──────────────────────────────────────┘
```

生成的 `Client` 只持有一个 `*http.Client`。本 starter 为每个配置项注册一个
`*http.Client`,其 `http.RoundTripper` 由 [`starter-http-client/httpx`](../../../starter-http-client/httpx)
用三个可组合的 stdlib 抽象装配而成,全部收敛在同一个 `http.RoundTripper` 缝隙上:

* [`discovery`](../../../cloud/discovery) —— 设置了 `service-name` 时,`Resolver`
  持续维护最新的端点快照;
* [`loadbalance`](../../../cloud/loadbalance) —— `Pool` 为每次请求挑选一个存活端点
  (任意已注册策略,并可选离群剔除),传输层随即把请求主机改写为该端点;
* [`resilience`](../../../cloud/governance/resilience) —— 可选的执行器包裹整条链路,使限流、
  熔断与重试保护每一次调用。由于它位于负载均衡器**之外**,重试会重新挑选一个
  新端点,而熔断器则以逻辑服务名为键。

## 快速开始

### 1. 声明接口并生成客户端

在 IDL 中描述远程调用(见 [example/idl/greet.idl](example/idl/greet.idl)),用
`gs-http-gen --client` 生成 Go 客户端。生成的包([example/proto](example/proto))
提供一个只含 `Target` 字段的 `Client` 结构体。

### 2. 引入 starter 并配置客户端实例

```go
import _ "go-spring.org/starter-http-client"
```

`spring.http-client.<name>` 下的每个配置项为进程级 transport 贡献一条路由。在
直连地址与经由发现的服务之间切换,只需改配置——调用代码始终不变。见
[example/conf/app.properties](example/conf/app.properties):

```properties
# 直连地址 —— 固定到某台主机,不走发现。
spring.http-client.direct.addr=127.0.0.1:9471

# 服务发现 + 负载均衡 —— 按逻辑名路由。
spring.http-client.discovered.service-name=greet-svc
spring.http-client.discovered.discovery=static
spring.http-client.discovered.balancer=round_robin

# 韧性不在这里配置:策略是进程级 govern.*(starter-governance)。连续 2 次失败熔断:
#   govern.enabled=true
#   govern.default.enabled=true
#   govern.default.error-threshold=2
#   govern.default.open-duration=30s
spring.http-client.guarded.addr=127.0.0.1:9473
```

### 3. 调用

生成的客户端直接给 `Target` 即可——无需注入任何东西,已安装的 transport 按 target
分派。见 [example/example.go](example/example.go):

```go
client := &proto.Client{Target: "greet-svc"}
_, resp, err := client.Greet(ctx, &proto.GreetReq{Name: "Grace"})
```

## 核心特性

[example.go](example/example.go) 会启动三个进程内后端,并端到端断言全部四项结果:

* **直连地址** —— `direct` 客户端固定到某个后端。
* **服务发现 + 负载均衡** —— `discovered` 客户端按服务名路由,在多个实例间轮询
  (通过 `servedBy` 字段在两者间跳变来观测)。
* **韧性** —— `guarded` 客户端指向一个总是失败的后端;超过错误阈值后熔断器打开,
  后续调用以 `resilience.ErrCircuitOpen` 快速失败,不再触网。
* **链路追踪透传** —— 客户端 span 注入 W3C `traceparent` 头,后端原样回显,因此
  同一个 `trace_id` 在两端都可观测。

## 配置项

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.http-client.<name>.addr` | — | 直连 `host:port`;可与 `service-name` 同配,此时 service-name 只作纯治理 label。 |
| `spring.http-client.<name>.service-name` | — | 经由发现解析的逻辑名;只要设置即同时是治理 resource label。 |
| `spring.http-client.<name>.discovery` | — | 已注册的发现后端名,设置 `service-name` 时必填。 |
| `spring.http-client.<name>.balancer` | `round_robin` | 策略:`round_robin`、`least_conn`、`consistent_hash`、`weighted`、`zone_aware`。 |
| `spring.http-client.<name>.driver` | `default` | 传输装配 driver([RegisterDriver](driver.go)):嵌 `DefaultDriver` 在装配产物外加 auth/metric,或整体替换为自己的 `httpx.NewTransport` 调用。 |
| `spring.http-client.<name>.suspend-threshold` | `0` | 剔除端点的连续失败次数(0 表示不剔除)。 |
| `spring.http-client.<name>.suspend-for` | `0` | 被剔除端点的隔离时长。 |
| `spring.http-client.<name>.tls.enabled` | `false` | 打开该 entry 的 TLS 配置面(完整 `tls.*`:cert-file/key-file/ca-file/server-name/insecure-skip-verify)。 |

韧性与故障注入在这里**没有配置 key**:策略是进程级 `govern.*`(见
starter-governance)。治理资源标签在发现模式下为 `http:<service-name>`,直连模式下为
`http:<addr>`。单次请求超时来自 `govern.default.attempt-timeout`。

本 starter 在装配期即快速失败:`addr` 与 `service-name` 至少设置其一;仅按
服务名路由(未配 `addr`)时 `discovery` 必填。解析到 `http:*` 资源的 error-rate
熔断有 `min-requests=5` 下限(govern rule 显式更高值优先)——由 `starter-http-client/httpx` 施加。

## 可观测性

可观测能力内置在 [`starter-http-client/httpx`](../../../starter-http-client/httpx):底层传输自动包
[`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp),
启用韧性的执行器还会包 fault + observe(outcome 分维指标 + 按
每次出站请求都会产生一个客户端 span,并通过
[`starter-otel`](../../starter-otel) 安装的 OpenTelemetry 全局对象注入 W3C
`traceparent` 头。没有 `starter-otel` 时这些全局对象是空实现——不产生 span,也不
改动请求字节。这与其他客户端类 starter 采用的零配置开箱一致。

## 切换韧性后端

`resilience.driver=default` 使用内置的零依赖实现。切换到 Sentinel 只需
`driver=sentinel` 外加空导入 [`starter-resilience`](../starter-resilience),
无需改动任何代码。

