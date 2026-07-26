# resilience
[English](README.md) | [中文](README_CN.md)

`resilience` 是零依赖、框架无关的客户端容错抽象:限流、熔断、bulkhead 隔离、
重试、每次尝试超时、降级。客户端 starter 把单一 `Executor` seam 插入自家请求
钩子(HTTP RoundTripper / Redis Hook / GORM plugin ...);分布式限流走并列的
`RateLimiter` seam。

## 特性

- `Policy` 字段:`RateLimit` / `Burst`、`ErrorThreshold` / `OpenDuration`、
  `MaxConcurrent`、`MaxRetries`、`Timeout`。
- 中立拒绝错误:`ErrRateLimited`、`ErrCircuitOpen`、`ErrBulkheadFull`。
- 内置 `"default"` 驱动 —— 进程内、零依赖。推荐的生产驱动 `sentinel` 在
  `starter/starter-resilience`。
- 三个可 opt-in 的适配 seam:
  - `NewRoundTripper` —— HTTP client `http.RoundTripper`(覆盖面最广)。
  - `NewDialer` —— 连接级 `DialFunc`,匹配
    `discovery.LiveDialer.DialContext`。
  - `NewHandler` —— 入站 HTTP admission;拒绝时 429 / 503。
- `Fallback(ctx, exec, resource, fn, degrade)` —— 组合任意 executor 的降级
  helper。
- 独立 `RateLimiter` + `LimiterDriver` 注册表(内置 token bucket / sliding
  window;`starter-go-redis` 提供 Redis 全局共享令牌桶)。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

包一个 HTTP client:

```go
import (
    "net/http"

    "go-spring.org/spring/cloud/resilience"
)

drv, _ := resilience.MustGetDriver("default")
exec, _ := drv.NewExecutor(resilience.Policy{
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

与 `spring/discovery` 在拨号层组合:

```go
ld, _ := discovery.NewClientDialer(ctx, "default", "orders")
dial  := resilience.NewDialer(ld.DialContext, exec, "orders")
```

对入站请求限流:

```go
handler := resilience.NewHandler(mux, exec, func(r *http.Request) string { return r.URL.Path })
```

独立 RateLimiter(全局分布式配额靠 starter 提供驱动):

```go
ldrv, _ := resilience.MustGetLimiter("default") // 或 starter-go-redis 的 "redis"
limiter, _ := ldrv.NewRateLimiter(resilience.LimitPolicy{Rate: 100, Burst: 100})
if ok, _ := limiter.Allow(ctx, "tenant:42"); !ok { /* reject */ }
```
