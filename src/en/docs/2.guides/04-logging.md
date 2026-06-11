# Logging

Go-Spring provides a high-performance, extensible, tag-routed structured logging system.
It draws on Log4j2's layered plugin architecture and splits log classification, routing, output, formatting, and context extraction into independent pluggable components,
while also embracing the Go ecosystem's preference for simplicity and performance, avoiding the bulky and complicated configuration style often seen in Log4j2.

From now on, business code only needs to state "what kind of log this is" and "which fields to record", without worrying about where it is eventually written, which format it uses, or whether it is written asynchronously.
Whether the output is text or JSON, synchronous or asynchronous, a single output or fan-out to multiple targets, it can all be expressed flexibly through the same configuration language.

---

## Quick Start

The following example shows the complete usage flow: register tags first, then load the configuration, and finally output formatted logs and structured logs respectively.

```go
package main

import (
	"context"
	"os"

	"github.com/go-spring/log"
)

// Register tags. Tags are used for log routing, specify output targets in configuration files, and also carry business semantics.
var (
	TagAppStartup  = log.RegisterAppTag("startup", "")     // Application startup
	TagAppShutdown = log.RegisterAppTag("shutdown", "")    // Application shutdown
	TagBizOrder    = log.RegisterBizTag("order", "create") // Create order
	TagBizUser     = log.RegisterBizTag("user", "login")   // User login
)

func main() {
	// Configure logs at INFO and above to be output to the console.
	config := map[string]string{
		"logger.root.type":  "ConsoleLogger",
		"logger.root.level": "INFO",
	}

	// Use KV configuration, supporting configuration loaded from multiple sources.
	if err := log.RefreshConfig(config); err != nil {
		panic("log configuration failed: " + err.Error())
	}

	ctx := context.Background()

	// Formatted log, format mode, consistent with fmt.Sprintf in the standard library.
	log.Infof(ctx, TagAppStartup, "application started successfully, version: %s, PID: %d", "v1.0.0", os.Getpid())

	// Structured log, field mode, consistent with logging libraries such as zerolog and logrus.
	log.Info(ctx, TagBizOrder,
		log.Int("order_id", 10001),
		log.String("user", "alice"),
		log.Float("amount", 99.99),
		log.Bool("success", true),
		log.Strings("tags", []string{"vip", "new_user"}),
		log.Msg("order created successfully"),
	)

	// Supports lazy evaluation to avoid computing expensive fields that may ultimately be unnecessary.
	log.Debug(ctx, TagBizUser, func() []log.Field {
		return []log.Field{
			log.String("trace", "user_login_flow"),
			log.Msg("user login flow started"),
		}
	})

	// Supports log APIs at different levels. Log levels follow common conventions.
	log.Warnf(ctx, TagBizUser, "user %s attempted wrong password %d times", "bob", 3)
	log.Errorf(ctx, TagBizOrder, "failed to create order %d: %s", 10002, "insufficient stock")
}
```

Run the example above and you will see console output similar to the following.
Because the root logger level is `INFO`, the `Debug` call above does not actually construct or output a log.

```text
[INFO][2026-05-02T09:36:40.801][/Users/didi/ccc/myapp/main.go:31] _app_startup||msg=application started successfully, version: v1.0.0, PID: 87684
[INFO][2026-05-02T09:36:40.802][/Users/didi/ccc/myapp/main.go:34] _biz_order_create||order_id=10001||user=alice||amount=99.99||success=true||tags=["vip","new_user"]||msg=order created successfully
[WARN][2026-05-02T09:36:40.802][/Users/didi/ccc/myapp/main.go:52] _biz_user_login||msg=user bob attempted wrong password 3 times
[ERROR][2026-05-02T09:36:40.802][/Users/didi/ccc/myapp/main.go:53] _biz_order_create||msg=failed to create order 10002: insufficient stock
```

---

## Core Components

The Go-Spring logging design references Log4j2's layered plugin architecture and splits the entire path from log creation to persistence into six independent layers.
Each layer is responsible for only one category of responsibilities, and the layers collaborate through interfaces, so modifying any layer does not affect the others.

```
┌───────────────────────────────────────────────────────────┐
│                    Application API Layer                  │
│  Trace/Debug/Info/Warn/Error/Panic/Fatal · Record · GetLogger
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                         Tag Layer                         │
│  Tag registration → Tag lookup → Logger binding → Routing │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                        Logger Layer                       │
│  SyncLogger  │  AsyncLogger  │  Console/File/RollingFile  │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                       Appender Layer                      │
│  ConsoleAppender  │  FileAppender  │  RollingFileAppender │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                        Layout Layer                       │
│  TextLayout (human-readable) │ JSONLayout (structured)    │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                        Encoder Layer                      │
│  Field encoding → Type encoding → Array/Object encoding   │
└───────────────────────────────────────────────────────────┘
```

- **Application API Layer**: This is the unified entry point for business code and the external facade of the entire logging system.
The application layer provides methods for all levels from `Trace` to `Fatal`, and supports three calling styles: formatted output, structured fields, and lazy evaluation.
Its core value is to completely decouple business code from the underlying implementation.
No matter how output targets are adjusted later, whether synchronous/asynchronous mode is switched, or the log format is changed, the business-side log calling code does not need to change.
At the same time, this layer is also responsible for automatically extracting trace information from `context.Context`, ensuring context fields do not need to be passed manually at every call site.

- **Tag Layer**: This is the most distinctive layer in the Go-Spring logging system, and it completely changes the traditional package-name inheritance routing approach.
A tag is a static semantic marker for a log. It is registered centrally during initialization and bound to a Logger after configuration refresh.
When a log is generated, the tag layer performs route lookup in the priority order of "exact match → prefix match → root Logger" and ultimately determines which output pipeline the log enters.
This design allows different semantic logs emitted from the same code file to go to completely different destinations:
startup logs can be printed to the console, business logs written to files, audit logs sent to Kafka, and error logs pushed to an alerting system.

- **Logger Layer**: This is the scheduling hub of the log stream, connecting tag routing above with actual writing below.
The Logger first performs level filtering: if the log event's level is outside the configured range, the event is discarded directly here, avoiding unnecessary subsequent encoding and IO costs.
After passing the level check, the Logger distributes the event to one or more Appenders bound to it.
The Logger layer also determines the write model:
a synchronous Logger completes all writes in the calling goroutine;
an asynchronous Logger first writes events into an in-memory queue, and a background goroutine handles subsequent processing, isolating log IO from the request path.

- **Appender Layer**: This is where logs are truly persisted. It writes encoded log events to specific target systems.
Console, local file, rolling file, Kafka, HTTP APIs, and remote logging services each correspond to an Appender implementation.
A Logger can bind multiple Appenders at the same time, implementing "one log, multiple outputs".
For example, the same error log can be written to a local file and also reported to an alerting platform.
If a new output target is needed, we only need to implement a new Appender and register it with the plugin system, then it can be enabled in configuration.

- **Layout Layer**: This is the log formatting layer, deciding what the final output looks like.
Layout does not care where fields come from or where they are ultimately written. It focuses on one thing: converting log events into byte streams in the target format.
The built-in TextLayout is human-readable and organizes fields as `key=value`;
JSONLayout is designed for machine parsing and retrieval, outputs standard single-line JSON, and can directly connect to log collection systems.
If other formats such as Protobuf or CSV are needed, we only need to add the corresponding Layout implementation and register it with the plugin system, then it can be enabled in configuration.

- **Encoder Layer**: This is the key layer for performance optimization and the layer closest to low-level encoding details.
The Encoder is designed to avoid reflection and intermediate objects as much as possible, allowing fields to be encoded directly into the target Writer.
For example, primitive fields (Int, String, Bool, and so on) all carry their own type information. During encoding, they directly use type-specific encoding paths without dynamic inference.
Arrays and nested objects use streaming encoding: iterate and write at the same time, avoiding building a complete intermediate structure first.
This design can significantly reduce memory allocations and GC pressure under high concurrency, keeping the logging system overhead as low as possible.

---

## Tag System

Tags are the core innovation of the Go-Spring logging system. They fundamentally change how logs are routed.

Traditional logging systems usually route logs according to hierarchical inheritance by package or class name:
logs from package `a.b.c` inherit the configuration of `a.b`, and `a.b` inherits the configuration of `a`.
This design feels natural in object-oriented languages such as Java, because class names themselves represent semantic boundaries in code.
But in Go, there is no class hierarchy, and package names often reflect code organization rather than business semantic boundaries.
The more fundamental problem is that package names and log semantics have never had a one-to-one relationship.
In the same `dao` package, there may be database connection pool initialization logs, business operation logs, and downstream RPC call logs at the same time.
These logs should have completely different importance, output destinations, and retention periods, but under the package-name routing model they can only share the same output strategy.

