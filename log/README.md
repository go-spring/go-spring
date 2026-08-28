# Go-Spring :: Log

<div>
   <img src="https://img.shields.io/github/license/go-spring/log" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/log" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/log?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/log" > 
      <img src="https://codecov.io/gh/go-spring/log/graph/badge.svg?token=QBCHVEK97Q" alt="test-coverage"/> 
   </a>
   <a href="https://deepwiki.com/go-spring/log"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

**Go-Spring :: Log** is a high-performance and extensible structured logging library designed specifically for
the Go programming language. It provides a flexible tag-based classification system, contextual tracing-field
extraction, multi-level logging configuration, and multiple output options, making it ideal for a wide range of
server-side applications.

## Features

* **Multi-Level Logging**: Supports standard log levels such as `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Panic`,
  and `Fatal`, suitable for debugging and monitoring in various scenarios.
* **Structured Logging**: Records logs in a structured key-value format, natively carrying tracing information
  such as `trace_id` and `user_id`, making them easy for log aggregation systems to parse and analyze.
* **Context Integration**: Extracts contextual tracing information (e.g., request ID, user ID) from
  `context.Context` and automatically attaches it to log entries.
* **Tag-Based Logging**: An innovative tag system distinguishes logs across different modules or business lines,
  with hierarchical suffix-wildcard matching, so a unified API is available without explicitly creating logger
  instances.
* **Plugin Architecture**:
    * **Appender**: Supports multiple output targets including console, plain file, and time-rolling file.
    * **Layout**: Provides both plain text and JSON formatting for log output.
    * **Logger**: Offers both synchronous and asynchronous loggers; asynchronous mode does not block the
      business thread.
* **Flexible Rolling Logs**: Rotates files automatically at fixed time intervals, cleans up expired logs, and
  can separate WARN-and-above logs into a dedicated file.
* **Performance Optimizations**: Utilizes buffer pools and log event object pools to minimize memory allocation
  overhead, with excellent benchmark results.
* **Dynamic Configuration Reload**: The logging configuration can be reloaded at runtime via `RefreshConfig`
  without restarting the application; reading configuration files is the caller's job.
* **Well-Tested**: All core modules are covered with unit tests to ensure stability and reliability.

## Core Concepts

### Tag

Tag is the core concept in this logging library, used to categorize logs. After a tag is registered via the
`RegisterTag` function, the configuration can group-match tags with a suffix wildcard (e.g. `_app_request_*`
matches all tags starting with `_app_request_`).

This design enables a unified logging API without explicitly creating logger instances; even third-party
libraries can emit logs in a standardized way. The framework automatically matches each tag to the most
specific logger.

Three prefixes are officially recommended, covering most backend scenarios:

| Prefix | Applicable scenarios | Example |
|--------|----------------------|---------|
| `_app_` | Application lifecycle and infrastructure (startup, configuration, connection pools, circuit breakers) | `_app_startup` |
| `_biz_` | Business processes and domain events | `_biz_order_create` |
| `_rpc_` | External dependency calls (databases, caches, MQ, downstream services) | `_rpc_redis_get` |

```go
// Register tags by category
var (
	TagAppStartup     = log.RegisterAppTag("startup", "init") // application startup phase
	TagBizOrderCreate = log.RegisterBizTag("order", "create") // order creation business
	TagRpcRedisQuery  = log.RegisterRPCTag("redis", "query")  // Redis query RPC
)
```

#### When to register a custom tag

Most users never care about log tags, and that is by design: an unconfigured tag falls back to the
default logger, which is the baseline behavior — not a degraded one. A tag is not a module identity;
it is a tuning handle. Register a custom tag only when you can answer the question
"who needs to adjust these logs independently of the main log?" Concretely:

1. **Default to `log.TagAppDef` / `log.TagBizDef`.** One-off assembly, lifecycle, and error logs —
   the vast majority of framework and starter logs — belong here. Do not register a tag just
   because "this is a module".
2. **Register a custom tag only for log populations that need independent tuning**: access logs
   (high volume, separately sampled or sinked), retry/circuit-breaker events, background relays,
   or logs bridged in from a third-party framework (use `RegisterRPCTag` to keep them contained).
   If you cannot name the operator who would tune it, use the default tag.
3. **A registered tag is a public configuration contract.** `logger.<tag>.*` becomes user-visible
   configuration and must be documented (e.g. in the starter README); a tag that is not documented
   should not exist.

Naming: `subType` is a short technology or product name (no `starter_` or family prefixes — the
`_app_` prefix already conveys that, and the 4-segment budget is scarce); the optional `action`
distinguishes multiple log populations within the same module (e.g. `gin` `access` vs. lifecycle),
and is left empty otherwise.

