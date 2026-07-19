# aspect
[English](README.md) | [中文](README_CN.md)

`aspect` 是 Spring AOP 的 Go 惯用法等价:通过显式、类型安全的拦截器链加装饰器
约定,配合已有的 DI 容器来达到同等效果。可以让横切关注点(事务、缓存、计时、
panic 恢复、审计)一次声明、多处应用,而不侵入业务代码 —— 不做字节码织入、
不做运行时动态代理。

## 特性

- `Chain` 由若干 `Interceptor` 包裹目标函数;index 0 是最外层。nil 或空链是
  透明透传。
- `Joinpoint{Context, Method, Result, Proceed}` —— 拦截器调用 `Proceed` 继续
  链,或直接返回结果短路。
- `Around[T]` 在调用点还原静态类型,全程零反射。
- 内置拦截器:`Recover`、`Timing`、`Cache`(可插拔 `Store`,自带零依赖
  `MemoryStore`)、`Transactional`(可插拔 `TxManager`)、`Only`(按方法名切点)。
- `NewHandler` 把 `http.Handler` 包成一次请求即一个 joinpoint 的形式;5xx 会
  被上报为链错误。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

```go
import (
    "context"

    "go-spring.org/stdlib/aspect"
)

type OrderService interface {
    Place(ctx context.Context, o *Order) error
}

// 业务实现。
type orderService struct{ /* deps */ }

func (s *orderService) Place(ctx context.Context, o *Order) error { /* ... */ return nil }

// 装饰器与业务实现共享接口,调用方看不到链。
type orderServiceAspect struct {
    inner OrderService
    chain *aspect.Chain
}

func (a *orderServiceAspect) Place(ctx context.Context, o *Order) error {
    return a.chain.RunE(ctx, "OrderService.Place",
        func(ctx context.Context) error { return a.inner.Place(ctx, o) })
}

func newChain(tm aspect.TxManager) *aspect.Chain {
    return aspect.NewChain(
        aspect.Recover(),
        aspect.Timing(func(m string, d time.Duration, err error) { /* metric */ }),
        aspect.Transactional(tm),
    )
}
```

有强类型返回值时用 `Around`:

```go
order, err := aspect.Around(chain, ctx, "PlaceOrder",
    func(ctx context.Context) (*Order, error) { return repo.Insert(ctx, o) })
```

给 HTTP server 加横切时,直接包 mux,不用另写装饰器:

```go
h := aspect.NewHandler(mux, chain, func(r *http.Request) string { return r.URL.Path })
```
