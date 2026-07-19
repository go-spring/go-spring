# log Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`go-spring.org/log` is Go-Spring's structured logging library. It sits in
the `stdlib`-equivalent layer (zero business dependencies — only
`go-spring.org/stdlib` and the Go standard library) and is consumed by
`spring/` and every `starter-*`. Its goal is a pluggable, config-driven
logger with per-event zero-allocation goals in the hot path.

## 1. Responsibilities & Boundaries

- Emit structured events with levels (`Trace` … `Fatal`), tags, contextual
  fields, and pluggable output formatting.
- Load configuration from a flat property map (`RefreshConfig`) or a file
  (`RefreshFile`) so applications can hot-reload logger topology without
  restarting.
- Do **not** own log transport (Kafka, ES, Loki). Sinks are limited to
  console / file / rolling-file appenders shipped in-tree; anything else
  registers its own `Appender` plugin.

## 2. Key Abstractions & Seams

- **Plugin registry.** `RegisterPlugin[T](name)` (see `log/plugin.go`)
  keeps a `name → reflect.Type` map. Config values like `type=JSONLayout`
  are resolved through this registry; `PluginAttribute` / `PluginElement`
  struct tags declare how to inject scalar attributes and child plugins
  from the flattened storage. The library ships three plugin families:
  - **Appenders** (`plugin_appender.go`): `DiscardAppender`,
    `ConsoleAppender`, `FileAppender`, `RollingFileAppender`.
  - **Layouts** (`plugin_layout.go`): `TextLayout`, `JSONLayout`, both
    embedding `BaseLayout` with `fileLineMaxLength`.
  - **Loggers** (`plugin_logger.go`): `SyncLogger` (`"Logger"` alias),
    `AsyncLogger`, `DiscardLogger`, `ConsoleLogger`, `FileLogger`,
    `RollingFileLogger`. `AppenderRef` links a logger to a named appender.
- **Tag system.** `RegisterTag(name)` (`log/log_tag.go`) returns a
  `*Tag` whose `Logger` is swapped atomically at refresh time. Tags are
  the caller-side API — code writes `log.Infof(ctx, TagRequestIn, ...)`
  without ever holding a `Logger` value. Refresh maps tag names to
  configured loggers by regex.
- **Refresh pipeline.** `RefreshConfig(map)` → `parseExpr` (expands `!`
  inline map expressions) → `flatten.NewProperties` → `Refresh(storage)`.
  Refresh atomically replaces the global logger/appender set behind
  `sync/atomic` pointers, so readers never lock. `global.refreshed` is a
  one-way latch: `RegisterTag` panics after refresh so tags can only be
  declared during package init.
- **Context field extraction.** `StringFromContext` and
  `FieldsFromContext` are package-level function variables set once at
  boot (typically by `starter-otel` for `trace_id`/`span_id`). They are
  the sanctioned integration point for cross-cutting context data.
- **Field encoding.** `Field` (`log/field.go`) is a value type carrying
  `Key`, `Type` (`ValueType`), `Num` (numeric payload), `Any` (pointer /
  slice payload). Primitive helpers (`Bool`, `Int64`, `String`, `Msg`,
  `Msgf`, `Reflect`, `Array`, `Object`, `FieldsFromMap`) build fields
  without allocating a slice per call. `Event` and encoder buffers come
  from `sync.Pool` (`plugin_appender.go` `bufferPool`; buffers larger
  than `bufferCap`, default 10 KB / env `GS_LOGGER_BUFFER_CAP`, are
  discarded instead of reused).
- **Lifecycle.** Loggers and appenders may implement `Lifecycle`
  (`Start`/`Stop`); `Refresh` starts new plugins and stops replaced ones.

## 3. Constraints

- Zero dependency on `spring/` (this is a foundation library). Coupling
  goes one way: `spring/` and starters depend on `log`, never the reverse.
- All tag names and converters must be registered before the first
  `Refresh`. Registering after that panics — this preserves the
  atomic-swap contract.
- Tag strings must satisfy `isValidTag` (3–36 chars, lowercase / digit /
  underscore, 1–4 segments, optional single leading underscore); helpers
  `RegisterAppTag` / `RegisterBizTag` / `RegisterRPCTag` enforce the
  `_<main>_<sub>[_<action>]` shape.
- Appender writes take pooled buffers; do not retain a `Field.Any` slice
  or a `*bytes.Buffer` past `EncodeTo`.

## 4. Trade-offs & Alternatives Rejected

- **XML-style Log4j2 config not adopted.** Go-Spring uses a flat property
  map + inline `!` expressions (`db!: "{host: localhost, port: 5432}"`)
  because `flatten.Storage` is the shared config primitive. Layout and
  logger plugins get injected the same way as any framework bean.
- **Global logger singleton not exposed.** `GetLogger(name)` exists only
  for compatibility with legacy call sites that want `Write(level, []byte)`;
  new code writes through tags so refresh can rebind them.
- **No `zap.Sugar`-style formatted wrappers as the default path.** The
  primary API takes a `func() []Field` builder so level-disabled call
  sites pay zero allocation for the field slice. `*f` helpers exist for
  ergonomic use but are the slower path.
- **In-tree sinks only.** Kafka/OTel/HTTP sinks are not shipped here to
  keep `log` dependency-free; they belong in a starter that calls
  `RegisterPlugin` for its own appender type.
