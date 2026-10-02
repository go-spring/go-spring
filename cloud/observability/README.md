# observability

[English](README.md) | [中文](README_CN.md)

`observability` carries per-request attributes on a `context.Context`, so the
spans go-spring starts **on your behalf** carry them too. It is also where client
starters declare what a call is (`Operation`) and accumulate its attempts
(`Recorder`), for the `resilience` emitter to read.

## Why you need it

Most instrumentation starts its span inside the framework.
A registry registration, a lock acquisition, a cache or DB call — each creates
its span *below* your frame and hands the span-carrying context to an inner
closure, never back to you. `trace.SpanFromContext(ctx)` on your own context
finds the **parent** span, so there is nothing to call `SetAttributes` on.

Putting the attributes on the context instead means one place covers every span
in the process:

```go
ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
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
		ctx := observability.WithSpanAttributes(r.Context(),
			attribute.String("tenant", tenantOf(r)),
		)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

Nothing else is required. The reader is a `SpanProcessor` that `starter-otel`
registers on the provider automatically — see that starter's USAGE §2.5. If you
build your own `TracerProvider` instead, register
`observability.SpanAttributesProcessor()` yourself; without a reader the carrier
is inert.

## The span is a window

A span observes a process, not an instant: it is started before the work and
settled after it. That lifetime is what sets it apart from the other signals
here — it stays writable while the work runs, and it can be addressed from the
context the work carries. Three moments can add to it, and a fourth cannot:

| Moment | How |
|---|---|
| at `Start` | the static attributes the component passes to `Tracer.Start` |
| before `Start`, from a caller outside the frame | `observability.WithSpanAttributes`, applied by the processor as the span starts |
| while the span runs | `observability.SetSpanAttributes(ctx, ...)`, by whoever holds the span-carrying context |
| after `End` | not at all — `OnEnd` is handed a read-only span |

The third row is what a layer wrapping the instrumentation uses. A context is
derived and immutable, so `trace.SpanFromContext(ctx)` resolves to this
operation's span and no other: what such a layer writes cannot land on a
different call, and the instrumented component never has to know the layer
exists.

Two limits come with that design. The window is only as wide as context
propagation — a goroutine started from `context.Background()`, or a client that
ignores its ctx, leaves the span behind. And the window shuts at `End`: what the
caller learns only after the call returns cannot be added to the span it came
from.

A metric has no such window. It is a single measurement, not a process, so every
label is supplied at the moment it is recorded.

## Two shapes that look unreachable, and are not

Both are the `before Start` row above: neither needs the span object, only the
context it will be started from.

**A span the framework starts below your frame.** Registry registration, a lock
acquisition, a cache or DB call: you never see the span, so
`trace.SpanFromContext(ctx)` on your own context finds the parent. Annotate what
you hand in instead:

```go
ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
client.Call(ctx, ...)   // the span started inside carries tenant
```

**A layer that wraps the instrumentation.** Where a component exposes an
interceptor chain, the outer layer sits *outside* the one that starts the span —
so it holds a context without the span in it. Annotate before delegating:

```go
func (myInterceptor) Do(ctx context.Context, ...) {
	ctx = observability.WithSpanAttributes(ctx, attribute.String("tenant", t))
	return next(ctx, ...)   // the instrumented layer starts its span from this
}
```

## RefreshConf: the property-refresh funnel

Every config backend (nacos, etcd, consul, vault, k8s, file, ...) fires the
same application-wide property refresh when its watch reports a change.
`RefreshConf` is the shared funnel for those triggers: it runs the refresh
once and records the outcome — `config.refresh.total` by exclusive `status`,
`config.refresh.duration`, `config.refresh.last_success_timestamp`, and the
one log line for a fleet-wide event. Callers log only their backend events.

```go
_ = observability.RefreshConf(ctx, gs.RefreshProperties)
```

The refresh function is passed in (not referenced), so the package stays
spring-free; fn's error is returned unchanged — this is instrumentation, not
error policy.

## Operation and Recorder: what client starters declare

Clients do not emit — they **declare**, and the emitter is a single place:
`resilience`'s wrapped client executor. Two primitives riding the context carry
that declaration and its result.

`Operation` says what the call is. Set it on the **caller's** context, outside
the executor: the emitter reads it at `Execute` entry, so one set inside the
wrapped function is never seen.

```go
ctx = observability.WithOperation(ctx, observability.Operation{
    Name:   "get",                   // span name
    Metric: "db.client",             // metric-name prefix; empty panics
    Attrs:  []attribute.KeyValue{…}, // bounded → metric labels + span + log
    Detail: []attribute.KeyValue{…}, // possibly unbounded → span + log only

    LogTag: accessTag,               // the starter's own access-log tag
})
```

`Attrs` must stay bounded: a cache key, a SQL statement, a URL path or a topic
belongs in `Detail`, which never becomes a metric label.

`Recorder` is the attempt-level accumulator. The governance executor appends one
entry per downstream attempt; the emitter drains it after the retry loop.

```go
rec := observability.RecorderFrom(ctx)   // nil-safe — no recorder is fine
rec.AddAttempt(d, status, err)
```

`WithRecorder` is deliberately **not idempotent**: the most recent recorder on
the chain wins, so two nested executors each keep their own. Read it once at the
loop entry and keep the pointer — a per-write lookup would hand an inner
executor the outer call's records.

## What it does not cover

| Not covered | Where it belongs instead |
|---|---|
| **Context-carrier attributes on metrics** — the metric SDK has no per-record hook, so built-in metric labels stay closed by design | pass attributes where you record your own instrument |
| **Process-level dimensions** (env, cluster, version) — the same for every span in the process | the OTel resource: `spring.observability.service-name`, `OTEL_RESOURCE_ATTRIBUTES` |
| **Log fields** | `log.WithFields` / `log.Collector` in the log module |
