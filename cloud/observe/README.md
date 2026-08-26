# observe
[English](README.md) | [中文](README_CN.md)

`observe` is a uniform trace + metric + log observer for client operations
(cache, database, messaging, resilience-protected calls). Each client starter
attaches its instrumentation seam — a driver hook, a connection wrapper, a
gorm callback — to one `Observer`, so every client emits the same three
signals with the same vocabulary, behind one verbosity switch.

## Wiring a new starter in three steps

1. **Embed the config** in the starter's per-instance Config:

   ```go
   type Config struct {
       ...
       Observability observe.ObserveConfig `value:"${observability:=}"`
   }
   ```

2. **Build an Observer** at client construction with the client's system
   label:

   ```go
   obs := observe.NewDB("redis", c.Observability)      // database / cache
   obs := observe.NewProducer("kafka", c.Observability) // messaging publisher
   obs := observe.NewConsumer("kafka", c.Observability) // messaging consumer
   obs := observe.New(system, mySemConv, kind, cfg)     // custom convention
   ```

   If the client library already emits trace+metric (go-redis via redisotel,
   elasticsearch's transport instrumentation), pass
   `observe.WithoutTraceAndMetric()` (or `WithoutTrace()`) so the kit fills
   only the access-log gap without duplicate spans.

3. **Start/End at the seam.** Open a span where the operation begins and end
   it exactly once with the operation's error:

   ```go
   func (c *Client) Get(ctx context.Context, key string) (string, error) {
       ctx, sp := c.obs.Start(ctx, "GET", key)
       v, err := c.inner.Get(ctx, key)
       sp.End(err)
       return v, err
   }
   ```

   If the argument is only known later (a gorm callback sees the SQL after
   the span opened), call `sp.SetArg(sql)` before `End`.

## SemConv vocabulary

A `SemConv` bundles the metric-name prefix and the attribute keys so they
always travel together:

| SemConv | Metrics | Attributes | Span kind |
|---|---|---|---|
| `DBSemConv` | `db.client.operation.duration`, `db.client.active_requests` | `db.system`, `db.operation`, `db.statement` | client |
| `MessagingSemConv` | `messaging.client.operation.duration`, `messaging.client.active_requests` | `messaging.system`, `messaging.operation`, `messaging.destination.name` | producer / consumer |
| `ResilienceSemConv` | `resilience.operation.duration`, `resilience.active_requests` | `resilience.system`, `resilience.resource`, `resilience.arg` | internal |

Every metric additionally carries a `status` dimension (`ok`/`error`); the
error detail lives on the span and the access log. Start from `DBSemConv` or
`MessagingSemConv` when defining a custom one.

## Access-log level

`observability.level` controls only the access log (trace and metric ride
the global OTel pipeline and are not per-instance configurable):

- `off` — no access log; trace and metric still emit.
- `brief` — one record per operation: operation, status, duration, error.
  This is the default; an unset (`""`) level also behaves as `brief`.
- `detailed` — brief plus the operation argument (command key, SQL
  statement, topic, ...), bounded by `maxArgBytes` (default 512).

## SkipOps

`observability.skipOps` lists operation names (as passed to
`Observer.Start`, e.g. `PING`) for which all three signals — span, metric,
and access log — are suppressed, so a chatty health probe cannot flood the
backends. Matching is exact string equality.

## Relationship with starter-otel

The kit rides the OTel globals: when starter-otel is imported it installs
the real TracerProvider / MeterProvider; when it is absent the globals are
no-ops, so trace and metric add negligible overhead and change no behavior.
The access log always emits through the project `log` package, regardless
of starter-otel.

## Shared bridges

- `observe/lock` — `WrapLocker(system, cfg, inner)` wraps any
  `lock.Locker` with the three signals (`lock.*` metrics,
  `lock.acquired=false` on a missed TryAcquire).
- `observe/resilience` — `WrapExecutor(inner, system, cfg)` wraps any
  `resilience.Executor`: three signals plus a `resilience.calls` counter
  classified by outcome and `resilience.breaker.state_change` events.
- `observe/transaction` — `SagaObserver` / `TccObserver` / `AtObserver`
  open one child span per transaction phase.

## Installation

```
go get go-spring.org/cloud
```
