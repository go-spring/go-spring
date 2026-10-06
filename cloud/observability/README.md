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

## Metrics

A metric answers "how many / how long / how is it now". A node needs one when
someone watches it for a trend, a rate, or an alert. Three shapes cover that, and
the shape decides the name:

| Shape | When | Name |
|---|---|---|
| **operation** | a per-request / per-message cross-boundary action | `<family>.operation.total` + `<family>.operation.duration`, with an exclusive `status` axis (summing over `status` gives the operation count) |
| **event** | a low-frequency, semantically significant state change | a dedicated counter named after the event (`lock.lost.total`, `loadbalance.endpoint.suspension.total`) - not the total/duration template |
| **state** | "how is it now", read continuously | a gauge (`messaging.operation.active`, `loadbalance.endpoint.suspended`) |

Conventions that keep the vocabulary joinable:

- **Unbounded values never become metric attributes.** A key, an address, a
  destination - they go on the span or the log, never on a label. Metric
  attributes are **closed** by default; opening one requires a cardinality guard
  (Micrometer's `maximumAllowableTags` is the reference). A purely low-frequency
  config change (a governance policy applied) is served by a log alone - no
  metric is forced.
- **Name the capability, not the implementation.** Components of one kind share a
  metric name and shape; the implementation is a dimension value (`db.system`,
  `messaging.system`). More detail means a new field, not a new name.
- **The status axis is `status`** (`ok` / `error` / …), never `outcome` /
  `result`. The one exception is `config-bus`'s `outcome`, a message-disposition
  axis (`malformed` / `ignored` / `refreshed`) with a different semantic.
  (2026-09-22)
- **The duration noun root is always `duration`** (`<family>.operation.duration`),
  with unit `WithUnit("s")`; a log's duration field uses `duration_ms` (same root
  plus `_ms`). `scheduling.lag` is a queue backlog, not an operation duration, and
  is not a violation. Seeing `cost` / `latency` / `elapsed` / `outcome` is a
  signal to fix. (2026-09-22)
- **The duration histogram bucket boundaries have exactly one definition**: the
  unexported `durationBuckets` plus the exported `DurationBuckets()` (which
  returns a clone), both in `cloud/observability/buckets.go`. A call site writes
  `metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...)`. A
  domain that needs its own bounds defines them **locally** (`cloud/scheduling`'s
  `lagBuckets`), never by exporting the variable - its elements would then be
  mutable in place. (2026-09-30)
- **An instrument is created at wiring / construction time by default, never in a
  package-level `init`** - the OTel global meter only delegates to the first
  provider, so an `init`-time instrument binds to noop forever. The cloud-layer
  idiom is a package-level `sync.OnceValue` resolved on first use.
- **Multi-instance instrumentation registers at `Meter.RegisterCallback` and
  unregisters with `Unregister`**, not via a create-time callback
  (`WithInt64Callback`) - the second instance's callback is silently dropped.
  Per-instance means lifetime: the instance needs a `Close` / `Destroy` hook, or
  the callback leaks and duplicate series appear (which Prometheus rejects
  outright). A nil observer follows the module's passthrough convention rather
  than panicking, and `Close()` on nil is a no-op. (2026-09-30)

