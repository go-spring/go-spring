# starter-governance-sentinel

[English](README.md) | [中文](README_CN.md)

`starter-governance-sentinel` 把 [alibaba/sentinel-golang][sentinel] 注册为
[`cloud/governance/resilience`](../../cloud/governance/resilience) 韧性框架的生产 driver。
与 [`starter-governance`](../starter-governance) 一起空导入,并在治理文档里
写 `govern.driver=sentinel` —— 此后每个经治理中心解析 executor 的客户端,都在
同一份中立 `Policy` 之上获得自适应限流、熔断与并发隔离,无需按客户端配 key,
也无需改代码。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4):只贡献一个 bean —— 名为 `sentinel`
的 `resilience.Driver`,由 [`starter-governance`](../starter-governance) 收进 driver
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
`sentinel` 的 bean 并导出为 `resilience.Driver` —— 容器即 driver 目录,不再有
任何注册或查表。

### 2. 在治理文档里选它

driver 是**全进程选一次**,不按客户端选:治理文档用 `govern.driver` 指定,每个经
治理中心解析 executor 的客户端都会拿到它。`govern.*` 键写在治理文档里 —— 那是
它自己的一套系统,不是 `app.properties`(见
[`cloud/governance/README.md`](../../cloud/governance/README.md)):

```properties
govern.enabled=true
govern.driver=sentinel
govern.default.max-retries=3
govern.default.error-threshold=10
govern.default.attempt-timeout=1s
```

### 3. 或直接使用

```go
import "go-spring.org/cloud/governance/resilience"

exec, _ := starter_governance_sentinel.NewSentinelDriver().NewExecutor(resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 10,
    OpenDuration:   30 * time.Second,
    MaxRetries:     3,
    Timeout:        time.Second,
})

// 客户端传输
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, nil)}

// 客户端拨号
dial := resilience.NewDialer(baseDialer, exec, "upstream")
```

见 [`example/`](example) 的自包含冒烟——端到端验证 `Dialer` 以及限流 +
熔断 + 重试的组合(无需 docker)。

## Policy 映射

中立的 `resilience.Policy` 按 resource 懒加载到 sentinel 规则:

| `Policy` 字段    | Sentinel 规则          | 触发时的中立 error         |
| ---------------- | ---------------------- | -------------------------- |
| `RateLimit`      | flow(Direct/Reject)   | `ErrRateLimited`           |
| `ErrorThreshold` | circuit breaker        | `ErrCircuitOpen`           |
| `OpenDuration`   | breaker retry-after    | —                          |
| `MaxConcurrent`  | isolation              | `ErrBulkheadFull`          |
| `MaxRetries`     | 重试循环               | 最后一次尝试的 error       |
| `Timeout`        | 每次尝试的 ctx 截止    | `context.DeadlineExceeded` |

`RateLimit`、`ErrorThreshold`、`MaxConcurrent` 落成 sentinel 规则;
`MaxRetries` 与 `Timeout` 由 executor 在 sentinel entry 之外完成,因为
sentinel 本身不建模这两者。sentinel 的阻断原因会被映射为中立 sentinel,
调用方仅依赖 `cloud/governance/resilience`。

## Default driver

`cloud/governance/resilience` 内置零依赖的 `default` driver,供测试与轻量场景。要在
生产链路上得到实打实的限流与熔断,请导入本 starter;若无需即可继续使用
`default`。
