# Logger 体系

业务代码产生了结构化字段以后，日志事件还没有真正落地。接下来要回答的是：谁决定它要不要输出、输出到哪里、以什么方式输出？

在 Go-Spring 的日志系统里，这个角色就是 Logger。标签路由找到 Logger 后，Logger 负责级别过滤，并把事件分发给一个或多个输出目标。

Go-Spring 的 Logger 分为两类：

- 组合式 Logger：`SyncLogger`、`AsyncLogger`，通过 `appenderRef` 组合输出目标。
- 集成式 Logger：`ConsoleLogger`、`FileLogger`、`RollingFileLogger`，封装常见输出场景。

我们可以把组合式 Logger 理解成更灵活的管线，把集成式 Logger 理解成常见场景的快捷封装。

## SyncLogger

`SyncLogger` 在业务 goroutine 中同步完成写入。级别过滤、字段编码和 Appender 写入都在同一调用栈内执行。

配置示例：

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

同步写入确定性强，适合启动日志、审计日志、开发调试等场景。高并发业务日志如果直接同步写文件，可能阻塞请求路径。

## AsyncLogger

`AsyncLogger` 将日志产生和实际写入解耦。业务 goroutine 把事件放入缓冲区后返回，后台 goroutine 负责编码和写入。

配置示例：

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

生产高并发场景通常优先考虑异步写入，但要接受进程被强杀时缓冲区日志可能丢失的事实。我们需要根据日志价值选择缓冲区策略：审计日志更偏向阻塞，调试日志可以接受丢弃。

## ConsoleLogger

`ConsoleLogger` 是面向标准输出的集成式 Logger：

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = TextLayout
```

它适合本地开发、调试和容器 stdout 输出。高并发生产环境不建议把大量业务日志打到控制台，标准输出可能成为性能瓶颈。

## FileLogger

`FileLogger` 写入单个本地文件：

```properties
logger.file.type = FileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
```

它适合低流量服务、测试环境、短生命周期任务或定向调试。长期运行服务如果日志量较大，应使用滚动文件。

## RollingFileLogger

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

## 自定义 Logger

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

自定义 Logger 应尽量复用已有同步、异步和生命周期能力，只在差异点上扩展。

## Logger 是调度层

Logger 负责判断级别、调度事件和组合输出目标。同步、异步、控制台、文件、滚动文件这些差异，最终都服务于同一个目标：让日志事件按规则进入正确的输出路径。

真正写出日志，还要经过 Appender、Layout 和 Encoder。后面继续展开完整输出管线。
