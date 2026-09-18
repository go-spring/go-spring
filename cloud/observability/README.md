# observability

[English](README.md) | [中文](README_CN.md)

`observability` carries per-request attributes on a `context.Context`, so the
spans go-spring starts **on your behalf** carry them too.

You need this because most instrumentation starts its span inside the framework.
A registry registration, a lock acquisition, a cache or DB call — each creates
its span *below* your frame and hands the span-carrying context to an inner
closure, never back to you. `trace.SpanFromContext(ctx)` on your own context
finds the **parent** span, so there is nothing to call `SetAttributes` on.

Putting the attributes on the context instead means one place covers every span
in the process:

```go
ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
```

Nested spans inherit, and on a duplicate key the later source wins. The
attributes live exactly as long as the context does — there is no cleanup to
call, because a context is never pooled for reuse the way a thread-local is.

## Install

```
go get go-spring.org/cloud
```

## Usage

Set them where the value becomes known — typically a middleware:

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

Nothing else is required. The reader is a `SpanProcessor` that `starter-otel`
registers on the provider automatically — see that starter's USAGE §2.5. If you
build your own `TracerProvider` instead, register
`trace.ContextAttributesProcessor()` yourself; without a reader the carrier is
inert.

## Two shapes that look unreachable, and are not

Both come up when deciding whether this can help at all. Neither needs the span
object, only the context it will be started from.

**A span the framework starts below your frame.** Registry registration, a lock
acquisition, a cache or DB call: you never see the span, so
`trace.SpanFromContext(ctx)` on your own context finds the parent. Annotate what
you hand in instead:

```go
ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
client.Call(ctx, ...)   // the span started inside carries tenant
```

**A layer that wraps the instrumentation.** Where a component exposes an
interceptor chain, the outer layer sits *outside* the one that starts the span —
so it holds a context without the span in it. Annotate before delegating:

```go
func (myInterceptor) Do(ctx context.Context, ...) {
	ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", t))
	return next(ctx, ...)   // the instrumented layer starts its span from this
}
```

## Scope

| Not covered | Where it belongs instead |
|---|---|
| **Metrics** — the metric SDK has no per-record hook, so built-in metric labels stay closed by design | pass attributes where you record your own instrument |
| **Process-level dimensions** (env, cluster, version) — the same for every span in the process | the OTel resource: `spring.observability.service-name`, `OTEL_RESOURCE_ATTRIBUTES` |
| **Log fields** | `log.WithFields` / `log.Collector` in the log module |
