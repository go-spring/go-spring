# starter-resilience

[English](README.md) | [中文](README_CN.md)

`starter-resilience` 把 [alibaba/sentinel-golang][sentinel] 注册为
[`spring/resilience`](../../spring/resilience) 韧性框架的推荐 driver。空导入
后,任何选择 `driver=sentinel` 的 starter 或用户代码——HTTP RoundTripper、
Dialer、入站 Handler、重试策略——都能在同一份中立 `Policy` 之上获得自适应
限流、熔断与并发隔离。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4):不注册 bean,也不开监听端口。
`sentinel.InitDefault` 在 import 时就执行,故环境异常在启动时立刻炸出,
而不是等到第一次调用时才暴露。

[sentinel]: https://github.com/alibaba/sentinel-golang

## 安装

```bash
go get go-spring.org/starter-resilience
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-resilience"
```

`init` 会调用 `sentinel.InitDefault()`(失败即 panic),随后执行
`resilience.RegisterDriver("sentinel", ...)`。

### 2. 让上层适配器指向 sentinel driver

一切构筑在 `spring/resilience` 上的 starter/库都按名字选择 driver。以
`starter-oauth2-client` 为例,它读取
`spring.http.client.<name>.resilience.driver`:

```properties
spring.http.client.default.resilience.driver=sentinel
spring.http.client.default.resilience.rate-limit=100
spring.http.client.default.resilience.error-threshold=10
spring.http.client.default.resilience.open-duration=30s
spring.http.client.default.resilience.max-retries=3
spring.http.client.default.resilience.timeout=1s
```

### 3. 或直接使用

```go
import "go-spring.org/spring/resilience"

driver, _ := resilience.MustGetDriver("sentinel")
exec, _ := driver.NewExecutor(resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 10,
    OpenDuration:   30 * time.Second,
    MaxRetries:     3,
    Timeout:        time.Second,
})

// 服务端准入
handler := resilience.NewHandler(mux, exec, func(*http.Request) string { return "hello" })

// 客户端传输
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, nil)}

// 客户端拨号
dial := resilience.NewDialer(baseDialer, exec, "upstream")
```

见 [`example/`](example) 的自包含冒烟——端到端验证 `Handler`、`Dialer`
以及限流 + 熔断 + 重试的组合(无需 docker)。

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
调用方仅依赖 `spring/resilience`。

## Default driver

`spring/resilience` 内置零依赖的 `default` driver,供测试与轻量场景。要在
生产链路上得到实打实的限流与熔断,请导入本 starter;若无需即可继续使用
`default`。