Go-Spring's tag system is the answer to this problem.
A tag is a static semantic marker for a log. It answers "what kind of log is this" rather than "which file does this log come from".
Through tags, code can explicitly declare the business semantics of logs, allowing routing logic to truly align with business intent instead of being constrained by code organization.

Tags are usually registered centrally during initialization and automatically bound to the corresponding Logger after configuration refresh.
Business code only holds `*log.Tag` and does not hold Logger directly, so later adjustments to log output strategy do not require changing business code.

### Tag Naming Conventions

Tag naming is not just a syntax rule; it is also the foundation of team collaboration. Good names make code clearer, logs easier to search, and troubleshooting more efficient.
Especially as team size grows, unified naming conventions avoid the confusion of "everyone having their own style".

#### Naming Rules

| No. | Rule | Details | Design Intent |
|-----|------|---------|---------------|
| 1 | **Length range 3-36** | At least 3 characters and no more than 36 characters | Too short lacks semantics; too long is inconvenient for configuration and retrieval |
| 2 | **Character set restriction** | Only lowercase letters `a-z`, digits `0-9`, and underscores `_` are allowed | Avoid case confusion and ensure compatibility in file names, configuration keys, and other scenarios |
| 3 | **Segment constraint** | May have one leading underscore. After removing the leading underscore, it can be split into at most 4 segments separated by underscores | Enforce a hierarchical structure and avoid arbitrary naming without rules |
| 4 | **Format constraint** | Consecutive underscores are not allowed, and names may not end with an underscore | Ensure consistent formatting and reduce unnecessary parsing ambiguity |
| 5 | **Recommended three-part style** | Recommended format: `_<category>_<subtype>_<action>`. Category indicates the log class, subtype indicates the object or domain, and action indicates the operation that occurred | Ensure naming consistency from the source and greatly improve understandability in cross-team collaboration |

#### Category Prefixes

Go-Spring officially recommends the following four prefix categories, which cover most backend application scenarios:

| Category Prefix | Applicable Scenarios | Typical Examples |
|-----------------|----------------------|------------------|
| `_app_` | Application lifecycle and infrastructure | Startup, shutdown, configuration loading, health checks, scheduled task dispatching |
| | | `_app_startup`, `_app_shutdown`, `_app_config_reload` |
| `_biz_` | Business processes and domain events | Order creation, user login, payment callback, state change notification |
| | | `_biz_order_create`, `_biz_user_login`, `_biz_pay_success` |
| `_rpc_` | External dependency calls | Database operations, cache reads/writes, message queue sends, downstream HTTP calls, gRPC service requests |
| | | `_rpc_redis_get`, `_rpc_mysql_query`, `_rpc_http_call` |
| `_infra_` | Framework and middleware internals | Connection pool exhaustion, circuit breaker opening, retry triggered, degradation logic executed |
| | | `_infra_pool_exhausted`, `_infra_circuit_open` |

> These categories are recommended conventions, not technical restrictions. You can absolutely customize other categories according to project characteristics,
> but remember: **categories should remain consistent within the same project**.

### Tag Registration

Go-Spring provides `RegisterAppTag`, `RegisterBizTag`, and `RegisterRPCTag` to generate standardized tags,
and also provides `RegisterTag` for registering custom tags. When using it, be careful to follow the tag naming conventions.

**Usage example:**

```go
// Application-layer tags
log.RegisterAppTag("startup", "")      // _app_startup
log.RegisterAppTag("shutdown", "")     // _app_shutdown
log.RegisterAppTag("config", "reload") // _app_config_reload

// Business-layer tags
log.RegisterBizTag("order", "create") // _biz_order_create
log.RegisterBizTag("order", "cancel") // _biz_order_cancel
log.RegisterBizTag("user", "login")   // _biz_user_login

// RPC-layer tags
log.RegisterRPCTag("redis", "get")   // _rpc_redis_get
log.RegisterRPCTag("http", "call")   // _rpc_http_call
log.RegisterRPCTag("grpc", "invoke") // _rpc_grpc_invoke

// Custom tags
log.RegisterTag("_cache_hit")        // _cache_hit
log.RegisterTag("_mq_kafka_produce") // _mq_kafka_produce
```

### Tag Routing

When tags are bound to Loggers, they use an **exact-first, longest-first** matching strategy. Lookup proceeds from highest to lowest priority and stops immediately after the first match.
For example, for the tag `_biz_order_create`, the system attempts matches in the following order:

| Matching Stage | Match Target | Description |
|----------------|--------------|-------------|
| 1. Exact match | `_biz_order_create` | Completely identical tag name, highest priority |
| 2. Three-segment prefix | `_biz_order_*` | Remove the last segment and keep the first three segments for prefix matching |
| 3. Two-segment prefix | `_biz_*` | Keep the first two segments for prefix matching |
| 4. Fallback match | `logger.root` | Final destination for all logs |

By using the hierarchical nature of prefix matching, we can implement flexible routing where "major categories use common configuration and subcategories use special configuration".
When new business tags are added, as long as the names comply with conventions, they can automatically enter the correct output pipeline.

---

## Log Levels

Log levels are used to decide whether a log should be output.
Go-Spring assigns a numeric value to each level, so we can insert custom levels between standard levels and also express filtering conditions with ranges.

| Level | Value | Description |
|-------|-------|-------------|
| NONE | 0 | Disable all log output, suitable for scenarios such as performance stress tests where logs need to be temporarily suppressed. |
| TRACE | 100 | The finest-grained tracing information, used to record function inputs/outputs, loop iteration state, line-by-line execution traces, and so on. Usually not enabled in production. |
| DEBUG | 200 | Debug information, used to record component initialization details, conditional branch paths, configuration loading details, intermediate state snapshots, and other context needed for development troubleshooting. |
| INFO | 300 | Regular runtime information, used to record milestone events in normal flows such as service startup completion, request processing completion, key state changes, and scheduled task results. |
| WARN | 400 | Recoverable exception warnings, used to record abnormal situations where the system can still continue running, such as retries, call timeouts, service degradation, and resources approaching thresholds. |
| ERROR | 500 | Business or system errors, used to record issues requiring attention and troubleshooting, such as request processing failures, downstream dependency exceptions, and data validation failures. The process itself can still run normally. |
| PANIC | 600 | Severe system errors, used to record fatal issues that affect service availability, such as exceptions about to trigger panic, core component initialization failures, and resource exhaustion. |
| FATAL | 700 | Fatal errors, used to record the last log before process exit, usually unrecoverable core failures. The process terminates after the record is written. |
| MAX | 999 | Upper-bound level marker, used only for level range comparisons and not directly output as a log. |

### Custom Levels

We can insert custom levels between the standard log levels defined by Go-Spring.
For example, we can define an audit level `AUDIT` between `INFO` and `WARN`:

**Example code:**

```go
var AuditLevel = log.RegisterLevel(350, "AUDIT")
var TagBizAudit = log.RegisterBizTag("audit", "record")

log.Record(ctx, AuditLevel, TagBizAudit, 2,
	log.String("user_id", "10086"),
	log.String("action", "modify_password"),
	log.String("ip", "192.168.1.1"),
)
```

---

## Log Output

Go-Spring provides two types of output APIs: formatted logs and structured logs.
Simple text information can use `*f` formatting methods; business logs should preferably use structured fields.

**API example:**

```go
// Formatted logs
log.Infof(ctx, TagAppStartup, "application started successfully, version: %s, PID: %d", "v1.0.0", os.Getpid())
log.Warnf(ctx, TagBizUser, "user %s attempted wrong password %d times", "bob", 3)
log.Errorf(ctx, TagBizOrder, "failed to create order %d: %s", 10002, "insufficient stock")

// Structured logs
log.Info(ctx, TagBizOrder,
	log.Int("order_id", orderID),
	log.String("status", "created"),
	log.Int("duration_us", duration.Microseconds()),
	log.Msg("order creation completed"),
)
```

Formatted logs are intuitive and suitable for local debugging and simple prompt messages.
The downside is that fields are concatenated into strings, so later retrieval and statistics require secondary parsing by the logging platform.

Structured logs retain field types, making it easier for logging systems to directly index and aggregate them.
On performance-sensitive paths, strongly typed field constructors should be preferred.

