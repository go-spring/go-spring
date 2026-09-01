# observe Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`observe` gives every client starter one shared instrumentation kit: an
`Observer` emitting trace span + duration/in-flight metric + access log per
operation, one `ObserveConfig` bound under each starter's
`observability.*` block, and shared bridge adapters for the cross-cutting
client interfaces (lock, resilience executor, transaction observer).

## 1. Responsibilities & Boundaries

- **Does:** define `Observer` / `Span` / `SemConv` / `ObserveConfig`, ship
  the DB / messaging / resilience conventions. Instrumentation for the domain
  packages lives in the domain packages themselves
  (`experimental/lock.WrapLocker`, `experimental/transaction.SagaObserver`,
  `governance/resilience.WrapExecutor` et al.).
- **Refuses:**
  - No OTel bootstrap. The kit reads the OTel globals; installing providers
    is starter-otel's job.
  - No spring-core code. The otel-free core defines `lock.Locker`,
    `resilience.Executor`, `transaction.Observer`; the instrumentation
    belongs beside the adapters, not in core — this boundary is what makes
    the bridges live here.
  - No per-instance trace/metric config. Trace and metric ride one global
    pipeline; only the access log is meaningfully per-instance.

## 2. Key Abstractions / Seams

- **`SemConv` travels as a bundle.** A metric prefix is meaningless without
  its matching attribute keys, so `New` takes the whole bundle. The named
  constructors (`NewDB`, `NewProducer`, `NewConsumer`) guarantee a coherent
  SemConv/span-kind pairing; `New` is the escape hatch for custom client
  types.
- **`Start`/`End` is the only call shape.** One span per operation with
  `defer`-friendly semantics; skipped ops return a no-op Span so the seam
  code needs no branching. `Span.SetArg` covers the gorm-style case where
  the argument arrives after timing started.
- **OTel globals as the dependency seam.** Without starter-otel the
  tracer/meter are no-ops and the SpanContext stays invalid — an
  unconfigured app pays almost nothing, which is what lets every starter
  wire the kit unconditionally.
- **`statusKey` is the kit's own attribute**, deliberately not OTel
  semconv: the coarse ok/error code is the metric dimension; error detail
  stays on the span/log.

## 3. The resilience bridge (folded into the main Observer, 2026-08)

`WrapExecutor` was initially a standalone trace-only wrapper; it now builds
on `observe.New` with `ResilienceSemConv`, so a protected call emits the
same three signals as every other client op (span kind internal, the
"operation" is the protected resource name). On top of that it adds:

- a `resilience.calls` counter with a **six-value outcome** dimension:
  `success`, `rate_limited`, `circuit_open`, `bulkhead_full`, `timeout`,
  `error` — distinguishing "rejected by protection" from "downstream
  failed" on a dashboard, plus the `resilience.outcome` attribute on the
  span;
- a `resilience.breaker.state_change` counter + log (trip at Warn,
  recovery/half-open at Info), subscribed via
  `BreakerEventListenerSetter` when the inner driver supports it —
  drivers without the capability are silently skipped, call-level signals
  still emit.

The motivation: the core resilience package deliberately does no
metric/trace/log, so circuit trips were a production black box. A nil inner
executor returns nil — an unarmed client stays untouched.

## 4. The lock bridge: WrapLocker(system, cfg, inner)

Wraps any `lock.Locker` so `Acquire`/`TryAcquire` emit the full three
signals under the `lock` convention (`lock.system` / `lock.operation` /
`lock.key`). Two shape decisions:

- it goes through `observe.New` (not a bespoke trace wrapper, its earlier
  form), so the four lock starters share one ~70-line implementation
  differing only in the system label;
- a missed `TryAcquire` is not an error — `err` stays nil and the miss is
  recorded as `lock.acquired=false` so the metric dimension separates a
  loss from a win.

## 5. Trade-offs / Alternatives Rejected

- **otel-only, rather than a provider-neutral signal API.** Supporting
  non-OTel backends would fork every emitter for a speculative need; OTel
  is the lingua franca and its globals give a zero-config no-op path. The
  cost — the spring core cannot host this kit — is accepted and resolved
  by layering (abstraction in core, bridges here).
- **Dedicated observe-lock / observe-resilience / observe-transaction
  modules were collapsed into subpackages of one
  `go-spring.org/cloud/observe`** once the module boundary proved to be
  overhead without a consumer difference; the lock/transaction bridges
  target the cloud domain packages (`cloud/lock`, `cloud/transaction`,
  `cloud/governance/resilience`) without dragging starters into
  the dependency graph. The gorm bridge is the deliberate exception: it
  lives in `go-spring.org/starter-gorm`'s `observe` package, because this
  module must not depend on gorm.
- **Binary status, no error taxonomy on the metric.** Classifying error
  kinds per client would drift per backend; the resilience bridge is the
  one place an outcome taxonomy pays for itself, so it lives there (six
  values) rather than in the generic Observer.
- **Access log always emits (unless `off`).** The log signal does not
  depend on starter-otel being present, so an uninstrumented app still
  gets per-op visibility; `SkipOps` is the escape valve for volume.
