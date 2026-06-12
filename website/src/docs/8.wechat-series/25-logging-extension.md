# Go-Spring 实战第 25 课 —— 日志扩展：自定义组件与插件边界

上一篇我们介绍了怎样用配置组织 Logger、Appender 和 Layout，并在刷新时替换整条日志链路。内置组件已经覆盖同步、异步、控制台、普通文件和滚动文件等常见场景，但项目仍然可能需要新的输出目标、内部日志格式，或者特殊的调度策略。

这些变化不需要重新建立一套日志系统。Go-Spring 已经把一条日志拆成 Logger、Appender、Layout 和 Encoder 几层，自定义能力也应该沿着这些边界进入原有管线。

## 选择扩展位置

扩展日志系统之前，先要判断变化发生在哪一层。

| 需求 | 扩展位置 |
| --- | --- |
| 改变过滤、采样、同步或异步调度 | Logger |
| 写入新的目标系统 | Appender |
| 改变一条完整日志的输出格式 | Layout |
| 改变 Field 的底层编码方式 | 在 Layout 中组合或实现 Encoder |

例如，把日志写入 Kafka、内部采集服务或审计平台，变化发生在输出目标，应当扩展 Appender。如果只是要求一行日志使用 CSV 或内部文本格式，则扩展 Layout 即可。

扩展点越靠近真正变化的位置，需要接管的职责越少。为了改变输出格式而重写 Logger，会同时把级别过滤、Event 所有权、并发调度和关闭流程都带进来。

## 自定义 Layout

Layout 接收完整 Event，并把它编码到 Writer。下面的 CompactLayout 输出级别、Tag，以及一个包含上下文和业务字段的 JSON 对象。

```go
type CompactLayout struct {
	log.BaseLayout
}

func (l *CompactLayout) EncodeTo(e *log.Event, w log.Writer) {
	_, _ = w.WriteString(e.Level.UpperName())
	_ = w.WriteByte(' ')
	_, _ = w.WriteString(e.Tag)
	_ = w.WriteByte(' ')

	enc := log.NewJSONEncoder(w)
	enc.AppendEncoderBegin()
	if e.CtxString != "" {
		log.String("ctxString", e.CtxString).Encode(enc)
	}
	log.EncodeFields(enc, e.CtxFields)
	log.EncodeFields(enc, e.Fields)
	enc.AppendEncoderEnd()
	_ = w.WriteByte('\n')
}

func init() {
	log.RegisterPlugin[CompactLayout]("CompactLayout")
}
```

注册以后，自定义 Layout 和内置 Layout 使用同一套配置方式。

```properties
logging.appender.console.type = ConsoleAppender
logging.appender.console.layout.type = CompactLayout
```

这个扩展只改变 Event 的字节表示。Appender 仍然负责标准输出，Logger 的路由、过滤和调度也没有变化。

Encoder 位于 Layout 内部，负责把 Field 写成具体格式。它不是独立的 `encoder.*.type` 配置节点。需要新的字段编码协议时，通常由自定义 Layout 在 `EncodeTo` 中创建或组合对应 Encoder。

## 自定义 Appender

Appender 面向输出目标。它实现的接口包含生命周期、名称、写入方法和并发能力声明。

```go
type Appender interface {
	log.Lifecycle
	GetName() string
	Append(e *log.Event)
	ConcurrentSafe() bool
}
```

资源型 Appender 通常在 `Start` 中创建客户端、连接或后台任务，在 `Append` 中完成编码和写出，在 `Stop` 中刷新缓冲并释放资源。

自定义 Appender 完成以后，需要注册为日志插件。

```go
func init() {
	log.RegisterPlugin[RemoteAppender]("RemoteAppender")
}
```

配置中的 `type` 使用注册名称，而不是 Go 类型名。

```properties
logging.appender.remote.type = RemoteAppender
logging.appender.remote.endpoint = collector.example.com
logging.appender.remote.layout.type = JSONLayout
```

