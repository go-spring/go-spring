# Go-Spring 实战第 23 课 —— Logger 体系：同步、异步与滚动文件的取舍

业务代码把日志事件和字段交给 Go-Spring 以后，事件还没有真正落地。接下来必须先决定三个问题，即这条日志能不能被级别过滤掉，写入是否允许阻塞业务 goroutine，最终输出到控制台、普通文件还是滚动文件。

在 Go-Spring 的日志系统里，回答这些问题的对象是 Logger。标签路由找到 Logger 后，Logger 先做级别判断，再把事件分发给一个或多个输出目标。

Go-Spring 的 Logger 分成两类。组合式 Logger 包括 `SyncLogger`、`AsyncLogger`，它们通过 `appenderRef` 组合输出目标；集成式 Logger 包括 `ConsoleLogger`、`FileLogger`、`RollingFileLogger`，它们封装常见输出场景。

也就是说，组合式 Logger 更适合需要自己拼管线的场景，集成式 Logger 更适合用较少配置覆盖常见需求。选型时先看日志是否能阻塞业务路径，再看输出目标和滚动策略。

## SyncLogger 适合必须确定写入的日志

`SyncLogger` 在业务 goroutine 里同步完成写入。级别过滤、字段编码和 Appender 写入都在同一调用栈内执行，因此它的确定性更强。

启动日志、审计日志、开发调试这类日志通常更关心“写入是否已经完成”。下面这组配置证明 `SyncLogger` 可以把同一类标签同步分发到多个 Appender，重点看 `appenderRef`。

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

这段配置的语义是：`_app_*` 标签命中的事件先经过 `INFO` 级别过滤，然后依次写入 `console` 和 `file`。代价也很明确，写入发生在业务 goroutine 中。如果高并发业务日志直接同步写文件，请求路径就可能被文件 IO 或锁竞争拖慢。因此 `SyncLogger` 更适合低频但价值高的日志。

## AsyncLogger 适合高并发业务日志路径

`AsyncLogger` 将日志产生和实际写入解耦。业务 goroutine 把事件放入缓冲区后返回，后台 goroutine 负责编码和写入。

高并发业务日志通常更关注请求路径延迟。下面这组配置证明 `AsyncLogger` 会把 `_biz_*` 标签命中的事件先放入缓冲区，再由后台 goroutine 写出。重点看 `bufferSize` 和 `onBufferFull`。

```properties
logger.async.type = AsyncLogger
logger.async.tag = _biz_*
logger.async.level = INFO
logger.async.bufferSize = 50000
logger.async.onBufferFull = block
logger.async.appenderRef[0].ref = file
```

缓冲区满策略对应不同取舍。

| 策略 | 行为 |
|------|------|
| `block` | 阻塞业务 goroutine，等待缓冲区空位 |
| `discard` | 丢弃新日志 |
| `drop-oldest` | 丢弃最旧日志，保留最新现场 |

这段配置的语义是：业务 goroutine 只负责把事件交给缓冲区，缓冲区满时按照 `onBufferFull` 决定等待、丢弃新事件，或者丢弃最旧事件。生产高并发场景通常会优先选异步写入，但异步也有边界。进程被强杀时，缓冲区里的日志可能来不及写出；缓冲区满时，不同策略也会在延迟和完整性之间做选择。因此审计日志更偏向 `block`，调试日志可以接受丢弃。

## ConsoleLogger 面向标准输出和本地排障

`ConsoleLogger` 是面向标准输出的集成式 Logger。下面的配置证明它可以直接声明输出格式，不需要额外声明 Appender。

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = TextLayout
```

它的语义是把 Logger 和控制台 Appender 封装在一起。本地开发、启动阶段排障和容器 stdout 采集都适合使用控制台输出。但在高并发生产环境里，大量业务日志写 stdout 可能成为性能瓶颈，也可能把应用日志和运行环境采集策略绑得过紧。

## FileLogger 适合低流量单文件写入

`FileLogger` 写入单个本地文件。下面的配置证明它适合把一类标签直接写入固定文件。

```properties
logger.file.type = FileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
```

这段配置的语义是持续追加到 `./logs/app.log`，不负责自动滚动和过期清理。它适合日志量不大、文件生命周期由外部系统管理，或者只需要一次性定向调试的场景。如果长期运行服务日志量较大，文件切割和保留策略就应该交给滚动文件能力。

## RollingFileLogger 更适合长期运行的生产服务

`RollingFileLogger` 面向生产长期运行场景。下面的配置证明它可以同时表达按时间滚动、过期清理、级别分离和内置异步。

```properties
logger.file.type = RollingFileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
logger.file.interval = 24h
logger.file.maxAge = 168h
logger.file.separate = true
logger.file.async = true
logger.file.bufferSize = 50000
```

这些配置背后的语义可以分开看。高流量服务通常用较短滚动间隔，例如 `1h`；保留时间由磁盘容量和合规要求决定；`separate=true` 可以把 `WARN` 及以上日志单独输出，便于排障；`async=true` 时，`RollingFileLogger` 已经内置异步写入，不需要再额外套一层 `AsyncLogger`。

## 自定义 Logger 只扩展内置能力覆盖不了的差异点

当内置 Logger 不能覆盖某个策略时，可以通过组合内置 Logger 扩展差异逻辑。下面的例子证明自定义 Logger 可以复用 `log.AsyncLogger`，只在 `Append` 里补充采样判断。

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

func (l *SamplingLogger) Append(e *log.Event) {
	if e.Level.Code() >= log.ErrorLevel.Code() {
		l.AsyncLogger.Append(e)
		return
	}
	if l.rand.Float64() < l.SampleRate {
		e.Fields = append(e.Fields, log.Bool("sampled", true))
		l.AsyncLogger.Append(e)
	}
}

func init() {
	log.RegisterPlugin[SamplingLogger]("SamplingLogger")
}
```

这个例子没有重写缓冲区、生命周期和 Appender 分发逻辑，而是复用 `AsyncLogger` 已经提供的能力。自定义 Logger 的边界越小，后续跟随 Go-Spring 日志体系演进的成本越低。

## Logger 体系

Logger 只管级别判断、事件调度和输出目标组合。同步、异步、控制台、文件、滚动文件这些差异，最终都服务于同一个目标，即让日志事件按规则进入正确的输出路径。

选 Logger 时可以先判断日志是否允许阻塞业务 goroutine，再判断输出目标是否需要滚动、分离和内置异步。这个边界确定以后，Logger 就停在事件调度层，不再承担“字节写到哪里”和“字段如何编码”的职责。
