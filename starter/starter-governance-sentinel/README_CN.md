# starter-governance-sentinel

[English](README.md) | [中文](README_CN.md)

`starter-governance-sentinel` 让 [alibaba/sentinel-golang][sentinel] 回答出站
resilience 引擎的**限流决策**。

它贡献一个 `ratelimit.Provider` bean，导出算法名 `sentinel`。治理文档用
`limiter: sentinel` 选中它，和内置的 `token-bucket` 完全同构；timeout、retry、
熔断、规则匹配、per-rule key 和指标仍归 resilience 引擎所有——没有第二套 executor，
也没有重复的管道。

[sentinel]: https://github.com/alibaba/sentinel-golang

## 安装

```bash
go get go-spring.org/starter-governance-sentinel
```

## 快速开始

```go
import _ "go-spring.org/starter-governance-sentinel"
```

导入即调用 `sentinel.InitDefault()`(失败 panic)，并把 provider 注册进进程的 limiter factory。

在治理文档里选中它：

```properties
# governance document
spring.governance.rate-limit.services[0].service=orders
spring.governance.rate-limit.services[0].limiter=sentinel
spring.governance.rate-limit.services[0].rate-limit=100
```

或程序化使用：

```go
import (
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	StarterGovernanceSentinel "go-spring.org/starter-governance-sentinel"
)

fact, _ := ratelimit.NewFactory([]ratelimit.Provider{
	ratelimit.NewLocalProvider(),
	StarterGovernanceSentinel.Provider{},
})
mgr := resilience.NewManager(fact)
_ = mgr.ApplyRateLimit(resilience.RateLimitSettings{Default: ratelimit.Policy{
	Limiter:   StarterGovernanceSentinel.LimiterAlgorithm,
	RateLimit: 100,
}})
exec := mgr.ExecutorFor("orders")

client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, "orders", nil)}
```

没有任何 provider 导出的算法名会在文档校验时被拒绝，拼错不会拖到第一次调用才暴露。

## 策略映射

| 中立旋钮 | Sentinel flow 规则 |
| --- | --- |
| `RateLimit` + `Window` | `Threshold` 按 `StatIntervalInMs`(每秒归一化到每窗口) |
| `MaxWait` | `Throttling` + `MaxQueueingTimeMs`(排队而非拒绝) |
| `WarmUp` / `ColdFactor` | `WarmUp` token 策略；预热优先于排队(sentinel 只允许 WarmUp+Reject) |

映射不了的旋钮不伪造：`Burst` 在 sentinel 没有对应项；规则的算法名是 `sentinel`，
不再是 `token-bucket`。

## 边界

- **只做限流。** timeout、retry、熔断、按 service/action/metadata 的规则匹配、
  指标与状态面全部留在 resilience 引擎。
- **没有全局选择。** provider 按规则里的算法名选中；没有 `spring.resilience.driver`
  键，也没有后端 bean。
- **per-rule 预算。** 引擎把规则身份作为计数 scope，所以同一 service 的两条规则各自
  独立计桶(默认规则的 scope 就是 service label)。
- **sentinel v1.0.4 固定版本。** 后续 minor 改过规则结构字段，这个 pin 保持翻译稳定。

见 [`example/`](example) 的自包含冒烟(无需 docker)。