### Lazy Evaluation

Go-Spring **requires** `Trace` and `Debug` to use lazy evaluation.
These two levels often contain time-consuming operations such as complex serialization and aggregate computation, and online environments often do not enable them.
**Requiring** lazy evaluation at the API level ensures that no meaningless computation overhead is incurred when the level is disabled.

**Usage example:**

```go
log.Trace(ctx, TagBizUser, func() []log.Field {
	detail, _ := json.Marshal(complexObject)
	stats := calculateExpensiveStats()

	return []log.Field{
		log.String("detail", string(detail)),
		log.Any("stats", stats),
		log.Int("item_count", len(complexObject.Items)),
	}
})
```

### Adjusting Stack Depth

When users need to wrap their own logging utility functions, the file name and line number output by APIs such as `Info` and `Warn` point to the wrapper function itself,
rather than the real business call location. In this case, we can adjust the call stack depth through the `skip` parameter of the `Record` function to skip stack frames from the wrapper layer.

**Example code:**

```go
func Audit(ctx context.Context, tag *log.Tag, fields ...log.Field) {
	log.Record(ctx, AuditLevel, tag, 3, fields...)
}
```

---

## Structured Logging

Traditional logs concatenate all information into free text. Retrieval and statistics both depend on regular expression matching, which is neither precise nor type-aware.
Structured logging breaks log content into typed key-value pairs (fields), making log data directly indexable, aggregatable, and machine-understandable.

Go-Spring's Field system references mainstream logging libraries such as zerolog and zap, and uses a **strongly typed** design:
each field carries type information and directly follows a dedicated path during encoding, avoiding reflection overhead and improving encoding and decoding performance.

The first parameter of almost all Field constructors is the field name (key). If there is a second parameter, it represents the field value (value).
Only a few special cases are exceptions, such as the `Nil` field and the `Msg` field.

**Basic usage example:**

```go
log.Info(ctx, tag,
	log.Int("user_id", userID),
	log.String("ip", ip),
	log.Bool("success", success),
	log.Msg("user login completed"),
)
```

Go-Spring provides rich field types covering common scenarios such as primitive types, pointers, arrays, and nested objects:

### Primitive Types

Go-Spring provides corresponding field constructors for all primitive types. Each function directly encodes the corresponding type, avoiding type inference overhead.

**Primitive type field examples:**

```go
log.Bool("success", true)

log.Int("user_id", 10001)
log.Int("duration_us", duration.Microseconds())

log.Uint("bytes_transferred", uint64(1024*1024))

log.Float("amount", 99.99)
log.Float("latency_ms", 123.45)

log.String("order_no", "ORD202401010001")
log.String("ip", "192.168.1.1")
```

### Pointer Types

Go-Spring provides corresponding field constructors for pointer versions of all primitive types, directly handling pointer dereferencing and nil checks.
If the pointer is `nil`, the field value outputs `null`; if non-`nil`, it outputs the actual value pointed to.

**Pointer type field examples:**

```go
var enabled *bool
log.BoolPtr("enabled", enabled)

var userID *int64
log.IntPtr("user_id", userID)

var remark *string
log.StringPtr("remark", remark)

log.Nil("deleted_at")
```

### Message Fields

Go-Spring provides two special field functions, `Msg` and `Msgf`. Their key is always `msg`.
They can be used to store human-readable log summaries.
Structured information (user IDs, order numbers, status codes, and so on) should still be split into independent fields for retrieval and aggregation.

**Usage examples:**

```go
log.Msg("order created successfully")
log.Msgf("processed %d records, succeeded %d, failed %d", total, success, failed)
```

### Arrays and Nested Objects

Go-Spring also provides corresponding field constructors for arrays and complex nested objects.

**Array and object field examples:**

```go
log.Ints("item_ids", []int{1, 2, 3})
log.Strings("tags", []string{"vip", "new_user"})
log.Bools("flags", []bool{true, false})
log.Floats("prices", []float64{9.99, 19.99})

log.Object("order",
	log.String("order_no", "ORD001"),
	log.Int("user_id", int64(10001)),
	log.Float("amount", 99.99),
	log.Bool("paid", true),
	log.Object("item",
		log.String("sku", "ITEM001"),
		log.Int("quantity", 2),
	),
)
```

### Map Expansion

Go-Spring provides a field function `FieldsFromMap`, which expands a `map[string]any` into multiple fields,
instead of outputting it as a single map field.

**Usage example:**

```go
data := map[string]any{
	"order_id": "ORD001",
	"amount":   99.99,
	"user_id":  int64(10001),
	"success":  true,
}

log.Info(ctx, tag, log.FieldsFromMap(data))
```

### Automatic Type Inference

Go-Spring provides the `Any` field function, which automatically selects an appropriate field constructor according to the field value.
When the value cannot be recognized, it selects a suitable encoding path based on the dynamic type and finally falls back to reflection-based encoding.

**Usage examples:**

```go
log.Any("order_id", "ORD001")
log.Any("amount", 99.99)
log.Any("user_id", int64(10001))
log.Any("tags", []string{"a", "b"})
```

Although `Any` is convenient, strongly typed fields are more explicit and can avoid reflection paths. Therefore, strongly typed fields are recommended whenever possible.

---

## Logger

Logger is the core scheduling unit of the entire logging system, connecting tag routing above with Appender writing below.
When a log event arrives, the Logger first performs level filtering.
If the event level is outside the configured range, it is discarded directly here, avoiding unnecessary subsequent encoding and IO costs.
After the level check passes, the Logger distributes the event to one or more Appenders bound to it to complete the final write operation.

Go-Spring designs Logger into two major categories, corresponding to different usage scenarios:

- **Composed Loggers**: Include `SyncLogger` and `AsyncLogger`. They do not contain write logic themselves.
They reference one or more Appenders through `appenderRef`, supporting flexible multi-output and strategy composition.

- **Integrated Loggers**: Include `ConsoleLogger`, `FileLogger`, and `RollingFileLogger`.
They encapsulate common output targets and Layout configuration internally, satisfying most scenarios with shorter configuration.

The two categories of Logger are functionally equivalent. The core consideration when choosing is the balance between configuration simplicity and flexibility.
For simple scenarios, prefer integrated Loggers. Use composed Loggers when multiple outputs or complex strategies are needed.

### SyncLogger (Synchronous)

`SyncLogger` is the most basic Logger implementation. It directly executes the write process in the business-calling goroutine,
from level filtering and field encoding to Appender writing, all within the same call stack, returning only after writing is complete.

The biggest characteristic of synchronous writing is **strong determinism**:
the log is either written successfully or an error is reported immediately; there is no intermediate state where it is "floating in an in-memory buffer".

It is especially suitable for the following scenarios:
- Low-throughput phases such as application startup and initialization, where errors should be exposed as early as possible
- Critical paths such as audit logs and transaction records, where logs must not be lost
- Local development and debugging environments, where immediate log output is helpful for breakpoint tracing

The following configuration example shows how to use `SyncLogger` to output to both console and file, implementing "one source, two writes":

**Complete configuration example:**

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout

logger.sync.type = SyncLogger
logger.sync.tag = _app_*
logger.sync.level = INFO
logger.sync.appenderRef[0].ref = console
logger.sync.appenderRef[1].ref = file
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `tag` | Yes | Tag expression bound to the Logger | `_app_*`, `_biz_order_create` |
| `level` | Yes | Log level or level range | `INFO`, `DEBUG~WARN` |
| `appenderRef[n].ref` | Yes | Referenced Appender name | `console`, `file` |
| `appenderRef[n].level` | No | Level filtering range for this Appender | `WARN~MAX` |

**Usage notes:**
- **In synchronous mode, bound Appenders must be concurrency-safe**.
  Because multiple goroutines may call the same SyncLogger at the same time, the underlying Appender's `Append` method will be called concurrently, so concurrency safety must be guaranteed.
- If an Appender itself does not guarantee concurrency safety (for example, file writing without locks), it can be used together with an outer `AsyncLogger`,
  where the asynchronous Logger's single goroutine consumer guarantees serialized writes.
- Synchronous mode writes block business goroutines. In high-concurrency scenarios, prefer `AsyncLogger`.

### AsyncLogger (Asynchronous)

`AsyncLogger` is a Logger implementation designed for high-concurrency production environments.
It completely decouples log "production" from log "writing":
business goroutines only need to put log events into an in-memory buffer and return immediately,
while actual field encoding and Appender writing are completed asynchronously by an independent background goroutine.

