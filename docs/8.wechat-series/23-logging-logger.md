# Go-Spring 实战第 23 课：Logger 体系：同步、异步、控制台、文件和滚动文件怎么选

业务代码在 Go-Spring 日志 API 中产生了结构化字段以后，日志事件还没有真正落地。接下来要回答的问题是：谁来决定它要不要输出、输出到哪里、用什么方式输出？

在 Go-Spring 的日志系统里，这个角色就是 Logger。标签路由找到 Logger 后，Logger 会负责级别过滤，再把事件分发给一个或多个输出目标。

为了照顾不同写入场景，Go-Spring 的 Logger 分为两类：

- 组合式 Logger：`SyncLogger`、`AsyncLogger`，通过 `appenderRef` 组合输出目标。
- 集成式 Logger：`ConsoleLogger`、`FileLogger`、`RollingFileLogger`，封装常见输出场景。

先简单理解就好：组合式 Logger 是更灵活的管线，集成式 Logger 是常见场景的快捷封装。

选型时先问两个问题就够了：这条日志能不能阻塞业务 goroutine，以及它最终要进 stdout、普通文件还是滚动文件。答案确定以后，再考虑是否需要自定义 Logger。

## SyncLogger 适合强确定性写入

`SyncLogger` 在业务 goroutine 里同步完成写入。级别过滤、字段编码和 Appender 写入都在同一调用栈内执行。

下面这组配置让 `_app_*` 标签走同步 Logger，并同时写入控制台和文件。重点看 `appenderRef`，它决定同步 Logger 会把同一条事件分发到哪些目标。

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

同步写入确定性强，适合启动日志、审计日志、开发调试等场景。但如果高并发业务日志直接同步写文件，就可能阻塞请求路径。

## AsyncLogger 适合高并发日志路径

`AsyncLogger` 将日志产生和实际写入解耦。业务 goroutine 把事件放入缓冲区后返回，后台 goroutine 负责编码和写入。

下面这组配置让 `_biz_*` 标签走异步 Logger。重点看 `bufferSize` 和 `onBufferFull`，它们决定高峰期日志是在业务 goroutine 中等待，还是被丢弃或替换。

```properties
logger.async.type = AsyncLogger
logger.async.tag = _biz_*
logger.async.level = INFO
logger.async.bufferSize = 50000
logger.async.onBufferFull = block
logger.async.appenderRef[0].ref = file
```

缓冲区满策略包括：

| 策略 | 行为 |
|------|------|
| `block` | 阻塞业务 goroutine，等待缓冲区空位 |
| `discard` | 丢弃新日志 |
| `drop-oldest` | 丢弃最旧日志，保留最新现场 |

生产高并发场景通常会优先选异步写入，但也要接受进程被强杀时缓冲区日志可能丢失的事实。所以我们需要根据日志价值选择缓冲区策略：审计日志更偏向阻塞，调试日志可以接受丢弃。

## ConsoleLogger 适合 stdout 场景

`ConsoleLogger` 是面向标准输出的集成式 Logger：

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = TextLayout
```

它适合本地开发、调试和容器 stdout 输出。如果是高并发生产环境，不建议把大量业务日志打到控制台，标准输出可能成为性能瓶颈。

## FileLogger 适合单文件低流量写入

`FileLogger` 写入单个本地文件：

```properties
logger.file.type = FileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
```

它适合低流量服务、测试环境、短生命周期任务或定向调试。如果长期运行服务日志量较大，应使用滚动文件。

## RollingFileLogger 适合生产长期运行

`RollingFileLogger` 面向生产环境，支持按时间滚动、过期清理、级别分离和内置异步：

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

常见建议：

- 高流量服务用较短滚动间隔，例如 `1h`。
- 保留时间根据磁盘容量和合规要求设置。
- `separate=true` 可以把 `WARN` 及以上日志单独输出，便于排障。
- `async=true` 时不需要再额外套一层 `AsyncLogger`。

## 自定义 Logger 只扩展差异点

可以通过组合内置 Logger 扩展差异逻辑。例如采样 Logger：

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

自定义 Logger 尽量复用已有同步、异步和生命周期能力，只在差异点上扩展。这样扩展点更小，也更容易维护。

## Logger 只负责调度事件

Logger 只管判断级别、调度事件和组合输出目标。同步、异步、控制台、文件、滚动文件这些差异，最终都服务于同一个目标：让日志事件按规则进入正确的输出路径。

Logger 选好以后，真正写出日志前还要经过 Appender、Layout 和 Encoder，这三层分别决定输出目标、输出格式和字段编码方式。