Appender 的 `Append` 不能修改 Event，也不能在返回以后继续持有 Event 引用。Event 来自对象池，上游 Logger 会在处理结束后重置并复用它。如果 Appender 要把工作交给自己的后台 goroutine，必须先复制真正需要的数据。

如果输出客户端本身已经提供异步队列，还要重新判断是否需要 AsyncLogger。重复增加缓冲层会让背压、丢弃、错误上报和关闭顺序更难控制。

## 自定义 Logger

只有变化涉及 Event 调度时，才需要自定义 Logger。例如按比例采样、按内容分流，或者实现项目特有的批处理策略。

Logger 负责的边界比 Appender 更大。

```go
type Logger interface {
	log.Lifecycle
	GetName() string
	GetTags() []string
	GetLevel() log.LevelRange
	Append(e *log.Event)
}
```

自定义 Logger 接收 Event 后，必须保证它最终被处理并调用 `Reset`，或者交给另一个明确负责 Event 所有权的 Logger。即使某条日志因为采样被丢弃，也不能直接返回而不释放 Event。

大多数输出扩展并不需要重写 Logger。优先组合 SyncLogger、AsyncLogger 和 Appender，可以继续复用已有的级别过滤、缓冲区满策略和关闭语义。

并发约束也由 Logger 的调度方式决定。SyncLogger 会在调用方 goroutine 中直接调用 Appender，因此 Appender 必须声明并实现并发安全；AsyncLogger 由单个后台 goroutine 消费 Event，可以使用非并发安全的 Appender。日志刷新会检查这组组合是否合法。

## 插件怎样从配置创建

自定义 Logger、Appender 或 Layout 完成以后，还需要进入日志系统的插件注册表，配置中的 `type` 才能找到它。

```go
func init() {
	log.RegisterPlugin[RemoteAppender]("RemoteAppender")
}
```

`RegisterPlugin` 的泛型参数必须是结构体，传入的字符串则是配置使用的注册名：

```properties
logging.appender.remote.type = RemoteAppender
```

日志刷新时，Go-Spring 会先读取 `type`，根据注册名创建对应的结构体指针，再把当前配置前缀下的属性注入实例。注册名不要求与 Go 类型名相同，但同一个名称不能重复注册。插件和转换器都应当在第一次日志刷新之前完成注册。

### 普通属性

普通配置使用 `PluginAttribute` 注入。结构体 Tag 的第一个值是属性名，`default` 用于声明配置缺失时的默认值。

```go
type RemoteAppender struct {
	log.AppenderBase

	Endpoint string        `PluginAttribute:"endpoint"`
	Timeout  time.Duration `PluginAttribute:"timeout,default=3s"`
}
```

对应配置如下：

```properties
logging.appender.remote.type = RemoteAppender
logging.appender.remote.endpoint = collector.example.com
logging.appender.remote.timeout = 5s
```

`endpoint` 没有默认值，缺少配置会让插件创建失败；`timeout` 没有配置时则使用 `3s`。日志插件内置支持字符串、数值、布尔值和 `time.Duration` 等常用类型，属性值会先解析 `${key}` 引用，再转换成字段类型。

切片属性既可以使用逗号分隔，也可以使用连续数组下标。复杂对象数组则需要使用下标展开配置。插件作者只需要声明字段的配置语义，不需要在 `Start` 中再次读取原始 Properties。

### 嵌套插件

Logger、Appender 和 Layout 之间还存在嵌套关系，这类配置使用 `PluginElement` 注入。`AppenderBase` 已经声明了 Layout：

```go
type AppenderBase struct {
	Name   string `PluginAttribute:"name"`
	Layout Layout `PluginElement:"layout,default=TextLayout"`
}
```

因此，RemoteAppender 嵌入 `AppenderBase` 后，可以直接使用下面的子配置：

```properties
logging.appender.remote.type = RemoteAppender
logging.appender.remote.endpoint = collector.example.com
logging.appender.remote.layout.type = JSONLayout
logging.appender.remote.layout.fileLineMaxLength = 48
```

插件系统会先创建 `RemoteAppender`，再根据 `layout.type` 创建 `JSONLayout`，并继续注入 `fileLineMaxLength`。如果没有配置 `layout.type`，这里会使用 `PluginElement` 声明的默认类型 `TextLayout`。

