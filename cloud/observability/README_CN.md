# observability

[English](README.md) | [中文](README_CN.md)

`observability` 把每请求的属性（如租户、压测标记）挂在 `context.Context`
上，让框架**替你创建**的那些 span 也能带上它们。

## 为什么需要它

go-spring 绝大多数插桩的 span 都在框架内部创建：注册中心的注册、加锁、
缓存和 DB 访问，各自在内部闭包里启动 span、记录完就结束。这些 span 从不
暴露给调用方，你拿到自己的 ctx 时，`trace.SpanFromContext(ctx)` 返回的是
**父** span——不是正在记录的那个，也没有任何 API 允许你给它
`SetAttributes`。

所以属性不从 span 传入，而从 context 传入：挂到 context 上，之后凡是由
这个 context 启动的 span——无论在框架哪一层——都能读到：

```go
ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
```

子 span 自动继承；同名属性后设置的胜出。属性生命周期与 context 完全
一致，context 用完即弃，没有清理 API。

## 安装

```
go get go-spring.org/cloud
```

## 用法

在值已知的地方标注，通常是中间件：

```go
func TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := observability.WithContextAttributes(r.Context(),
            attribute.String("tenant", tenantOf(r)),
        )
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

读方是一个 `SpanProcessor`，由 starter-otel 自动注册到 provider
（见该 starter 的 USAGE §2.5）。自己构造 `TracerProvider` 的用户需要手动
注册 `trace.ContextAttributesProcessor()`——没有这个读方，挂上去的属性
不会被任何 span 采集。

## 两个典型场景

两种情况都**不需要拿到 span 对象**，只需要 span 将要从中启动的那个
context。

**span 在框架内部启动。** 如 `client.Call(ctx, ...)` 内部的缓存或 DB 访问
span，你在 ctx 上看不到它们。标注你传进去的 ctx 即可：

```go
ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
client.Call(ctx, ...)   // 内部启动的 span 带上 tenant
```

**拦截器位于插桩之外。** 组件暴露拦截器链时，外层拦截器在"启动 span 的
那一层"外面执行，它手里的 ctx 上还没有 span。在委派给下一层之前标注：

```go
func (myInterceptor) Do(ctx context.Context, ...) {
    ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
    return next(ctx, ...)   // 下一层用这个 ctx 启动 span
}
```

## RefreshConf：属性刷新漏斗

每个配置后端（nacos、etcd、consul、vault、k8s、file……）在 watch 到变更后
触发的都是同一个应用级属性刷新。`RefreshConf` 就是这些触发点的共享漏斗：
执行一次刷新并记录结果——按互斥 `status` 统计的 `config.refresh.total`、
`config.refresh.duration`、`config.refresh.last_success_timestamp`，以及
这个全局事件应有的那一条日志。调用方只记自己的后端事件。

```go
_ = observability.RefreshConf(ctx, gs.RefreshProperties)
```

刷新函数由调用方传入（而非包内引用），因此本包不依赖 spring；fn 的错误
原样返回——这里是插桩，不是错误策略。

## 不覆盖什么

| 需求 | 应去哪里 |
|---|---|
| **context 属性进 metric**——metric SDK 没有记录时的钩子，内置 metric 标签按设计保持封闭 | 给你自己的 instrument 在记录处传属性 |
| **进程级维度**（env、cluster、version）——进程内每个 span 都一样 | OTel resource：`spring.observability.service-name`、`OTEL_RESOURCE_ATTRIBUTES` |
| **日志字段** | log 模块的 `log.WithFields` / `log.Collector` |