### Logger

A `Logger` is the component that actually processes logs. Different tags can be matched to different loggers,
and each logger can independently set its level and output. You can also retrieve a logger instance by name
via `GetLogger`, mainly for compatibility with legacy projects that record pre-formatted messages via `Write`.

### Log Levels

Seven levels are supported: `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Panic`, and `Fatal`. Note that
`Panic` and `Fatal` are level semantics only (logs accompanying a panic / fatal error) - the library
does **not** panic or exit the process after recording; whether to terminate is the caller's decision.

A logger's `level` accepts two forms:

- Single level: `DEBUG`, meaning that level and above (e.g. `INFO` is equivalent to `INFO~FATAL`)
- Level range: `DEBUG~INFO`, emitting only logs inside the range - useful for filters like
  "everything below WARN"

When no logger is configured, the default level is `INFO` (overridable via the `GS_LOGGER_DEFAULT_LEVEL`
environment variable).

### Caller Information

Every log entry automatically carries the caller's file name and line number. The default `fast` mode
(caches per call-site PC; identical results to `default` but faster) can be adjusted via the
`GS_LOGGER_CALLER_TYPE` environment variable:

| Value | Description |
|-------|-------------|
| `fast` | Default. Caches lookups by call-site PC; output is identical to `default`, just faster |
| `default` | Resolves through `runtime.Caller` on every call, no caching |
| `none` | No caller information collected, best performance |

### Contextual Field Extraction

You can configure hook functions to extract contextual data from `context.Context` and include it in log entries:

* `StringFromContext`: extracts a string value from the context (e.g., a request ID), shown as a
  standalone segment
* `FieldsFromContext`: returns a list of structured fields from the context, such as trace ID or user ID

Both hooks are package-level function variables set once at process startup (`starter-otel` sets them
automatically to emit tracing fields). Extracted fields are placed before business fields. Both are
invoked for every log entry - keep them lightweight and prefer cached values.

## Logging API

Every level comes in two styles: structured and formatted. Note that the field parameter takes
two different forms:

- `Trace` / `Debug` take a **lazy builder** `func() []Field` - the closure is not invoked (and
  nothing is allocated) when the level is disabled; ideal for high-frequency debug logs
- `Info` through `Fatal` take **ready-made fields** `...Field` - fields are constructed before
  the call

```go
log.Debug(ctx, tag, func() []log.Field {
	return []log.Field{log.String("user_id", "10001")} // skipped when level disabled
})
log.Info(ctx, tag, log.String("user_id", "10001"), log.Msg("login succeeded"))
log.Infof(ctx, tag, "user %s logged in", userID)
```

Common field constructors:

| Category | Functions |
|----------|-----------|
| Message | `Msg` / `Msgf` |
| Scalars | `Nil` / `Bool` / `Int` / `Uint` / `Float` / `String` (each with a nullable `XxxPtr` variant) |
| Slices | `Bools` / `Ints` / `Uints` / `Floats` / `Strings` |
| Composite | `Reflect` (encode any value via reflection) / `Any` / `Array` / `Object` (nested fields) / `FieldsFromMap` |

## Output Formats

**TextLayout** (default, plain text with `||` separators):

```
[INFO][2026-08-19T10:23:45.123][main.go:42] _app_def||event=user_login||user_id=10001||msg=user logged in successfully
```

**JSONLayout** (friendly to log aggregation systems):

```json
{"level":"info","time":"2026-08-19T10:23:45.123","fileLine":"main.go:42","tag":"_app_def","event":"user_login","user_id":10001,"msg":"user logged in successfully"}
```

Contextual fields (produced by `StringFromContext` / `FieldsFromContext`) precede business fields in
both formats.

## Installation

```bash
go get go-spring.org/log
```

## Quick Start

```go
package main

import (
	"context"
	"encoding/json"
	"os"

	"go-spring.org/log"
	"go-spring.org/stdlib/flatten"
)

func main() {
	// Configure context field extraction.
	// You may also use StringFromContext if only strings are needed.
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		return []log.Field{
			log.String("trace_id", "0a882193682db71edd48044db54cae88"),
			log.String("span_id", "50ef0724418c0a66"),
		}
	}

	// Load the logging configuration from a file. Parsing the file is the
	// caller's job: decode it into a map, flatten it, then refresh.
	b, err := os.ReadFile("log.json")
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		panic(err)
	}
	if err := log.RefreshConfig(flatten.Flatten(m)); err != nil {
		panic(err)
	}

	ctx := context.Background()

	// Simple formatted logging
	log.Infof(ctx, log.TagAppDef, "app started, version: %s", "v1.0.0")
	log.Errorf(ctx, log.TagBizDef, "failed to handle order request: %v", err)

	// Structured logging
	log.Info(ctx, log.TagAppDef,
		log.String("event", "user_login"),
		log.Int("user_id", 10001),
		log.Msg("user logged in successfully"),
	)
}
```