One sanctioned exception: `RefreshConf` builds its instrument at the top of each
`Run`, because it must come after starter-otel installs the global provider and
`Run` is low-frequency. The general rule is that a bean carries only what the
container must arbitrate; a read-only process-level seam like `otel.Meter()` is
not a global escape.

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
_ = observability.RefreshConf(ctx, func(ctx context.Context) error {
	return gs.RefreshProperties(ctx)
})
```

`ctx` is the trigger's own context and the only carrier of identity: the fields
the path was given at its head — the backend's coordinates, the path's
`trace_id` — are what the round's records and logs show. The funnel adds none of
its own. The refresh function is passed in (not referenced), so the package
stays spring-free; fn's error is returned unchanged — this is instrumentation,
not error policy.

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

## Calling the tracer

- **Call `otel.Tracer` / `otel.Meter` at the use site** (`otel.Tracer(name).Start(...)`); do not
  cache them in a package-level `var`. A tracer captured at init no longer forwards to a new
  provider after one is set / restored / set again, so its spans are all lost. (2026-09-24)
- **A helper that starts a span hands the span-carrying context back to its caller**, or the log
  lines written right next to the call cannot join the span just created.
- **The instrumentation seam is callback-shaped.**
  `Observer.RegisterAttempt(ctx, service, reason string, fn func(ctx context.Context) error) error`
  takes its `system` at `NewObserver(system, ...)` construction (not on each call), hands `fn` the
  span context, and treats `fn`'s error as the reported result. It does **not** return
  `(ctx, finisher)`: that shape lets callers write `_, attempt :=` and drop the context. Seeing a
  `(context.Context, func(error))` return shape is the anti-pattern signal. The observed operation
  must genuinely take a context; a library that cannot gets a `func(context.Context) error`
  placeholder. (2026-09-17)

## Where instrumentation lives

- **The capability lives in `cloud`; a starter only controls whether it is enabled.** A starter
  answers "is it on" and knows nothing about how the signals are produced.
- **A new component plugs into observation through the local instrumentation idiom** - the templates
  are `cloud/lock/observe.go` and `starter/starter-redigo/observe.go`. That idiom is: a static
  package tag via `log.RegisterAppTag("<system>", "access")`; access-log levels with log's native
  semantics (error `Warn` / success-with-fields `Debug`, whose signature is the lazy
  `func() []Field` / plain `Info`, with silence meaning the tag's logger level is configured off);
  span attributes and metric names following the OTel `db.*` / `messaging.*` conventions.
  (2026-09-05)
- **Same-kind components share one observation vocabulary** - same name, same shape, same value
  wordlist. Component-specific fields may differ (the completeness / commonality / flexibility
  principles). A missing signal, or a missing `status` (so success cannot be told from failure), is
  a defect, not a style difference. Ask: if the backend is swapped, do the operational assets still
  read? (2026-09-18)
- **Backend-native OTel instrumentation is a legitimate signal source.** Elasticsearch's
  `elastictransport`, go-redis's `redisotel` and kafka's `kotel` are fine as-is - do not add a second
  layer on top of them, and do not call them violations for not calling `otel.Tracer(` themselves.
- **The `observe` suite has been removed** (`cloud/observe` was deleted whole); each domain builds
  its own instrumentation, and `DurationBuckets` is the one piece unified in `cloud/observability`.
- **Process-level instrument state need not be global.** A multi-instance component uses a
  per-instance `Observer` (built at block construction, `Close()`d at teardown, state held in
  instance fields) - template `cloud/discovery.Observer`. (2026-09-30)

## Extending observability

- **Built on go-spring's `log` and OTel's spans and metrics; there is no bespoke API.** Unification
  happens at the two existing layers, not in a new one: spans' protocol is the context
  (`trace.SpanFromContext`), logs' hook is `log.FieldsFromContext`. Metric labels stay closed;
  business labels get their own `otel.Meter(...)` instrument. (2026-09-17)
- **Every observability point must be both uniform and leave a customization slot**, promised by a
  written convention and checked mechanically (`scripts/check-observability.sh`, driven by the
  family rules). The checker's criterion is the set of common content the family rules require;
  anything outside that list is not policed.
- **Acceptance criteria V1–V5**: V1 vocabulary compliance / V2 same-kind joinable /
  **V3 every instrumentation point has a convention-defined extension entry and is checked** /
  V4 only existing log+otel APIs / V5 drift is caught by CI. V3 is the problem's landing point;
  V1/V2 are prerequisites; V5 prevents recurrence.
- **Process-level dimensions enter through environment variables, not a new go-spring API**:
  `starter-otel`'s `NewResource` merges `resource.Default()`, so `OTEL_RESOURCE_ATTRIBUTES`
  declares process dimensions and `OTEL_SERVICE_NAME` overrides the service name. **Trap (do not
  revert):** `resource.Default()` must come **first**, then go-spring's own attributes - OTel's
  default resource unconditionally carries `service.name` (`unknown_service:<binary>`), and the
  reversed order clobbers the configured name; the `service.name` from `resource.Environment()` must
  also be read explicitly for `OTEL_SERVICE_NAME` to win. (2026-09-17)
- **A user-defined log field name is used verbatim** as the log key and the span attribute key -
  never rewritten.
- **`attribute.Value.AsString()` returns "" for non-STRING values** (a bool renders as
  `load_test=` and makes an assertion silently pass); assert with `Value.Emit()`.

## What it does not cover

| Not covered | Where it belongs instead |
|---|---|
| **Context-carrier attributes on metrics** — the metric SDK has no per-record hook, so built-in metric labels stay closed by design | pass attributes where you record your own instrument |
| **Process-level dimensions** (env, cluster, version) — the same for every span in the process | the OTel resource: `spring.observability.service-name`, `OTEL_RESOURCE_ATTRIBUTES` |
| **Log fields** | `log.WithFields` / `log.Collector` in the log module |
