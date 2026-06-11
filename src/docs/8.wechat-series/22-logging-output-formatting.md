# Go-Spring 实战第 22 课 —— 日志输出与格式化：Appender、Layout 与 Encoder

上一篇咱们介绍了日志的路由与调度。Tag 会根据精确规则、层级通配和 Root Logger 找到对应的 Logger；Logger 完成级别过滤以后，再根据具体类型同步、异步或者直接处理 Event。组合式 Logger 通过 `AppenderRef` 分发 Event，集成式 Logger 则在内部使用相应的 Appender。

Event 进入 Appender 以后，日志处理还需要回答三个问题：写到哪里、整条日志采用什么格式、每个 Field 怎样转换成字节。为了解决这三个问题，Go-Spring 分别使用 Appender、Layout 和 Encoder 承担对应的职责；它们共同组成了日志的输出阶段。

## Appender

我们可以把 Appender 理解为日志输出目标的抽象。它负责接收 Event、管理输出资源并完成实际写入，但不负责选择 Logger，也不决定 Event 是否能够通过级别过滤。

Appender 可能会持有文件句柄、连接或者其他资源，因此需要通过 `Start` 和 `Stop` 参与日志系统的生命周期。与此同时，`ConcurrentSafe` 会声明它能否被多个 goroutine 并发调用；日志系统会据此检查组合式 Logger 与 Appender 的连接是否合法。

Go-Spring 内置了四种 Appender：`DiscardAppender`、`ConsoleAppender`、`FileAppender`、`RollingFileAppender`。

### DiscardAppender

`DiscardAppender` 接收 Event，但不产生实际输出。

```properties
logging.appender.discard.type = DiscardAppender
```

它通常用于组合式 Logger 的某个输出分支。虽然它与上一课介绍的 `DiscardLogger` 都会丢弃日志，但两者所在的层次不同：`DiscardLogger` 在 Logger 层直接结束处理；`DiscardAppender` 则仍然是一个可以被 `AppenderRef` 引用的输出目标。

### ConsoleAppender

`ConsoleAppender` 将日志写入标准输出。本地开发时可以直接查看输出；在容器环境中，日志采集系统也可以统一读取标准输出。

```properties
logging.appender.console.type = ConsoleAppender
logging.appender.console.layout.type = TextLayout
```

它不持有需要关闭的文件资源，同时也支持并发调用。至于最终输出普通文本还是 JSON，则由内部配置的 Layout 决定。

### FileAppender

`FileAppender` 持续向指定文件追加日志，适合日志量较小、文件生命周期由外部系统管理，或者只需要固定文件输出的场景。

```properties
logging.appender.file.type = FileAppender
logging.appender.file.dir = ./logs
logging.appender.file.file = app.log
logging.appender.file.layout.type = JSONLayout
```

`FileAppender` 不负责自动切换文件，也不负责清理历史文件；它会在 `Start` 阶段打开文件，在 `Stop` 阶段关闭文件。

需要注意的是，`FileAppender` 不会自动创建 `dir` 对应的目录。因此，如果目录不存在或者没有写入权限，Appender 就会在日志系统启动阶段返回错误。

### RollingFileAppender

对于长期运行的服务，单个日志文件会不断增长，通常需要按时间滚动文件，并定期清理历史内容。这类场景可以使用 `RollingFileAppender`。

```properties
logging.appender.rolling.type = RollingFileAppender
logging.appender.rolling.dir = ./logs
logging.appender.rolling.file = app.log
logging.appender.rolling.interval = 1h
logging.appender.rolling.maxAge = 168h
logging.appender.rolling.syncLock = false
logging.appender.rolling.layout.type = JSONLayout
```

`interval` 表示文件滚动周期，`maxAge` 表示历史文件的保留时间。不过，历史文件清理依赖后续滚动触发，并不是到达 `maxAge` 后立即删除；应用启动时也不会单独扫描和清理旧文件。

`RollingFileAppender` 内部的 `RollingFileWriter` 不是并发安全的。如果它连接在 `SyncLogger` 后面，多个业务 goroutine 可能会同时调用 Appender，此时需要设置 `syncLock=true`。反过来，如果它连接在 `AsyncLogger` 后面，Event 会由后台 goroutine 顺序写入，通常就可以保持 `syncLock=false`。