This "producer-consumer" model brings two main advantages:
- **Business requests are not affected by IO jitter**: slow IO operations such as disk flushes and network latency do not block business threads, making request response times more stable
- **Higher write throughput**: background single-goroutine serial writes can eliminate concurrent lock overhead, and batch writes can better utilize system caches

The following configuration example shows a typical asynchronous Logger configuration for business logs:

**Complete configuration example:**

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout

logger.async.type = AsyncLogger
logger.async.tag = _app_*
logger.async.level = INFO
logger.async.appenderRef[0].ref = console
logger.async.appenderRef[1].ref = file
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `tag` | Yes | Tag expression bound to the Logger | `_app_*`, `_biz_order_create` |
| `level` | Yes | Log level or level range | `INFO`, `DEBUG~WARN` |
| `bufferSize` | No | Buffer size, default `10000` | `10000`, `50000` |
| `onBufferFull` | No | Strategy when the buffer is full, default `block` | `block`, `discard`, `drop-oldest` |
| `appenderRef[n].ref` | Yes | Referenced Appender name | `console`, `file` |
| `appenderRef[n].level` | No | Level filtering range for this Appender | `WARN~MAX` |

**Buffer full strategies:**

| Strategy | Behavior | Applicable Scenarios |
|----------|----------|----------------------|
| `block` | Business goroutine blocks until there is space in the buffer | Logs must not be lost, and business request latency is acceptable |
| `discard` | Directly discard newly arrived log events | Business performance first, log loss allowed in extreme cases |
| `drop-oldest` | Discard the oldest event in the buffer to make room for new logs | Troubleshooting focuses more on the latest on-site information |

**Usage notes:**
- Buffer size should be estimated according to log generation rate and peak duration. In high-concurrency scenarios, increase it to 20000-50000
- When the process performs a normal `Stop`, AsyncLogger triggers graceful shutdown and does its best to write buffered events;
  however, if the process is forcibly killed (such as `kill -9`), unwritten logs in the buffer may still be lost
- Because AsyncLogger internally guarantees single-goroutine serial writes, bound Appenders do not need to be concurrency-safe
- In asynchronous mode, log output has millisecond-level delay. Pay attention to ordering issues during debugging

### ConsoleLogger

`ConsoleLogger` is the most commonly used integrated Logger, designed specifically for standard output (stdout) scenarios and ready to use out of the box.
It integrates `ConsoleAppender` and `TextLayout` internally. No extra Appender configuration is required; a simple set of configuration enables console output.

**Configuration example:**

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = TextLayout
logger.console.layout.fileLineMaxLength = 30
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `tag` | Yes | Tag expression bound to the Logger | `_app_*`, `_biz_order_create` |
| `level` | Yes | Log level or level range | `INFO`, `DEBUG~WARN` |
| `layout.*` | No | Output format configuration, default `TextLayout` | |

It is the preferred Logger for local development and debugging, and also the default configuration solution when setting up new projects.

**Usage notes:**
- **In high-concurrency production scenarios, heavy console output may become a performance bottleneck**.
  Because standard output is a system-wide shared resource, the kernel protects it with locks during concurrent writes. Many goroutines writing to the console at the same time may cause severe lock contention.
- In production, prefer `FileLogger` or `RollingFileLogger` to write local files, and only output key information during startup to the console.
- In container environments, JSON output is recommended so collectors can directly ingest structured data.

### FileLogger

`FileLogger` is an integrated Logger for single-file output. It directly encapsulates `FileAppender` internally,
so it can write to a local file without extra Appender configuration.
It uses machine-parseable `JSONLayout` by default, and can be switched to `TextLayout` as needed.

It is more suitable for the following scenarios:
- Small services or monolithic applications with low log volume and no need for time-based rolling
- Automated testing and CI environments where log files need to be collected for assertions and analysis
- Temporary debugging scenarios where logs for a specific tag are quickly output to an independent file

**Configuration example:**

```properties
logger.file.type = FileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
logger.file.layout.fileLineMaxLength = 60
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `tag` | Yes | Tag expression bound to the Logger | `_app_*`, `_biz_order_create` |
| `level` | Yes | Log level or level range | `INFO`, `DEBUG~WARN` |
| `dir` | Yes | Log file directory, automatically created on startup | `./logs`, `/var/log/app` |
| `file` | Yes | Log file name | `app.log`, `audit.log` |
| `layout.*` | No | Output format configuration, default `JSONLayout` | |

**Usage notes:**
- Files are not rolled automatically. Logs are continuously appended to the same file, which may become too large during long-running operation
- If time-based splitting and automatic cleanup of expired logs are needed, use `RollingFileLogger`
- The log directory must ensure the application process has write permissions; otherwise startup reports an error

### RollingFileLogger

`RollingFileLogger` is a full-featured integrated Logger designed for production environments and is the recommended choice for most online services.
It internally encapsulates `RollingFileAppender`, supports automatic time-based file rolling, and can split ordinary logs and alert logs by level.
It also has built-in asynchronous writing capability, satisfying most production needs out of the box.

Its core features include:
- **Time-driven rolling**: automatically create new log files at fixed time intervals to avoid oversized single files
- **Automatic expiration cleanup**: log files older than the retention period are automatically deleted without external cleanup scripts
- **Level-separated output**: ordinary logs and alert logs are stored separately, so alert files can be checked first during troubleshooting
- **Built-in asynchronous support**: enable asynchronous writing through configuration without wrapping another `AsyncLogger` outside

The following configuration example shows a typical production configuration:

**Production configuration example:**

```properties
logger.file.type = RollingFileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
logger.file.layout.fileLineMaxLength = 60
logger.file.interval = 24h
logger.file.maxAge = 168h
logger.file.separate = true
logger.file.async = true
logger.file.bufferSize = 50000
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `tag` | Yes | Tag expression bound to the Logger | `_app_*`, `_biz_order_create` |
| `level` | Yes | Log level or level range | `INFO`, `DEBUG~WARN` |
| `dir` | Yes | Log file directory, automatically created on startup | `./logs`, `/var/log/app` |
| `file` | Yes | Log file name prefix | `app.log`, `audit.log` |
| `interval` | No | Rolling interval, default `1h`, aligned to whole hours | `1h`, `24h`, `168h` |
| `maxAge` | No | Maximum log retention time, default `168h` (7 days) | `24h`, `168h`, `720h` |
| `separate` | No | Whether to enable level-separated output, default `false` | `true`, `false` |
| `async` | No | Whether to enable built-in asynchronous writing, default `false` | `true`, `false` |
| `bufferSize` | No | Asynchronous buffer size, effective when async is enabled | `10000`, `50000` |
| `layout.*` | No | Output format configuration, default `JSONLayout` | |

**Usage notes:**
- `interval` rolling interval: choose according to log volume. Use `1h` for high-traffic services and `24h` for ordinary services
- `maxAge` retention time: decide according to compliance requirements and disk capacity, usually 7 days (168h) or 30 days
- `separate = true` level separation: after enabling, ordinary logs are written to `app.log.<time>`, and logs at `WARN` and above are written to `app.log.wf.<time>`, which can greatly improve troubleshooting efficiency
- `async = true` asynchronous writing: after enabling, the internal asynchronous writing mechanism is used directly, with no need to wrap another `AsyncLogger`
- `bufferSize` buffer size: in high-concurrency scenarios, increase it to 20000-50000 to avoid buffer-full discards during peaks

### Custom Logger

Go-Spring logging's plugin-based architecture allows users to customize Logger implementations to meet special needs of business systems.

The following implements a custom Logger based on percentage sampling:

**Complete implementation code:**

```go
type SamplingLogger struct {
	log.AsyncLogger

	SampleRate float64 `PluginAttribute:"sampleRate,default=0.01"`
	rand       *rand.Rand
}

func (l *SamplingLogger) Start() error {
	l.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return l.AsyncLogger.Start()
}

func (l *SamplingLogger) Stop() {
	l.AsyncLogger.Stop()
}

func (l *SamplingLogger) Append(e *log.Event) {
	// Keep all ERROR and higher-level logs
	if e.Level.Code() >= log.ErrorLevel.Code() {
		l.AsyncLogger.Append(e)
		return
	}

	// Keep lower-level logs according to the sampling rate
	if l.rand.Float64() < l.SampleRate {
		e.Fields = append(e.Fields, log.Bool("sampled", true))
		l.AsyncLogger.Append(e)
	}
}

func init() {
	log.RegisterPlugin[SamplingLogger]("SamplingLogger")
}
```