## Configuration

Go-Spring :: Log supports property files, JSON, YAML, and other configuration formats — the caller parses the
file into a flat property map (e.g., via `stdlib/flatten`) and hands it to `RefreshConfig`. For example:

```properties
# Async logger buffer size
bufferSize=1000

# File appender
appender.file.type=FileAppender
appender.file.dir=./logs
appender.file.file=app.log
appender.file.layout.type=JSONLayout

# Console appender
appender.console.type=ConsoleAppender
appender.console.layout.type=TextLayout

# Root logger
logger.root.type=Logger
logger.root.level=INFO
logger.root.appenderRef.ref=console

# Dedicated async logger for matching tags
logger.request.type=AsyncLogger
logger.request.level=DEBUG
logger.request.tag=_app_request_*,_rpc_*
logger.request.bufferSize=${bufferSize}
logger.request.onBufferFull=block
logger.request.appenderRef[0].ref=file
```

**Configuration notes**:
- `appender.xxx.type` - appender type
- `logger.yyy.type` - logger type
- `logger.yyy.level` - log level range, supports forms like `DEBUG` and `DEBUG~INFO`
- `logger.yyy.tag` - list of tags to match, supports the suffix wildcard
- `logger.yyy.appenderRef[n].ref` - name of a linked appender; multiple refs allowed
- `logger.yyy.appenderRef[n].level` - optional per-appender level filter
- `${property}` variable references are supported

### Common Settings

Frequently used attributes of each plugin beyond `type` (unlisted ones use defaults):

**FileAppender / RollingFileAppender**

| Attribute | Default | Description |
|-----------|---------|-------------|
| `dir` | `./logs` | Log directory |
| `file` | - | Log file name (required) |
| `layout` | `TextLayout` | Layout plugin |
| `interval` | `1h` | Rotation interval (Rolling only, e.g. `24h`) |
| `maxAge` | `168h` | Max retention of old logs; expired files are cleaned automatically (Rolling only) |
| `syncLock` | `false` | Recommended when multiple goroutines write the same file (Rolling only) |

**Logger (SyncLogger / AsyncLogger)**

| Attribute | Default | Description |
|-----------|---------|-------------|
| `level` | all | Log level range |
| `tag` | `*` | List of tags to match |
| `bufferSize` | `10000` | Buffered event count for async mode, minimum 100 (Async only) |
| `onBufferFull` | `discard` | Buffer-full strategy: `block` / `discard` / `drop-oldest` (Async only) |

**TextLayout / JSONLayout**

| Attribute | Default | Description |
|-----------|---------|-------------|
| `fileLineMaxLength` | `48` | Max display width of the caller's "file:line" field; longer values are truncated |

Convenience loggers (`ConsoleLogger`, `FileLogger`, `RollingFileLogger`) build their appenders in-tree -
configure `dir` / `file` / `layout` and other same-named fields directly on the logger. `RollingFileLogger`
additionally supports `separate` (WARN-and-above go to a `.wf` file) and `async` + `bufferSize` +
`onBufferFull` (built-in async output).

## Built-in Plugins

### Appender

| Plugin | Description |
|--------|-------------|
| `ConsoleAppender` | Writes to standard output |
| `FileAppender` | Writes to a single file |
| `RollingFileAppender` | Rotates files at fixed time intervals, automatically cleans up expired logs |
| `DiscardAppender` | Discards all logs |

### Layout

| Plugin | Description |
|--------|-------------|
| `TextLayout` | Human-readable plain text format |
| `JSONLayout` | Structured JSON format |

### Logger

| Plugin | Description |
|--------|-------------|
| `Logger` / `SyncLogger` | Synchronous logger that writes on the calling thread |
| `AsyncLogger` | Asynchronous logger processed by a background thread without blocking the business logic. Supports three buffer-full strategies: `block` (wait for space), `discard` (drop the new event), `drop-oldest` (drop the oldest event) |
| `ConsoleLogger` | Convenience logger that writes directly to the console |
| `FileLogger` | Convenience logger that writes directly to a file |
| `RollingFileLogger` | Convenience logger for time-rolling files, supports error-log separation |
| `DiscardLogger` | Discards all logs |

