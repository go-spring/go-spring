# observability

[English](README.md) | [中文](README_CN.md)

`observability` 把每请求的属性挂在 `context.Context` 上,让 go-spring **代你启动**的那些 span
也能带上它们。

需要它,是因为绝大多数插桩的 span 都在框架内部创建。注册中心注册、加锁、缓存或 DB 访问——
这些 span 都在**你的 frame 之下**创建,带 span 的 context 只交给内部闭包,从不回传给你。
你在自己的 context 上调 `trace.SpanFromContext(ctx)` 拿到的是**父** span,根本没有
`SetAttributes` 的对象。

把属性挂到 context 上,一处代码即可覆盖进程内的每一个 span:

```go
ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
```

子 span 自动继承;同名字段后者胜。属性的生命周期与 context 完全一致——没有清理 API 需要调用,
因为 context 不像 thread-local 那样会被复用。

## 安装

```
go get go-spring.org/cloud
```

## 用法

在值已知的地方设置,通常是中间件:

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

到此为止,不需要再做别的。读方是一个 `SpanProcessor`,由 `starter-otel` 自动注册到 provider 上——
见该 starter 的 USAGE §2.5。如果你是**自己**构造 `TracerProvider`,就要自己注册
`trace.ContextAttributesProcessor()`;没有读方,载体是死的。

## 两种「看起来够不到」、其实够得到的形状

判断这套机制能不能帮上忙时,这两种情况最常被想到。**两者都不需要 span 对象,只需要它将要由之启动的那个 context。**

**span 由框架在你的 frame 之下启动。** 注册中心注册、加锁、缓存或 DB 访问:你根本看不到那个 span,
在自己的 context 上调 `trace.SpanFromContext(ctx)` 得到的是父 span。标注你**传进去**的那个即可:

```go
ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
client.Call(ctx, ...)   // 内部启动的 span 带上 tenant
```

**有一层包住了插桩。** 组件暴露拦截器链时,外层位于「启动 span 的那层」**之外**,
因此它持有的 context 里没有 span。**在委派之前标注**即可:

```go
func (myInterceptor) Do(ctx context.Context, ...) {
	ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
	return next(ctx, ...)   // 被插桩的那层用这个 ctx 启动它的 span
}
```

## 适用边界

| 不覆盖 | 应去哪里 |
|---|---|
| **metric**——metric SDK 没有记录时的钩子,内置 metric 标签按设计保持封闭 | 给你自己的 instrument 在记录处传属性 |
| **进程级维度**(env、cluster、version)——进程内每个 span 都一样 | OTel resource:`spring.observability.service-name`、`OTEL_RESOURCE_ATTRIBUTES` |
| **日志字段** | log 模块的 `log.WithFields` / `log.Collector` |