**Configuration usage example:**

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout

logger.sampling.type = SamplingLogger
logger.sampling.tag = _app_*
logger.sampling.level = INFO
logger.sampling.appenderRef[0].ref = file
logger.sampling.sampleRate = 0.01
```

The sampling Logger in the example reuses the full asynchronous writing capability through composition (embedding `log.AsyncLogger`),
and only adds sampling filtering logic at the `Append` entry point.
This composition approach allows custom extensions to focus only on differentiated logic, without reimplementing complex basic functions such as buffer management, concurrency control, and graceful shutdown.

---

## Appender

Appender is the final execution unit for log persistence. It writes encoded log events to various target systems.
By binding multiple Appenders to one Logger at the same time, you can flexibly implement a "one source, multiple outputs" fan-out strategy.
For example, the same error log can be persisted to a local file and also pushed in real time to an alerting platform or monitoring system.

Go-Spring has four built-in core Appenders, covering all scenarios for backend services from development and testing to production:

| Appender Type | Output Target |
|---------------|---------------|
| `DiscardAppender` | Null output. All log events are silently discarded without any persistence or IO operations |
| `ConsoleAppender` | Process standard output (stdout), printed directly to a terminal or container log collection channel |
| `FileAppender` | A single local file. Logs are continuously appended and are not automatically rolled or cleaned up |
| `RollingFileAppender` | A sequence of files automatically rolled by time, with support for automatic cleanup of expired log files |

### DiscardAppender

`DiscardAppender` silently discards all written log events and produces no actual output.
It is rarely used in most cases and is only used in certain special testing scenarios.

```properties
appender.discard.type = DiscardAppender
```

### ConsoleAppender

`ConsoleAppender` writes encoded log events directly to the process standard output stream (stdout),
and is the preferred output target for local development and debugging scenarios.
It is usually used with `TextLayout` to output human-readable formatted text.

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `layout.*` | No | Output format configuration, default `TextLayout` | |

**Usage notes:**
- Standard output is a system-wide shared resource. During concurrent writes, the kernel protects it with locks. Many goroutines writing to the console at the same time may cause severe lock contention and degrade performance.
- In high-concurrency production scenarios, it is recommended to output key information only during startup and write business logs uniformly to local files.
- In container environments, JSON output is recommended so log collectors can directly ingest structured data.

### FileAppender

`FileAppender` continuously writes logs in append mode to a single local file. It is the most basic and reliable persistent output solution.
It uses operating-system-level append semantics to guarantee the atomic integrity of each log line. Even if the process crashes abnormally, already written logs are not corrupted.

**It is suitable for the following scenarios:**
- **Small low-traffic services**: daily log volume is not large and time-based rolling is not needed
- **CI/CD test environments**: after tests complete, log files can be used for assertion analysis and issue tracing
- **Targeted debugging output**: logs for specific tags are output to independent files, avoiding mixing with other logs
- **Short-lived processes**: scheduled tasks, CLI tools, and so on, where logs are fully retained after the run ends
- **Audit compliance logs**: audit data that needs permanent archiving, where independent files are convenient for backup management

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `dir` | Yes | Log directory | `./logs`, `/var/log/app` |
| `file` | Yes | Log file name | `app.log`, `audit.log` |
| `layout.*` | No | Output format, default `TextLayout` | |

**Usage notes:**
- Directory write permissions are checked before startup. Insufficient permissions cause direct failure, avoiding silent log loss during runtime
- Files are not automatically rolled. Long-running services may cause a single file to become too large
- No automatic cleanup mechanism is provided. Accumulated historical logs need external scripts such as logrotate or manual periodic processing
- When multiple goroutines write concurrently, it is recommended to use `AsyncLogger` to guarantee serialization
- When log volume grows and rolling is needed, you can seamlessly switch to `RollingFileAppender`; the configuration is highly compatible

### RollingFileAppender

`RollingFileAppender` is a full-featured persistent output component specifically designed for production environments and is the preferred solution for most online services.
It automatically rolls files at fixed time intervals and has built-in automatic cleanup for expired logs, completely solving the operations problem caused by unlimited growth of a single file.
Compared with `FileAppender`, it adds lifecycle management capabilities and achieves a better balance among reliability, performance, and operability.

```properties
appender.rolling.type = RollingFileAppender
appender.rolling.dir = ./logs
appender.rolling.file = app.log
appender.rolling.interval = 1h
appender.rolling.maxAge = 168h
appender.rolling.syncLock = false
appender.rolling.layout.type = JSONLayout
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `dir` | No | Root directory for log files, default `./logs`; all rolled files are created under this directory | `./logs`, `/data/logs/app` |
| `file` | Yes | Log file name prefix; a timestamp suffix is automatically appended during rolling | `app.log`, `audit.log` |
| `interval` | No | Rolling interval, default `1h`, supports hour-level precision and uses whole-hour alignment | `1h`, `6h`, `24h` |
| `maxAge` | No | Maximum log retention time, default `168h` (7 days); files older than this are automatically deleted on the next roll | `24h`, `168h`, `720h` |
| `syncLock` | No | Whether to enable built-in mutex lock, default `false`; enabling guarantees concurrency safety for multiple writers | `true`, `false` |
| `layout.*` | No | Output format configuration, default `TextLayout` | |

**Usage notes:**
- Rolling time uses a whole-hour alignment strategy, ensuring rolling times are consistent across multiple instances and making log collection and management easier.
  For example, with `interval = 1h`, the first roll occurs at 0 minutes 0 seconds of the next hour, and then rolls exactly on the hour afterward.
- In synchronous Logger scenarios with multiple goroutines writing, enabling `syncLock = true` lock protection is recommended
- When used with `AsyncLogger`, it is recommended to set `syncLock = false`; the asynchronous Logger's single goroutine consumer guarantees serialized writes and avoids lock overhead
- When multiple processes write to the same log directory simultaneously, `syncLock = true` must be enabled; otherwise log content may interleave
- Log retention time should be planned reasonably according to disk capacity to avoid exhausting storage space with log files

### Custom Appender

Go-Spring logging's plugin-based architecture allows users to customize Appenders and output logs to various target systems.

The following implements a custom Appender based on percentage sampling:

```go
type SamplingAppender struct {
	log.FileAppender

	SampleRate float64 `PluginAttribute:"sampleRate,default=0.01"`
	rand       *rand.Rand
}

func (a *SamplingAppender) Start() error {
	a.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return a.FileAppender.Start()
}

func (a *SamplingAppender) Stop() {
	a.FileAppender.Stop()
}

func (a *SamplingAppender) Append(e *log.Event) {
	// Keep all ERROR and higher-level logs
	if e.Level.Code() >= log.ErrorLevel.Code() {
		a.FileAppender.Append(e)
		return
	}

	// Keep lower-level logs according to the sampling rate
	if a.rand.Float64() < a.SampleRate {
		e.Fields = append(e.Fields, log.Bool("sampled", true))
		a.FileAppender.Append(e)
	}
}

func (a *SamplingAppender) ConcurrentSafe() bool {
	return a.FileAppender.ConcurrentSafe()
}

func init() {
	log.RegisterPlugin[SamplingAppender]("SamplingAppender")
}
```

**Configuration usage example:**

```properties
appender.sampling.type = SamplingAppender
appender.sampling.dir = ./logs
appender.sampling.file = app.log
appender.sampling.layout.type = TextLayout
appender.sampling.sampleRate = 0.01

logger.sync.type = SyncLogger
logger.sync.tag = _app_*
logger.sync.level = INFO
logger.sync.appenderRef[0].ref = sampling
```

This example is similar to the sampling Logger above, except the sampling functionality has been moved to the Appender layer.

---

## Layout

Layout is the formatting layer for log events. It determines how log events are encoded into the final byte stream for output.
It does not care where logs come from or where they are ultimately written. It focuses on one thing: converting structured `Event` objects into the target format.

Layout uses a streaming encoding design. Fields are written directly to the output `Writer`, avoiding the overhead of first constructing complete intermediate objects and then serializing them.
This streaming architecture can significantly reduce GC pressure under high concurrency.

Go-Spring provides two built-in Layout implementations: `TextLayout` and `JSONLayout`.

### TextLayout

`TextLayout` outputs human-readable single-line text and is the default choice for local development and console troubleshooting.
It uses `||` as the segment separator, avoiding conflicts with spaces, commas, equal signs, and other characters that may appear in log content.

Its output format is as follows:

```text
[Level][Time][File:Line] Tag||Context string||key1=value1||key2=value2||msg=Log message
```

Meaning of each segment:

| Segment | Description |
|---------|-------------|
| `[Level]` | Log level, such as INFO / WARN / ERROR |
| `[Time]` | Event occurrence time, default format includes millisecond precision |
| `[File:Line]` | Source file and line number of the log call site; the head is automatically truncated if too long |
| `Tag` | Semantic tag of the log, used for routing matching |
| `Context string` | Trace identifier extracted by the `StringFromContext` hook |
| `key=value` fields | Structured fields, output in the order they were added |

Configuration example:

```properties
appender.console.layout.type = TextLayout
appender.console.layout.fileLineMaxLength = 48
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `fileLineMaxLength` | No | Maximum displayed length of the file path. If exceeded, the head is truncated with `...`; default `48` | `30`, `60` |

---

### JSONLayout

`JSONLayout` outputs standard single-line JSON and is the recommended configuration for production environments.
It preserves the original type information of fields, making it easy for logging systems to directly index, aggregate, and retrieve.

Configuration example:

```properties
appender.console.layout.type = JSONLayout
appender.console.layout.fileLineMaxLength = 48
```

| Configuration Item | Required | Description | Example |
|--------------------|----------|-------------|---------|
| `fileLineMaxLength` | No | Maximum displayed length of the file path. If exceeded, the head is truncated with `...`; default `48` | `30`, `60` |

JSONLayout does not use the general-purpose serialization from the standard library `encoding/json`; instead, it uses a dedicated encoder for log fields,
so it has significant advantages in performance and memory allocation.

---

### Custom Layout Extension

Go-Spring logging Layouts are fully pluggable.
When other formats are needed (such as CSV, Protobuf, or custom delimiters), simply implement the `Layout` interface and register it with the plugin system, then it can be enabled in configuration.

The following implements a custom Layout that outputs CSV format:

```go
// CSVLayout outputs comma-separated CSV format
type CSVLayout struct {
	log.BaseLayout
}

func (l *CSVLayout) EncodeTo(e *log.Event, w log.Writer) {
	// Escape CSV special characters
	escape := func(s string) string {
		if strings.ContainsAny(s, ",\"\n") {
			return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
		}
		return s
	}

	w.WriteString(e.Level.UpperName())
	w.WriteByte(',')
	w.WriteString(e.Time.Format("2006-01-02T15:04:05.000"))
	w.WriteByte(',')
	w.WriteString(escape(l.GetFileLine(e)))
	w.WriteByte(',')
	w.WriteString(e.Tag)
	w.WriteByte(',')
	if e.CtxString != "" {
		_, _ = w.WriteString(escape(e.CtxString))
		_, _ = w.WriteString(",")
	}

	var buf bytes.Buffer
	enc := log.NewTextEncoder(&buf, " ")
	enc.AppendEncoderBegin()
	log.EncodeFields(enc, e.CtxFields)
	log.EncodeFields(enc, e.Fields)
	enc.AppendEncoderEnd()

	w.WriteString(escape(buf.String()))
	w.WriteByte('\n')
}

func init() {
	log.RegisterPlugin[CSVLayout]("CSVLayout")
}
```

**Configuration usage example:**

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = CSVLayout
```

---

## Encoder

Encoder is the field encoding layer, responsible for writing `Field` as text or JSON byte streams. Its design goals are:

- Primitive types use dedicated encoding paths to reduce reflection
- Fields carry type information, so no re-inference is needed during encoding
- Write directly to the target `Writer`, reducing intermediate objects

Business code usually does not need to operate Encoder directly. Only when implementing a custom Layout do you need to select and combine Encoders.

---

## Context Extraction

Context extraction is one of the core functions of a microservice logging system, used to answer the question "which request or trace does this log belong to".

In a distributed system, a request may pass through multiple services and components, producing a large number of logs.
Without a unified trace identifier, troubleshooting cannot connect these logs together at all.
The core goal of context extraction is: **let every log automatically carry trace identifiers without requiring business code to pass them manually**.

Common context fields include:

| Field Name | Description | Typical Scenario |
|------------|-------------|------------------|
| `trace_id` | Global trace ID, spanning the entire request lifecycle | Distributed tracing |
| `span_id` | Current span ID, identifying a single call or operation | Subdividing trace stages |
| `request_id` | HTTP request ID generated by the gateway or entry layer | Web service troubleshooting |
| `user_id` | Current operating user ID | User behavior audit |
| `client_ip` | Client IP address | Security analysis and traffic statistics |
| `tenant_id` | Tenant identifier in multi-tenant scenarios | SaaS system isolation |

If business code manually passed these fields at every log call, it would not only be repetitive and cumbersome, but also very easy to omit.
Go-Spring provides two global hook functions that can automatically extract context information from `context.Context`
and inject it into every output log.

---

### FieldsFromContext

The `FieldsFromContext` hook is used to extract multiple structured fields and returns `[]log.Field`.
The extracted fields are injected into the final log event and are treated the same as fields added by business code.

This is the **recommended preferred** extraction method because it preserves field type information, making it convenient for logging systems to directly index and aggregate.

#### Basic usage example

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
	var fields []log.Field

	// Extract trace fields
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		fields = append(fields, log.String("trace_id", traceID))
	}
	if spanID, ok := ctx.Value("span_id").(string); ok {
		fields = append(fields, log.String("span_id", spanID))
	}

	// Extract business context fields
	if userID, ok := ctx.Value("user_id").(int64); ok {
		fields = append(fields, log.Int("user_id", userID))
	}
	if requestID, ok := ctx.Value("request_id").(string); ok {
		fields = append(fields, log.String("request_id", requestID))
	}

	return fields
}
```

After setup, business code only needs to call the logging API normally:

```go
// Business code does not need to explicitly pass fields such as trace_id and user_id
log.Info(ctx, TagBizOrder,
    log.String("order_no", "ORD001"),
    log.Msg("order created successfully"),
)
```

The final output log automatically includes context fields:

```json
{
    "level": "info",
    "time": "2026-05-03T11:27:16.214",
    "fileLine": ".../myapp/main.go:46",
    "tag": "_biz_order",
    "trace_id": "123456",
    "span_id": "123456",
    "request_id": "123456",
    "order_no": "ORD001",
    "msg": "order created successfully"
}
```

#### OpenTelemetry integration example

This is a common integration approach in production environments, directly extracting trace information from OTel Context:

```go
import "go.opentelemetry.io/otel/trace"

log.FieldsFromContext = func(ctx context.Context) []log.Field {
	var fields []log.Field

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID()
		spanID := span.SpanContext().SpanID()

		fields = append(fields,
			log.String("trace_id", traceID.String()),
			log.String("span_id", spanID.String()),
		)

		// If the span has a sampled marker, it can also be extracted
		if span.SpanContext().IsSampled() {
			fields = append(fields, log.Bool("sampled", true))
		}
	}

	return fields
}
```

---

### StringFromContext

The `StringFromContext` hook is used to extract one already formatted string.
When used with `TextLayout`, it needs to comply with the format requirements of `TextLayout`;
when used with `JSONLayout`, it needs to comply with the format requirements of `JSONLayout`.

#### Usage example

```go
type traceCtxType struct{}

