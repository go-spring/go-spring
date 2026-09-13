# resilience
[English](README.md) | [中文](README_CN.md)

`resilience` 是框架无关的客户端容错抽象:限流、熔断、bulkhead 隔离、
重试、每次尝试超时、降级。客户端 starter 把单一 `Executor` seam 插入自家请求
钩子(HTTP RoundTripper / Redis Hook / GORM plugin ...);分布式限流走并列的
`RateLimiter` seam。

## 特性

- `Policy` 字段:`RateLimit` / `Burst`、`ErrorThreshold` / `OpenDuration`、
  `MaxConcurrent`、`MaxRetries`、`Timeout`。
- 中立拒绝错误:`ErrRateLimited`、`ErrCircuitOpen`、`ErrBulkheadFull`。
- 内置 `"default"` 驱动 —— 进程内、零依赖。推荐的生产驱动 `sentinel` 在
  `starter/starter-governance-sentinel`。
- 两个客户端适配 seam:
  - `NewRoundTripper` —— HTTP client `http.RoundTripper`(覆盖面最广)。
  - `NewDialer` —— 连接级 `DialFunc`,匹配
    a dial closure over a round-robin pick pool。
- 入站 admission 不在本包:各协议 starter 用 `resilience.ExecutorFor` seam
  自建中间件(拒绝时 429 / 503;见 starter-gin / starter-grpc 的 admission)。
- `Fallback(ctx, exec, resource, fn, degrade)` —— 组合任意 executor 的降级
  helper。
- 独立 `RateLimiter` + `LimiterDriver` seam(内置 token bucket / sliding
  window;`starter-go-redis` 提供 Redis 全局共享令牌桶)。

## 可插拔后端

本包**不带任何注册表**。后端是以自身名字贡献、导出为 `Driver`(或
`LimiterDriver`)的 bean;消费方把它们收成按名字索引的目录,再用
`resilience.Resolve` 解析配置里的名字。内置后端无需 bean 即应答 `"default"`,
经 `NewDefaultDriver()` / `NewDefaultLimiterDriver()` 取得。

```go
// 贡献一个后端(通常在 starter 的 init 里)
gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
    Name("sentinel").
    Export(gs.As[resilience.Driver]())
```

`Export` 是承重的:gs 按精确类型索引 bean,缺了它具体的 driver 对目录不可见。

> **破坏性变更(2026-09-13)。** 包级注册表已删除:`RegisterDriver`/
> `GetDriver`/`NewExecutor(name, policy)`/`RegisterLimiter`/`GetLimiter`。
> 迁移方式 —— `GetDriver("x")` → 名为 `"x"` 的 bean(经
> `resilience.Resolve` 在注入的目录里解析);`NewExecutor("default", p)` →
> `NewDefaultDriver().NewExecutor(p)`;`RegisterDriver("x", d)` → 按上面的
> 方式贡献 bean。`starter-go-redis/experimental` 的
> `RegisterLimiterDriver(name, client)` 改为 `NewLimiterDriver(client)`
> (把返回值贡献为 bean)。`govern.driver` 及所有配置 key 不变。

## 安装

```
go get go-spring.org/cloud
```

## 用法

包一个 HTTP client:

```go
import (
    "net/http"

    "go-spring.org/cloud/governance/resilience"
)

exec, _ := resilience.NewDefaultDriver().NewExecutor(resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 5,
    MaxRetries:     2,
    Timeout:        2 * time.Second,
})

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec,
        func(r *http.Request) string { return r.URL.Host }),
}
```

与 `cloud/discovery` 在拨号层组合:

```go
ld, _ := discovery.NewClientDialer(ctx, "default", "orders")
dial  := resilience.NewDialer(ld.DialContext, exec, "orders")
```

独立 RateLimiter(全局分布式配额靠 starter 提供驱动):

```go
limiter, _ := resilience.NewDefaultLimiterDriver().NewRateLimiter(
    resilience.LimitPolicy{Rate: 100, Burst: 100})
if ok, _ := limiter.Allow(ctx, "tenant:42"); !ok { /* reject */ }
```
