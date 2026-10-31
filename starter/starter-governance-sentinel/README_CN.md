# starter-governance-sentinel

[English](README.md) | [中文](README_CN.md)

`starter-governance-sentinel` 把 [alibaba/sentinel-golang][sentinel] 注册为
[`cloud/governance/dataplane/outbound/resilience`](../../cloud/governance/dataplane/outbound/resilience) 韧性框架的生产 driver。
空导入它,并在治理文档里
写 `spring.resilience.driver=sentinel` —— 此后每个经治理中心解析 executor 的客户端,都在
同一份中立 `Policy` 之上获得自适应限流、熔断与并发隔离,无需按客户端配 key,
也无需改代码。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4):只贡献一个 bean —— 名为 `sentinel`
的 `resilience.Driver`,由 [`cloud/governance/dataplane/outbound/resilience`](../../cloud/governance/dataplane/outbound/resilience) 收进 driver
目录 —— 也不开监听端口。`sentinel.InitDefault` 在 import 时就执行,故环境异常
在启动时立刻炸出,而不是等到第一次调用时才暴露。

[sentinel]: https://github.com/alibaba/sentinel-golang

## 安装

```bash
go get go-spring.org/starter-governance-sentinel
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-governance-sentinel"
```

`init` 会调用 `sentinel.InitDefault()`(失败即 panic),随后把后端贡献为名为
`sentinel` 的 bean 并导出为 `resilience.Driver` —— resilience manager 按名注入该 bean,不再有
任何注册或查表。

### 2. 在治理文档里选它

driver 是**全进程选一次**,不按客户端选:治理文档用 `spring.resilience.driver` 指定,每个经
治理中心解析 executor 的客户端都会拿到它。`spring.governance.*` 键写在治理文档里 —— 那是
它自己的一套系统,不是 `app.properties`(见
[`cloud/governance/README.md`](../../cloud/governance/README.md)):

```properties
spring.resilience.driver=default
spring.resilience.driver=sentinel
spring.governance.client.resilience.max-retries=3
spring.governance.client.resilience.error-threshold=10
spring.governance.client.resilience.timeout=1s
```

### 3. 或直接使用

```go
import "go-spring.org/cloud/governance/dataplane/outbound/resilience"

exec, _ := starter_governance_sentinel.NewSentinelDriver().NewExecutor("sentinel:test", resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 10,
    OpenDuration:   30 * time.Second,
    MaxRetries:     3,
    Timeout:        time.Second,
})

// 客户端传输
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, service, nil)}
```

见 [`example/`](example) 的自包含冒烟——端到端验证限流 +
熔断 + 重试的组合(无需 docker)。

## Policy 映射

中立的 `resilience.Policy` / `ServerPolicy` 经共享的 rules.go 翻译
（出站与入站引擎走同一份翻译，不会漂移）后按 service 懒加载到 sentinel 规则：

| 策略旋钮 | Sentinel 规则 | 触发时的中立 error |
| --- | --- | --- |
| `RateLimit` + `Window` | flow `Threshold`（**按窗口归一化**：每秒 → 每 StatIntervalInMs）+ `StatIntervalInMs` | `ErrRateLimited` |
| `RateLimitMaxWait` | flow `Throttling` + `MaxQueueingTimeMs`（排队而不是拒绝） | `ErrRateLimited` |
| `RateLimitWarmUpDuration` / `WarmUpColdFactor` | flow `WarmUp` + `WarmUpPeriodSec` / `WarmUpColdFactor`（预热与排队互斥，sentinel 只允许 WarmUp+Reject，预热优先） | `ErrRateLimited` |
| `breaker-strategy=error-rate` | breaker `ErrorRatio` | `ErrCircuitOpen` |
| `breaker-strategy=slow-call`（+ 慢调用对） | breaker `SlowRequestRatio` + `MaxAllowedRtMs` | `ErrCircuitOpen` |
| `breaker-strategy=error-count` | breaker `ErrorCount`（精确映射） | `ErrCircuitOpen` |
| `breaker-strategy=consecutive`（默认） | breaker `ErrorCount` —— **近似**：sentinel 没有连续失败熔断，要精确映射请写 `error-count` | `ErrCircuitOpen` |
| `HalfOpenRequests` | `ProbeNum`（半开放行试探数） | — |
| `OpenDuration` | `RetryTimeoutMs` | — |
| `MaxRetries` | 重试循环（sentinel 之外） | 最后一次尝试的 error |
| `Timeout` | ctx 截止（sentinel 之外） | `context.DeadlineExceeded` |

**合法忽略**（sentinel v1.0.4 无对应，文档注明而非伪造）：`Burst`、
`Algorithm`（flow 无滑动窗口/突发旋钮）。

sentinel 的 system / hotspot / 集群流控属 future work：它们是入站专用或参数
维度的原语，还没有 policy 字段能承载；字段有了会落进 server executor。

## Default driver

`cloud/governance/dataplane/outbound/resilience` 内置零依赖的 `default` driver,供测试与轻量场景。要在
生产链路上得到实打实的限流与熔断,请导入本 starter;若无需即可继续使用
`default`。

## 设计说明

* **本 starter 是引擎，不是缝隙。** 它不决定韧性*施加在哪里*——那是适配器的活：
  HTTP 客户端经 `resilience.NewRoundTripper` 包，
  入站准入中间件由各协议 starter 自建。单靠它本身，不会保护任何一次调用。
* **规则按 service 懒加载。** sentinel 一切以 service 名为键，故某 service 的
  策略 → 规则翻译发生在它首次 `Entry` 时，由互斥锁守护；从未被调用的 service 也
  永远不会装规则。
* **sentinel 锁死 v1.0.4。** 后续小版本调整了 flow / 熔断规则字段；锁版本让上游
  回归在此处暴露，而非波及每个适配器。