log.StringFromContext = func(ctx context.Context) string {
	trace, _ := ctx.Value(traceCtxType{}).(string)
	return trace
}
```

### Performance Notes

These two hook functions are executed **on every log output**, so their execution cost must be strictly controlled.
For example, all values that need to be extracted can be placed into `ctx` once at the request entry point, and extraction only performs simple type assertions and reads.
At the same time, avoid creating new objects inside hooks as much as possible, especially in high-frequency call scenarios.
If there is `if` logic inside the hook, put the most common and most likely existing fields first.

The following operations are absolutely forbidden:
- **No complex computation**: do not perform string concatenation, hash calculation, serialization, or similar operations inside hooks.
- **No network or disk IO**: absolutely do not call external APIs or read files inside hooks.
- **No lock operations**: do not acquire mutexes or perform synchronization operations inside hooks.
- **No reflection**: do not traverse values in Context through reflection.

---

## Configuration System

The Go-Spring logging system uses a **flattened KV configuration model**. Whether from configuration files, environment variables, configuration centers, or command-line arguments,
configuration can be modeled uniformly, and configurations from different sources can be merged by priority.

### Configuration Categories

Go-Spring logging system configuration is divided into three major categories by namespace:

**1. `logger.*` — Logger instance configuration**

Each Logger instance is prefixed with `logger.<name>.` in configuration. `<name>` is a custom instance name used to identify different Loggers.
Each Logger must configure the `type` field to specify the plugin type, such as `AsyncLogger`, `ConsoleLogger`, and so on.
Except for the root Logger, other Loggers must configure the `tag` field to specify the bound tag expression.
It should be specially noted that the root logger does not need to bind any tag because it is the fallback Logger instance.

**Typical example**:

```properties
logger.async.type = AsyncLogger
logger.async.tag = _app_*
logger.async.level = INFO
logger.async.appenderRef[0].ref = console
logger.async.appenderRef[1].ref = file
```

---

**2. `appender.*` — Appender instance configuration**

Each Appender instance is prefixed with `appender.<name>.` in configuration. `<name>` is a custom instance name referenced by a Logger's `appenderRef`.
Each Appender must configure the `type` field to specify the plugin type, such as `ConsoleAppender`, `RollingFileAppender`, and so on.
Each Appender can embed its own Layout configuration.

**Typical example**:

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

---

**3. Global variables and custom properties**

Configuration items without a namespace prefix are treated as global variables and can be referenced in other configuration using `${key}` syntax.
By referencing them in multiple places, changing one value can take effect globally.

**Typical example**:

```properties
log.dir = /var/log/app
log.level = INFO
log.retention = 168h

appender.file.dir = ${log.dir}
appender.rolling.maxAge = ${log.retention}
logger.root.level = ${log.level}
```

---

### Log Levels

The log level configuration item `level` supports two expression styles:
- A single level means output all logs at that level and above;
- A level range uses the left-closed, right-open interval `[MinLevel, MaxLevel)`, separated by `~`, and outputs only logs within this interval.

```properties
# Single level: output INFO and above
logger.root.level = INFO

# Range: output WARN, ERROR, PANIC (excluding FATAL)
logger.error_only.level = WARN~FATAL

# Range: output DEBUG, INFO (excluding WARN)
logger.debug_info.level = DEBUG~WARN
```

### Array Configuration

Configuration item arrays have two configuration methods, which can be selected according to content complexity: index-based style and comma-separated style.

#### Method 1: Index-based style (general)

This style is suitable for object arrays or complex structures:

```properties
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[0].level = DEBUG~WARN
logger.root.appenderRef[1].ref = file
logger.root.appenderRef[1].level = INFO~MAX
logger.root.appenderRef[2].ref = kafka
logger.root.appenderRef[2].level = ERROR~MAX
```

#### Method 2: Comma-separated style (simple strings)

This style is suitable for simple string lists:

```properties
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
```

Equivalent to:

```properties
logger.biz.tag[0]=_biz_order_*
logger.biz.tag[1]=_biz_user_*
logger.biz.tag[2]=_biz_pay_*
```

---

### Plugin Injection

All core components (Logger, Appender, Layout) in the Go-Spring logging system are managed through the plugin mechanism.
The plugin mechanism uniformly encapsulates component configuration parsing, type conversion, instance creation, and lifecycle management, making the logging system highly extensible.

Plugin configuration is declaratively injected into plugin instances through Struct Tags, mainly implemented by two tags:
- `PluginAttribute` is used to inject primitive type properties such as strings, numbers, booleans, and durations
- `PluginElement` is used to inject nested plugin instances and supports recursive composition

This design allows plugin authors to only declare field metadata without manually writing any configuration parsing code.

---

#### Regular Attribute Injection

`PluginAttribute` is used to inject primitive type properties such as strings, numbers, booleans, and durations.

**Tag syntax**: `attributeName,default=defaultValue`

- `attributeName`: required, the corresponding key name in the configuration file
- `default=defaultValue`: optional, the default value used when not configured

```go
type RollingFileAppender struct {
	log.AppenderBase

	FileDir  string        `PluginAttribute:"dir,default=./logs"`
	FileName string        `PluginAttribute:"file"` // Required field, no default value
	Interval time.Duration `PluginAttribute:"interval,default=1h"`
	MaxAge   time.Duration `PluginAttribute:"maxAge,default=168h"`
	SyncLock bool          `PluginAttribute:"syncLock,default=false"`
}
```

Go struct field names remain consistent with configuration key names, and camelCase naming is usually recommended.

---

#### Child Plugin Injection

`PluginElement` is used to inject nested plugin instances and supports recursive composition.
Child plugins themselves can also contain `PluginAttribute` and other `PluginElement` fields.

**Tag syntax**: `childPluginPrefix,default=defaultPluginType`

```go
type ConsoleLogger struct {
	log.LoggerBase

	Layout log.Layout `PluginElement:"layout,default=TextLayout"`
}
```

Configuration keys for child plugins are automatically prefixed with the prefix specified in the Tag, forming a hierarchical structure:

```properties
# Configuration for the parent plugin ConsoleLogger
logger.console.type = ConsoleLogger

# Configuration for the child plugin Layout, automatically prefixed with .layout
logger.console.layout.type = JSONLayout
logger.console.layout.fileLineMaxLength = 60
```

This nesting mechanism supports plugin composition at any depth, for example:
- Logger embeds an Appender array
- Appender embeds Layout
- Layout embeds Encoder

---

#### Plugin Registration

Plugins must be registered through the `log.RegisterPlugin` function before use, so the configuration system can create instances according to type names.

```go
// Custom plugin type
type SamplingAppender struct {
	log.FileAppender

	SampleRate float64 `PluginAttribute:"sampleRate,default=0.01"`
	rand       *rand.Rand
}

// Register plugin
func init() {
	log.RegisterPlugin[SamplingAppender]("SamplingAppender")
}
```

#### Lifecycle Management

Plugin lifecycle is uniformly managed by the logging system, throughout the entire process from configuration loading to runtime.
Plugins can implement `Start()` and `Stop()` methods.
The former is called after configuration injection completes and is used to initialize resources (such as creating connections or initializing random seeds),
and the latter is called when the system shuts down and is used to gracefully release resources (such as closing connections or flushing buffers).

```go
// Start is called after configuration injection completes and is used to initialize resources
func (l *SamplingLogger) Start() error {
	l.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return l.AsyncLogger.Start()
}

// Stop is called when the system shuts down and is used to clean up resources
func (l *SamplingLogger) Stop() {
	l.AsyncLogger.Stop()
}
```

---

#### Type Converters

If a plugin needs custom configuration types, it can use type converters. A type converter converts configuration strings into target types.
The Go-Spring logging system provides the `log.RegisterConverter` function for registering custom converters.

Type converters must be pure functions with no side effects, and the same input must always produce the same output. The function signature is:

```go
type Converter[T any] func(string) (T, error)
```

The log configuration level `LevelRange` is a custom type.

---

## Error Handling

Usually, when log write errors occur, we should not write logs again to files or the console. Instead, we should report errors to other systems and trigger alerts.
Error handling in logging systems has a special nature: when log writing fails, this error cannot be recorded again through the logging system,
otherwise it may cause infinite recursion and increase system burden. Sometimes it cannot even be written to standard error.

Go-Spring uses a **global error callback** design and reports write errors uniformly through the `log.ReportError` function.
This callback is only triggered when Event writing fails and is not called in other scenarios.

When implementing the callback, avoid time-consuming operations in the error handler and absolutely do not produce panic in the callback function.

```go
log.ReportError = func(err error) {
	// Report errors to other systems, such as an alerting system
	metric.Incr("log_write_error_total")
}
```

---

## Configuration Refresh

Go-Spring provides two functions, `log.Refresh` and `log.RefreshConfig`, for refreshing log configuration.
The former reads configuration from `flatten.Storage`, and the latter loads configuration from a flattened map.

The Go-Spring application framework uses `log.Refresh` to refresh log configuration.
If using the logging component independently, `log.RefreshConfig` is recommended.

```go
err := log.RefreshConfig(map[string]string{
	"appender.console.type":        "ConsoleAppender",
	"appender.console.layout.type": "TextLayout",

	"appender.file.type":        "FileAppender",
	"appender.file.dir":         "./logs",
	"appender.file.file":        "app.log",
	"appender.file.layout.type": "JSONLayout",

	"logger.sync.type":               "SyncLogger",
	"logger.sync.tag":                "_app_*",
	"logger.sync.level":              "INFO",
	"logger.sync.appenderRef[0].ref": "console",
	"logger.sync.appenderRef[1].ref": "file",
})
```

---

## Framework Adaptation

In addition to directly using Go-Spring's native logging API, existing logging entry points in a project can also be gradually connected to this logging system.

### GetLogger

Traditional logging frameworks usually distinguish loggers by name. For example, a logger named after the program is used to output application and business logs,
and a logger named rpc is used to output RPC call logs.

The Go-Spring logging system provides the `log.GetLogger` function for getting loggers by name, suitable for third-party library adaptation or project migration.

```go
rootLogger := log.GetLogger("root")
rootLogger.Write(log.InfoLevel, []byte("hello world\n"))
```

We need to define a Logger with the same name in configuration; otherwise an error is reported. For example, with the code above, we can provide the following configuration:

```properties
logger.root.type = FileLogger
logger.root.level = INFO
logger.root.dir = ./logs
logger.root.file = app.log
logger.root.layout.type = JSONLayout
logger.root.layout.fileLineMaxLength = 60
```

### Adapting the Standard Library log

The Go standard library's `log` package is the most basic and widely used logging library, and many third-party dependencies use it to output logs.
Unifying standard library log into the Go-Spring logging system can centralize log management for the entire application and avoid scattered log outputs.

The standard library log provides `io.Writer` as an output extension point. We only need to implement this interface and replace the default output through `SetOutput` to complete adaptation.

The following is a complete adaptation implementation and usage example:

```go
package main

