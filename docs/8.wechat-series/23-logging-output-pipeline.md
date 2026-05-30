# Go-Spring 实战第 23 课 —— 输出管线：Appender、Layout 与 Encoder 的职责边界

Logger 选好以后，一条日志仍然没有完成落地。Logger 只是决定事件是否通过级别过滤、交给哪些输出目标；真正写出字节之前，还要继续回答写到哪里、输出成什么格式、字段如何编码。

如果这些问题都由一个 Logger 实现承担，后续扩展会很难控制。比如同一套业务日志既要写本地滚动文件，又要换成 JSON 格式，还要优化字段编码路径，任何一个变化都可能牵动整个 Logger。

Go-Spring 在 Logger 之后继续拆成 Appender、Layout 和 Encoder 三层。Appender 负责输出目标，Layout 负责最终格式，Encoder 负责字段编码。这样输出目标、格式和编码效率可以分别演进。

## Appender 决定日志事件写到哪个目标

Appender 是日志落地执行单元。一个 Logger 可以绑定多个 Appender，于是同一条日志可以同时进入控制台、本地文件、滚动文件或自定义远端系统。

Go-Spring 内置四类 Appender。

| Appender | 输出目标 |
|----------|----------|
| `DiscardAppender` | 丢弃所有日志 |
| `ConsoleAppender` | 标准输出 |
| `FileAppender` | 单个本地文件 |
| `RollingFileAppender` | 按时间滚动的文件序列 |

## DiscardAppender 用显式配置表达丢弃语义

有些环境需要保留路由结构，但不希望某类日志真正落地。Go-Spring 可以用显式 Appender 表达丢弃语义。

```properties
appender.discard.type = DiscardAppender
```

`DiscardAppender` 会静默丢弃所有日志事件，不产生实际输出。它适合临时关闭某类日志、测试路由规则，或者为特定环境保留配置结构。显式丢弃比隐式没有输出目标更容易排查，因为路由仍然是完整的。

## ConsoleAppender 写向标准输出

容器环境通常会采集 stdout，本地开发也经常直接看控制台。下面的配置只决定输出目标，最终格式仍然交给 Layout。

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout
```

这段配置的语义是把事件写到标准输出，并使用 `TextLayout` 编码。`ConsoleAppender` 适合本地开发、启动排障和容器日志采集。生产高并发场景下，大量写 stdout 可能成为瓶颈，因此控制台输出通常不承担全部业务日志。

## FileAppender 写向不滚动的单个文件

如果日志量不大，也不需要 Go-Spring 自动滚动和清理，可以把输出固定到一个文件。下面的配置只声明单文件写入。

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

它的语义是持续追加到 `./logs/app.log`，文件生命周期不由 `FileAppender` 自动管理。它适合低流量服务、测试日志、短生命周期任务和审计归档；如果服务长期运行且日志持续增长，就要考虑滚动文件。

## RollingFileAppender 同时管理滚动和清理

长期运行服务更常见的是滚动文件。下面的配置把文件滚动和过期清理交给 `RollingFileAppender`，而 Layout 仍然独立配置。

```properties
appender.rolling.type = RollingFileAppender
appender.rolling.dir = ./logs
appender.rolling.file = app.log
appender.rolling.interval = 1h
appender.rolling.maxAge = 168h
appender.rolling.syncLock = false
appender.rolling.layout.type = JSONLayout
```

这段配置的语义是按 `interval` 切换文件，并按 `maxAge` 清理过期日志。`syncLock` 要跟上游 Logger 的写入方式一起看。如果同步 Logger 会被多个 goroutine 并发写入，可以开启 `syncLock=true`；如果配合 AsyncLogger，通常保持 `false`，由异步单 goroutine 保证串行写入。

## 自定义 Appender 只接入新的输出策略

当日志要写入 Kafka、HTTP 接口、远程日志服务，或者需要在落地前做采样、过滤时，可以实现自定义 Appender。下面的自定义 Appender 复用 `FileAppender` 的文件能力，只在 `Append` 中增加采样策略。

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

这个例子的语义是：输出目标仍然是文件，但进入文件之前多了一层采样。自定义 Appender 的重点是输出策略。生命周期、并发安全和基础错误处理如果能复用内置实现，就不要在差异逻辑里重新实现一遍。

## Layout 决定日志给人读还是给机器解析

Appender 只关心写到哪里，不关心事件长什么样。日志事件变成最终字节流之前，会先交给 Layout。

`TextLayout` 面向人类阅读，适合控制台、本地开发和临时排障。下面这行文本说明它会把级别、时间、文件位置、标签、上下文和字段组织成一行。

```text
[级别][时间][文件:行号] 标签||上下文字符串||key=value||msg=日志消息
```

如果使用文本格式，可以通过下面的配置控制文件行号展示长度，避免日志头部过长。

```properties
appender.console.layout.type = TextLayout
appender.console.layout.fileLineMaxLength = 48
```

`JSONLayout` 面向机器解析，通常用在日志采集系统里。同一个 Appender 只要替换 Layout，就可以改变输出格式。

```properties
appender.file.layout.type = JSONLayout
appender.file.layout.fileLineMaxLength = 48
```

这两组配置的语义是：输出目标没有变，事件变成字节的方式发生了变化。生产环境通常优先 JSON，原因是日志进入采集系统后，字段索引、过滤和聚合都依赖稳定的机器可解析格式。

## 自定义 Layout 处理特殊输出格式

如果系统需要 CSV、Protobuf 或自定义分隔格式，可以实现 Layout 并注册插件。自定义 Layout 只需要把 `Event` 编码到目标 Writer。

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

注册后，配置里把 Layout 类型改成插件名即可接入。

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = CSVLayout
```

这个扩展点的语义很窄：它只改变事件的字节表达，不改变 Logger 的路由和 Appender 的输出目标。因此特殊格式应该放在 Layout，而不是重新实现 Logger 或 Appender。

## Encoder 让字段编码少做无效工作

Encoder 是字段编码层。它的目标是让基础类型走专门编码路径，让字段携带的类型信息直接参与编码，并尽量直接写入目标 Writer，减少中间对象。

业务代码通常不直接操作 Encoder。只有实现自定义 Layout 时，才需要组合 Encoder。这个边界也说明了 Go-Spring 字段模型的意义，字段在调用点已经带上类型，输出时就不必再把所有内容当成未知对象处理。

## 输出管线

Appender、Layout、Encoder 分离后，输出目标、输出格式和编码实现可以独立扩展。写到哪里、长什么样、怎么编码，不需要绑死在一个实现里。

Go-Spring 的输出管线把运行期差异拆成三个稳定问题：Appender 决定写到哪里，Layout 决定长什么样，Encoder 决定字段如何变成字节。这个边界清楚以后，新增远端输出、切换 JSON、优化字段编码，都可以落在对应层次里，而不会牵动整条日志调用链。
