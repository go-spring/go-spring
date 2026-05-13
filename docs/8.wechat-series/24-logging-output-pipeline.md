# 日志输出管线

## 本篇要解决的问题

一条日志从业务代码产生后，不是直接写到文件。它会经过 Logger、Appender、Layout、Encoder 这条管线。

本篇聚焦 Logger 之后的三层：Appender 负责写到哪里，Layout 负责长什么样，Encoder 负责如何高效编码字段。

## Appender

Appender 是日志落地执行单元。一个 Logger 可以绑定多个 Appender，实现一条日志多路输出。

Go-Spring 内置四类 Appender：

| Appender | 输出目标 |
|----------|----------|
| `DiscardAppender` | 丢弃所有日志 |
| `ConsoleAppender` | 标准输出 |
| `FileAppender` | 单个本地文件 |
| `RollingFileAppender` | 按时间滚动的文件序列 |

## DiscardAppender

```properties
appender.discard.type = DiscardAppender
```

`DiscardAppender` 会静默丢弃所有日志事件，不产生实际输出。它适合临时关闭某类日志、测试路由规则，或者为某些环境保留配置结构但不落地日志。

## ConsoleAppender

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout
```

适合本地开发和容器日志采集。生产高并发场景下，应谨慎大量写 stdout。

## FileAppender

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

它会持续追加到单个文件，不自动滚动和清理。适合低流量服务、测试日志、短生命周期任务和审计归档。

## RollingFileAppender

```properties
appender.rolling.type = RollingFileAppender
appender.rolling.dir = ./logs
appender.rolling.file = app.log
appender.rolling.interval = 1h
appender.rolling.maxAge = 168h
appender.rolling.syncLock = false
appender.rolling.layout.type = JSONLayout
```

它支持按时间滚动、过期清理和并发安全配置。同步 Logger 多 goroutine 写入时可以开启 `syncLock=true`；配合 AsyncLogger 时通常保持 `false`，由异步单 goroutine 保证串行写入。

## 自定义 Appender

自定义 Appender 可以把日志写入 Kafka、HTTP 接口、远程日志服务或实现采样、过滤等策略。

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
	if e.Level.Code() >= log.ErrorLevel.Code() {
		a.FileAppender.Append(e)
		return
	}
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

## Layout

Layout 决定日志事件如何变成最终字节流。它不关心日志来自哪里，也不关心写到哪里。

内置 Layout 有两种。

`TextLayout` 面向人类阅读：

```text
[级别][时间][文件:行号] 标签||上下文字符串||key=value||msg=日志消息
```

配置：

```properties
appender.console.layout.type = TextLayout
appender.console.layout.fileLineMaxLength = 48
```

`JSONLayout` 面向机器解析：

```properties
appender.file.layout.type = JSONLayout
appender.file.layout.fileLineMaxLength = 48
```

生产环境通常优先 JSON，方便日志采集、索引和聚合。

## 自定义 Layout

需要 CSV、Protobuf 或自定义分隔格式时，可以实现 Layout 并注册插件：

```go
type CSVLayout struct {
	log.BaseLayout
}

func (l *CSVLayout) EncodeTo(e *log.Event, w log.Writer) {
	w.WriteString(e.Level.UpperName())
	w.WriteByte(',')
	w.WriteString(e.Time.Format("2006-01-02T15:04:05.000"))
	w.WriteByte(',')
	w.WriteString(e.Tag)
	w.WriteByte('\n')
}

func init() {
	log.RegisterPlugin[CSVLayout]("CSVLayout")
}
```

配置中直接使用：

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = CSVLayout
```

## Encoder

Encoder 是字段编码层。它的目标是：

- 基础类型走专门编码路径。
- 字段携带类型信息，编码时无需重新推断。
- 直接写入目标 Writer，减少中间对象。

业务代码通常不直接操作 Encoder。只有实现自定义 Layout 时，才需要组合 Encoder。

## 边界

Appender、Layout、Encoder 分离后，输出目标、输出格式和编码实现可以独立扩展。下一篇会讨论日志如何从 `context.Context` 提取链路和业务字段。