import (
	"context"
	stdlog "log"

	"github.com/go-spring/log"
)

// StdLogWriter implements the io.Writer interface and forwards standard library log output to the Go-Spring logging system.
type StdLogWriter struct {
	logger *log.LoggerWrapper
}

// Write implements the io.Writer interface and writes log content to the Go-Spring Logger at INFO level.
func (w *StdLogWriter) Write(p []byte) (int, error) {
	w.logger.Write(log.InfoLevel, p)
	return len(p), nil
}

func main() {
	// Replace the default output target of the standard library log. All subsequent logs output through stdlog are forwarded to Go-Spring.
	stdlog.SetOutput(&StdLogWriter{
		logger: log.GetLogger("root"),
	})

	// Initialize Go-Spring logging configuration.
	err := log.RefreshConfig(map[string]string{
		"logger.root.type":                     "FileLogger",
		"logger.root.level":                    "INFO",
		"logger.root.dir":                      "./logs",
		"logger.root.file":                     "app.log",
		"logger.root.layout.type":              "JSONLayout",
		"logger.root.layout.fileLineMaxLength": "20",
	})
	if err != nil {
		stdlog.Printf("log refresh config error: %v", err)
		return
	}

	// Output logs through the standard library log.
	stdlog.Println("hello from standard log")

	// Output logs using the native Go-Spring logging API, verifying that the two can coexist.
	log.Info(context.Background(), log.TagAppDef, log.String("user", "alice"))
}
```

After executing the example, standard library log output is forwarded through the adapter to the Go-Spring logging system and finally written to the `./logs/app.log` file.
The output contains two formats of logs:

```text
2026/05/03 22:27:11 hello from standard log
{"level":"info","time":"2026-05-03T22:27:11.584","fileLine":".../myapp/main.go:46","tag":"_app_def","user":"alice"}
```

As you can see, the first entry is a plain-text log generated by the standard library log, and the second entry is a structured log generated natively by Go-Spring.
Both types of logs ultimately converge to the same output target, achieving unified log management.

### Adapting Zap

Using the same idea as adapting standard library log, we can implement Zap's core interface to complete adaptation.
The Zap framework provides the `zapcore.Core` interface as the core abstraction for log writing. By implementing this interface,
Zap-generated log events are forwarded to the Go-Spring logging system.

The following is a complete adaptation implementation and usage example:

```go
package main

import (
	"context"
	stdlog "log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-spring/log"
)

// ZapGoSpringWriter is a zapcore.Core adapter
// used to forward Zap logs to the Go-Spring logging system.
type ZapGoSpringWriter struct {
	logger  *log.LoggerWrapper
	fields  []zapcore.Field
	Encoder zapcore.Encoder
}

// NewZapGoSpringWriter creates a Zap Core adapted to a Go-Spring Logger.
func NewZapGoSpringWriter(logger *log.LoggerWrapper) zapcore.Core {
	return &ZapGoSpringWriter{
		logger:  logger,
		Encoder: zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
	}
}

// Enabled determines whether the current log level is allowed to output.
func (c *ZapGoSpringWriter) Enabled(level zapcore.Level) bool {
	// Delegate to Go-Spring's level check
	return c.logger.Enable(toGoSpringLevel(level))
}

// With appends structured fields and returns a new Core instance.
func (c *ZapGoSpringWriter) With(fields []zapcore.Field) zapcore.Core {
	clone := &ZapGoSpringWriter{
		logger:  c.logger,
		fields:  append(c.fields, fields...),
		Encoder: c.Encoder,
	}
	return clone
}

// Check determines whether the log entry needs to be written.
func (c *ZapGoSpringWriter) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// Write encodes a Zap log entry and forwards it to the Go-Spring Logger.
func (c *ZapGoSpringWriter) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// Encode Zap fields in text format
	buf, err := c.Encoder.EncodeEntry(entry, append(c.fields, fields...))
	if err != nil {
		log.ReportError(err) // Report file write error
		return err
	}

	// Forward to Go-Spring Logger
	level := toGoSpringLevel(entry.Level)
	c.logger.Write(level, buf.Bytes())
	return nil
}

// Sync flushes the log buffer.
func (c *ZapGoSpringWriter) Sync() error {
	// Go-Spring Logger is managed by its own system, so no extra handling is needed here.
	return nil
}

// toGoSpringLevel maps Zap log levels to Go-Spring log levels.
func toGoSpringLevel(level zapcore.Level) log.Level {
	switch level {
	case zapcore.DebugLevel:
		return log.DebugLevel
	case zapcore.InfoLevel:
		return log.InfoLevel
	case zapcore.WarnLevel:
		return log.WarnLevel
	case zapcore.ErrorLevel:
		return log.ErrorLevel
	case zapcore.DPanicLevel:
		return log.PanicLevel
	case zapcore.PanicLevel:
		return log.PanicLevel
	case zapcore.FatalLevel:
		return log.FatalLevel
	default:
		return log.InfoLevel
	}
}

func main() {
	// Create a Zap Core that forwards to Go-Spring.
	core := NewZapGoSpringWriter(log.GetLogger("root"))

	// Create a Zap Logger based on the custom Core.
	zapLogger := zap.New(core, zap.AddCaller())
	defer zapLogger.Sync()

	// Initialize Go-Spring logging configuration.
	err := log.RefreshConfig(map[string]string{
		"logger.root.type":                     "FileLogger",
		"logger.root.level":                    "INFO",
		"logger.root.dir":                      "./logs",
		"logger.root.file":                     "app.log",
		"logger.root.layout.type":              "JSONLayout",
		"logger.root.layout.fileLineMaxLength": "20",
	})
	if err != nil {
		stdlog.Printf("log refresh config error: %v", err)
		return
	}

	// Output logs using Zap; they are ultimately written to the Go-Spring logging system.
	zapLogger.Info("zap info message",
		zap.String("user", "alice"),
		zap.Int64("order_id", 10001),
	)

	zapLogger.Warn("zap warn message",
		zap.String("action", "retry"),
		zap.Int("attempt", 3),
	)

	zapLogger.Error("zap error message",
		zap.Error(os.ErrNotExist),
	)

	// Output logs using the native Go-Spring logging API, verifying that the two can coexist.
	log.Info(context.Background(), log.TagAppDef, log.String("user", "alice"))
}
```

After executing the example above, Zap-generated logs are forwarded through the adapter to the Go-Spring logging system and finally written to the `./logs/app.log` file.
The output contains two formats of logs:

```text
{"level":"info","ts":1777816204.077995,"caller":"myapp/main.go:116","msg":"zap info message","user":"alice","order_id":10001}
{"level":"warn","ts":1777816204.078231,"caller":"myapp/main.go:121","msg":"zap warn message","action":"retry","attempt":3}
{"level":"error","ts":1777816204.078253,"caller":"myapp/main.go:126","msg":"zap error message","error":"file does not exist"}
{"level":"info","time":"2026-05-03T21:50:04.078","fileLine":"...myapp/main.go:131","tag":"_app_def","user":"alice"}
```

As you can see, the first three entries are logs generated by Zap (using Zap's own JSON format), and the last entry is a native Go-Spring log.
Both types of logs ultimately converge to the same output target, achieving unified logging system management.