**RollingFileLogger features**:
- Rotates log files automatically at the configured time interval
- Automatically removes old logs beyond the maximum retention period
- Supports `separate=true` to split WARN-and-above logs into a dedicated `.wf` file, making troubleshooting easier

## Benchmarks

The project ships benchmarks against mainstream logging libraries (zap, logrus, zerolog, slog, etc.); this
library delivers excellent performance while keeping a concise and extensible API. Run the following command
to see the comparison:

```bash
go test -bench=. ./benchmarks/logs
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GS_LOGGER_DEFAULT_LEVEL` | `INFO` | Default level when no logger is configured; accepts level-range syntax |
| `GS_LOGGER_BUFFER_CAP` | `10KB` | Pool-return cap for encoding buffers; larger buffers are discarded |
| `GS_LOGGER_CALLER_TYPE` | `fast` | Caller-info mode: `fast` / `default` / `none` |

Environment variables are read at process startup and do not take effect if changed at runtime;
use `RefreshConfig` for runtime adjustments to the logging topology.

## Custom Extensions

You can customize output targets and formats by implementing the `Appender` / `Layout` interfaces:

```go
type KafkaAppender struct {
	log.AppenderBase // built-in Name / Layout common fields

	Topic string `PluginAttribute:"topic"`
	// ... your Kafka producer
}

func (c *KafkaAppender) Start() error         { return producer.Connect() }
func (c *KafkaAppender) Stop()                { producer.Close() }
func (c *KafkaAppender) ConcurrentSafe() bool { return true }
func (c *KafkaAppender) Append(e *log.Event) {
	// encode the event with c.Layout and send it; do not hold the Event past Append
}

// Register during package init; then use appender.kafka.type=KafkaAppender in config
func init() {
	log.RegisterPlugin[KafkaAppender]("KafkaAppender")
}
```

Two things to note:

- An appender whose `ConcurrentSafe` returns `false` can only be attached to an `AsyncLogger`
  (written serially by the background thread); concurrency-safe appenders also work with
  synchronous loggers
- `Append` must not modify or retain the `Event` - it is a pooled object that may be reused at
  any time

## Architecture & Design

This library is Go-Spring's foundation logging library with zero business dependencies (only
`stdlib` and the Go standard library); it is consumed by `spring/` and every `starter-*`.
Design goals: **pluggable, config-driven, zero allocation per event on the hot path**.

### Layered Model

A log entry flows through three layers from call site to disk:

```
caller                    config (RefreshConfig)               output
log.Info(ctx, tag, ...) -> Tag -> Logger -> Layout -> Appender
```

- **Tag**: the sole caller-side entry point. Business code holds a `*Tag` and never a `Logger`
  value, so a config refresh can atomically rebind loggers without callers noticing.
- **Logger**: matched by tag, each with its own level and output.
- **Layout / Appender**: formatting and output targets, registered via `RegisterPlugin`;
  config values like `type=JSONLayout` are resolved through this registry.

### Hot Reload

`RefreshConfig` takes a flat property map, then atomically swaps the global logger/appender
set behind atomic pointers: readers take no locks and no logs are lost during the swap.
`Refresh` also starts new plugins and stops the replaced ones (implement `Lifecycle` to
participate). Reading and parsing the configuration file is the caller's job - the library
does not bind itself to any config format.

### Performance Design

- **Zero-allocation field construction**: the primary API takes a `func() []Field` builder, so
  level-disabled call sites allocate nothing for fields; `Infof`-style formatted variants are
  more ergonomic but take the slower path.
- **Object pooling**: log events and encoding buffers come from `sync.Pool`; buffers larger
  than 10 KB are discarded instead of returned to the pool (tunable via the
  `GS_LOGGER_BUFFER_CAP` environment variable).
- **Value-type fields**: `Field` is a value type; primitive helpers like `Bool`/`Int`/
  `String`/`Msg` build fields without allocating a slice.

### Usage Constraints

- Tags can only be declared during package init: calling `RegisterTag` after the first
  `RefreshConfig` panics - this is part of the atomic-swap contract.
- Tag naming rules: 3–36 characters, lowercase letters / digits / underscores only, 1–4
  segments; `RegisterAppTag` / `RegisterBizTag` / `RegisterRPCTag` generate well-formed names
  automatically.
- In a custom `Appender`, do not retain a `Field.Any` slice or a `*bytes.Buffer` past
  `EncodeTo` - they are pooled objects that may be reused at any time.
- Only console / file / rolling-file sinks ship in-tree. Remote outputs such as Kafka or Loki
  belong in their own starter implementing and registering an `Appender`, keeping this library
  dependency-free.

## License

Go-Spring :: Log is licensed under the [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).