`PluginElement` 只负责创建和注入嵌套对象。日志系统会自动管理顶层 Logger 和 Appender 的生命周期，但不会继续遍历 Layout 等子插件调用 `Start` 和 `Stop`。如果自定义子插件持有连接、缓冲区或后台任务，应当由所属的 Logger 或 Appender 显式启动和停止。

### 自定义类型转换

插件字段如果不是内置支持的配置类型，可以通过 `RegisterConverter` 注册字符串转换函数。例如，另一个远程输出组件使用自定义的重试策略：

```go
type RetryPolicy int

const (
	RetryDiscard RetryPolicy = iota
	RetryBlock
)

func parseRetryPolicy(s string) (RetryPolicy, error) {
	switch s {
	case "discard":
		return RetryDiscard, nil
	case "block":
		return RetryBlock, nil
	default:
		return 0, fmt.Errorf("invalid retry policy %q", s)
	}
}

type RetryingAppender struct {
	log.AppenderBase

	RetryPolicy RetryPolicy `PluginAttribute:"retryPolicy,default=discard"`
}

func init() {
	log.RegisterConverter(parseRetryPolicy)
	log.RegisterPlugin[RetryingAppender]("RetryingAppender")
}
```

配置中的 `retryPolicy=block` 会先经过 `parseRetryPolicy`，再注入 `RetryPolicy` 字段。转换函数接收配置字符串，返回目标类型或 error。转换失败会终止本次插件创建，因此转换器应当保持确定、无副作用，不要在其中建立连接或启动后台任务。

## 刷新与生命周期

配置刷新不是在旧对象上修改字段，而是建立一套新的日志链路。Go-Spring 会先创建并注入新插件，解析 Logger 对 Appender 的引用，再依次启动 Appender 和 Logger。全部成功后，系统替换 Tag 和命名 Logger 的引用，最后先停止旧 Logger，再停止旧 Appender。

这个顺序给自定义组件提出了明确要求：

- `Start` 只负责让已经完成配置注入的实例进入可用状态。
- `Start` 返回 error 时，组件必须自行释放本次调用中已经创建、但尚未交给框架管理的资源。
- `Stop` 应当允许组件完成缓冲刷写、后台任务退出和连接关闭。
- Logger 必须先停止事件生产或调度，Appender 才能安全释放输出资源。

如果后续组件启动失败，日志系统会停止本次刷新中已经成功启动的临时 Logger 和 Appender，当前正在使用的旧链路不会被替换。但对于正在返回 error 的那个 `Start`，框架无法判断它已经初始化到哪一步，所以仍需组件自己回滚局部资源。

Go-Spring App 正常关闭时，会先停止 Server、等待运行任务结束并关闭 IoC 容器，最后调用 `log.Destroy()`。这样关闭过程本身仍然可以记录日志。`Destroy` 同样先停止 Logger，再停止 Appender，因此自定义组件不能依赖进程直接退出完成清理。

## 输出错误怎样上报

插件创建、属性转换、引用解析和 `Start` 失败都发生在初始化或刷新阶段，可以直接通过返回的 error 终止本次刷新。运行期输出错误则不同：如果 RemoteAppender 写入失败后再次调用当前日志系统记录错误，日志仍会进入 RemoteAppender，可能形成递归。

自定义 Appender 应沿用 `log.ReportError` 把输出错误交给日志系统之外的通道：

```go
log.ReportError = func(err error) {
	metrics.Incr("log_write_error_total")
}
```

这个回调可能出现在日志输出路径上，应该保持简单、非阻塞，并且不能再次调用当前日志系统。指标、独立告警通道或专用错误收集器都可以承接这类错误。

日志扩展的关键不是实现更多类型，而是守住现有组件边界：调度放在 Logger，输出目标放在 Appender，整条日志格式放在 Layout，字段编码放在 Encoder。插件注册、配置注入、Event 所有权和生命周期都沿用同一套规则，自定义能力才能继续参与刷新和统一治理。
