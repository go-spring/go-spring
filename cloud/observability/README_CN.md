# observability

[English](README.md) | [中文](README_CN.md)

`observability` 把每请求的属性（如租户、压测标记）挂在 `context.Context`
上，让框架**替你创建**的那些 span 也能带上它们。客户端 starter 也在这里声明
一次调用是什么（`Operation`）、累积它的尝试记录（`Recorder`），供 `resilience`
的发射端读取。

## 为什么需要它

go-spring 绝大多数插桩的 span 都在框架内部创建：注册中心的注册、加锁、
缓存和 DB 访问，各自在内部闭包里启动 span、记录完就结束。这些 span 从不
暴露给调用方，你拿到自己的 ctx 时，`trace.SpanFromContext(ctx)` 返回的是
**父** span——不是正在记录的那个，也没有任何 API 允许你给它
`SetAttributes`。

所以属性不从 span 传入，而从 context 传入：挂到 context 上，之后凡是由
这个 context 启动的 span——无论在框架哪一层——都能读到：

```go
ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
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
        ctx := observability.WithSpanAttributes(r.Context(),
            attribute.String("tenant", tenantOf(r)),
        )
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

读方是一个 `SpanProcessor`，由 starter-otel 自动注册到 provider
（见该 starter 的 USAGE §2.5）。自己构造 `TracerProvider` 的用户需要手动
注册 `observability.SpanAttributesProcessor()`——没有这个读方，挂上去的属性
不会被任何 span 采集。

## span 是一个窗口

span 观测的是一个**过程**，不是某个瞬时：它在工作开始前启动、在工作结束后
结算。正是这个生命周期让它区别于这里的其他信号——工作运行期间它一直可写，
而且可以通过工作所携带的 context 寻址。属性可以在三个时刻进入，第四个不行：

| 时刻 | 方式 |
|---|---|
| `Start` 时 | 组件传给 `Tracer.Start` 的静态属性 |
| `Start` 之前（来自框架之外的调用方） | `observability.WithSpanAttributes`，由 processor 在 span 启动时应用 |
| span 运行期间 | `observability.SetSpanAttributes(ctx, ...)`，由持有该 span context 的一方调用 |
| `End` 之后 | 没有——`OnEnd` 拿到的是只读 span |

第三行是包裹插桩的那一层用的。context 是派生且不可变的，所以
`trace.SpanFromContext(ctx)` 只会解析到本次操作的 span，不会是别的：这一层写
进去的内容不可能落到另一次调用上，被插桩的组件也无需知道这一层存在。

这个设计带着两个限制。窗口的宽度等于 context 传播的宽度——从
`context.Background()` 起的 goroutine、或不认 ctx 的客户端，都会把 span 落在
身后。窗口在 `End` 关闭：调用返回之后才知道的信息，再也加不到它对应的 span
上。

metric 没有这样的窗口。它是一次测量而不是一个过程，所以每个标签都只能在记录
的那一刻交上来。

## 两个典型场景

两种都是上面表格里"`Start` 之前"那一行的情况：都**不需要拿到 span 对象**，
只需要 span 将要从中启动的那个 context。

**span 在框架内部启动。** 注册中心的注册、加锁、缓存或 DB 访问，你在自己的
ctx 上看不到这些 span，`trace.SpanFromContext(ctx)` 返回的是父 span。标注你
传进去的 ctx 即可：

```go
ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
client.Call(ctx, ...)   // 内部启动的 span 带上 tenant
```

**拦截器位于插桩之外。** 组件暴露拦截器链时，外层拦截器在"启动 span 的
那一层"外面执行，它手里的 ctx 上还没有 span。在委派给下一层之前标注：

```go
func (myInterceptor) Do(ctx context.Context, ...) {
    ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
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

## Operation 与 Recorder：客户端 starter 的"声明"接口

客户端不发射信号——它们**声明**，发射端只有一处：`resilience` 的 wrapped
client executor。context 上承载这份声明及其结果的是两个类型。

`Operation` 说明这次调用是什么。它必须挂在**调用方**的 context 上、位于
executor 之外：发射端在 `Execute` 入口读取，写在被包裹函数里的声明没人读。

```go
ctx = observability.WithOperation(ctx, observability.Operation{
    Name:   "get",                   // span 名
    Metric: "db.client",             // 指标名前缀——为空会 panic
    Attrs:  []attribute.KeyValue{…}, // 有界 → 指标标签 + span + 日志
    Detail: []attribute.KeyValue{…}, // 可能无界 → 仅 span + 日志

    LogTag: accessTag,               // starter 自己的访问日志 tag
})
```

`Attrs` 必须保持有界：缓存 key、SQL 语句、URL 路径、topic 一律放 `Detail`，
它永远不会变成指标标签。

`Recorder` 是尝试级累加器：治理 executor 为每次下游尝试追加一条，发射端在
重试循环之外把它读走。

```go
rec := observability.RecorderFrom(ctx)   // nil 安全——没有 recorder 也正常
rec.AddAttempt(d, status, err)
```

`WithRecorder` 刻意**不幂等**：链上最近的一个胜出，嵌套的两层 executor 各写
各的。在循环入口读一次并持有指针——每次写入都重新查会把手外层调用的记录
交给内层 executor。

## 不覆盖什么

| 不覆盖 | 应去哪里 |
|---|---|
| **context 属性进 metric**——metric SDK 没有记录时的钩子，内置 metric 标签按设计保持封闭 | 给你自己的 instrument 在记录处传属性 |
| **进程级维度**（env、cluster、version）——进程内每个 span 都一样 | OTel resource：`spring.observability.service-name`、`OTEL_RESOURCE_ATTRIBUTES` |
| **日志字段** | log 模块的 `log.WithFields` / `log.Collector` |