正因为 `SyncLogger` 可能会并发调用 Appender，所以日志系统在创建它时，会检查所引用的 Appender 是否支持并发调用。如果 Appender 不是并发安全的，日志配置就会在初始化阶段失败，而不会把并发问题留到运行期。

## Layout

Appender 决定日志写到哪里，Layout 则决定一条完整的日志长什么样。它会接收 Event，然后把格式化结果写入 `Writer`。

```go
type Layout interface {
	EncodeTo(e *log.Event, w log.Writer)
}
```

一个 Event 中不仅包含业务 Field，还包含 Level、Time、File、Line、Tag 和上下文信息。至于保留哪些内容、按照什么顺序组织，以及采用什么整体结构，则都由 Layout 决定。

Go-Spring 内置了 `TextLayout` 和 `JSONLayout`。两者处理的是同一个 Event，只是最终的表现形式不同。

它们都会处理下面这些公共信息：

- `level`：日志级别。
- `time`：日志时间。
- `fileLine`：调用文件和行号。
- `tag`：日志标签。
- 上下文字符串和上下文字段。
- 调用点传入的业务 Field。

两种 Layout 都可以通过 `fileLineMaxLength` 限制文件路径的展示长度。如果路径超过了限制，Layout 会截断前面的部分，只保留更接近文件名和行号的内容。

### TextLayout

`TextLayout` 生成便于人阅读的单行文本：

```text
[INFO][2026-06-05T10:30:00.000][order.go:42] _biz_order_create||order_id=10001||msg=订单创建完成
```

它会先写入级别、时间、调用位置和 Tag，再使用 `||` 分隔上下文信息与业务 Field。顶层 Field 使用 `key=value` 形式表达；数组和嵌套对象则仍然使用 JSON 结构。

```properties
logging.appender.console.layout.type = TextLayout
logging.appender.console.layout.fileLineMaxLength = 48
```

这种格式既保留了结构化字段，整体上又是一行面向人的普通文本，因此更适合本地开发和直接阅读。

### JSONLayout

`JSONLayout` 把完整 Event 组织成单行 JSON：

```json
{"level":"info","time":"2026-06-05T10:30:00.000","fileLine":"order.go:42","tag":"_biz_order_create","order_id":10001,"msg":"订单创建完成"}
```

```properties
logging.appender.file.layout.type = JSONLayout
logging.appender.file.layout.fileLineMaxLength = 48
```

级别、时间、调用位置、Tag、上下文信息和业务 Field 都会成为 JSON 字段；数字、布尔值、数组和对象也会保留各自的类型。因此，这种格式更适合日志采集、检索和聚合。

## Encoder

Layout 负责整条日志的结构，但不会分别实现每一种 Field 类型的编码逻辑。因此，具体字段怎样转换成字节，还需要由 Encoder 负责。

与两种 Layout 对应，Go-Spring 也内置了 `JSONEncoder` 和 `TextEncoder`。

`JSONEncoder` 供 `JSONLayout` 使用，按照 JSON 语法编码字段：

```json
{"success":true,"retry":2,"tags":["new_user","coupon"]}
```

`TextEncoder` 则供 `TextLayout` 使用。顶层字段采用 `key=value` 形式，数组和嵌套对象则交给内部的 JSON 编码逻辑：

```text
success=true||retry=2||tags=["new_user","coupon"]
```

无论使用哪一种 Encoder，强类型 Field 都可以直接进入对应的编码方法，不需要先放进 `map[string]any`，也不需要在输出阶段重新判断类型。只有使用 `Any` 并且无法识别具体类型时，才会回退到反射编码。

Encoder 最终会把结果写向 `Writer`。Go-Spring 的 `Writer` 支持普通字节、单字节和字符串写入，因此 Layout 和 Encoder 可以边组织、边编码，不需要先构造完整的中间对象。这样一来，Layout 可以专注于整条日志的结构；Encoder 则只需要关心单个 Field 的类型和值。

## 日志输出与格式化

Layout 组织日志，Encoder 编码字段，Appender 经缓冲区写入目标。失败由 ReportError 上报，三者职责独立。
