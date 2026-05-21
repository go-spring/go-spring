# Go-Spring 实战第 26 课 —— 日志治理：配置刷新与生态适配

上一篇讲完上下文提取以后，Go-Spring 的日志事件已经可以带着业务字段和链路字段进入输出管线。但一套日志系统真正落到项目里，还要继续面对工程治理问题。

配置如何组织，Logger、Appender、Layout 如何实例化，写入错误如何上报，配置刷新时新旧实例如何切换，标准库 `log` 和 Zap 这类既有入口如何接入，这些问题决定日志系统能否长期维护。

Go-Spring 的处理方式是把运行期拓扑交给配置描述，把实例创建交给插件系统，把拓扑替换交给刷新入口，把旧日志入口收进适配层。这样日志治理仍然围绕同一条输出管线展开，而不是在项目里形成多套互不相干的日志路径。

## 配置命名空间

日志配置继续使用扁平化 KV 模型，主要分为三类。也就是说，日志拓扑仍然可以用 Go-Spring 的 path 规则表达。

`logger.*` 配置 Logger。

```properties
logger.async.type = AsyncLogger
logger.async.tag = _app_*
logger.async.level = INFO
logger.async.appenderRef[0].ref = console
logger.async.appenderRef[1].ref = file
```

`appender.*` 配置 Appender。

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

无命名空间前缀的配置可以作为变量复用。

```properties
log.dir = /var/log/app
log.level = INFO

appender.file.dir = ${log.dir}
logger.root.level = ${log.level}
```

这几段配置的语义是：`logger.*` 描述事件调度，`appender.*` 描述输出目标，无命名空间前缀的 key 可以作为普通配置变量被引用。Go-Spring 配置模型在这里继续发挥作用，日志配置本质上也是一组 path，最终绑定到 Logger、Appender 和 Layout 插件上。因为底层仍然是同一套配置模型，日志系统不需要再发明另一套配置规则。

## level 语义

`level` 支持单个级别和范围。同一个字段既可以表达“从某个级别开始输出”，也可以表达一个级别区间。

```properties
logger.root.level = INFO
logger.error_only.level = WARN~FATAL
logger.debug_info.level = DEBUG~WARN
```

单个级别表示输出该级别及以上。范围使用左闭右开区间 `[MinLevel, MaxLevel)`。因此 `WARN~FATAL` 会覆盖 `WARN`、`ERROR`、`PANIC`，但不包含 `FATAL`；如果要包含更高边界，可以使用 `MAX` 作为上限。

## 数组配置

如果一个 Logger 要输出到多个目标，数组配置可以用索引方式稳定表达顺序和局部属性。下面的配置让同一个 Logger 把不同级别段分发给不同 Appender。

```properties
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[0].level = DEBUG~WARN
logger.root.appenderRef[1].ref = file
logger.root.appenderRef[1].level = INFO~MAX
```

这段配置的语义是：`console` 只接收 `DEBUG~WARN`，`file` 接收 `INFO~MAX`。Appender 引用本身是数组，因此顺序和每个目标上的局部属性都能稳定表达。

如果只是声明多个标签，简单字符串列表可以用逗号。标签数组也可以用更紧凑的形式表达。

```properties
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
```

等价于下面这种写法。

```properties
logger.biz.tag[0]=_biz_order_*
logger.biz.tag[1]=_biz_user_*
logger.biz.tag[2]=_biz_pay_*
```

## 插件注入

Logger、Appender、Layout 都通过插件机制创建。插件字段通过 tag 声明配置注入。普通属性和子插件分别使用不同注入方式。

普通属性使用 `PluginAttribute`。下面这个 Appender 把文件目录、文件名、滚动周期和锁策略都交给配置注入。

```go
type RollingFileAppender struct {
	log.AppenderBase

	FileDir  string        `PluginAttribute:"dir,default=./logs"`
	FileName string        `PluginAttribute:"file"`
	Interval time.Duration `PluginAttribute:"interval,default=1h"`
	MaxAge   time.Duration `PluginAttribute:"maxAge,default=168h"`
	SyncLock bool          `PluginAttribute:"syncLock,default=false"`
}
```

子插件使用 `PluginElement`。这类字段不是简单值，而是另一个需要由插件系统创建的对象。

```go
type ConsoleLogger struct {
	log.LoggerBase

	Layout log.Layout `PluginElement:"layout,default=TextLayout"`
}
```

配置中只需要声明子插件类型和它自己的属性，插件系统会递归创建 Layout。

```properties
logger.console.type = ConsoleLogger
logger.console.layout.type = JSONLayout
logger.console.layout.fileLineMaxLength = 60
```

自定义插件使用前需要先注册，日志配置里的 `type` 才能找到对应实现。

```go
func init() {
	log.RegisterPlugin[SamplingAppender]("SamplingAppender")
}
```

这些代码的语义是：`PluginAttribute` 注入字符串、布尔值、`time.Duration` 等普通属性；`PluginElement` 注入另一个插件对象。自定义插件使用前需要先注册，日志配置里的 `type` 才能找到对应实现。插件可以实现 `Start()` 和 `Stop()` 参与生命周期管理，资源型 Logger 或 Appender 适合把连接、文件句柄、后台 goroutine 的启动和停止放在这里。

## 错误边界

插件如果需要自定义配置类型，可以注册转换器。转换器只负责把配置字符串转换成目标类型。

```go
type Converter[T any] func(string) (T, error)
```

日志级别范围这类配置，就是典型的自定义类型。转换失败说明配置本身非法，应该在配置加载或刷新阶段暴露出来。

日志写入错误属于运行期输出失败。如果这类错误再通过日志系统记录，可能造成递归，所以 Go-Spring 使用全局错误回调。

```go
log.ReportError = func(err error) {
	metric.Incr("log_write_error_total")
}
```

这段回调的语义是把日志系统自身的写入错误交给外部观测系统处理。回调保持轻量即可，避免 panic 和耗时操作。

## 配置刷新

单独使用日志组件时，可以调用 `log.RefreshConfig`。下面的例子用一组扁平化配置动态替换日志拓扑。

```go
err := log.RefreshConfig(map[string]string{
	"appender.console.type":        "ConsoleAppender",
	"appender.console.layout.type": "TextLayout",

	"logger.sync.type":               "SyncLogger",
	"logger.sync.tag":                "_app_*",
	"logger.sync.level":              "INFO",
	"logger.sync.appenderRef[0].ref": "console",
})
```

这段代码的语义是创建新的 Appender 和 Logger，并把 `_app_*` 标签重新绑定到新 Logger。Go-Spring 应用框架内部使用 `log.Refresh` 从配置系统刷新日志配置。这样日志刷新会跟配置系统的刷新入口保持一致。

刷新日志配置时，应关注旧 Logger/Appender 的停止和新配置的启动是否成功。如果是生产环境，通常需要先在预发环境验证配置合法性。

## GetLogger

已有项目或第三方库可能按 name 获取 logger。Go-Spring 提供了 `GetLogger`，用于兼容这类按名称取日志器的代码。

```go
rootLogger := log.GetLogger("root")
rootLogger.Write(log.InfoLevel, []byte("hello world\n"))
```

这段代码的语义是绕过标签路由，直接向名为 `root` 的 Logger 写入原始字节。使用它之前，配置中需要存在同名 Logger。

```properties
logger.root.type = FileLogger
logger.root.level = INFO
logger.root.dir = ./logs
logger.root.file = app.log
logger.root.layout.type = JSONLayout
```

## 标准库 log 适配

标准库 `log` 通过 `io.Writer` 输出。所以，实现 Writer 即可转发到 Go-Spring。第三方依赖里暂时不能替换的 `log.Print` 调用，也可以通过这个入口进入统一管线。

```go
type StdLogWriter struct {
	logger *log.LoggerWrapper
}

func (w *StdLogWriter) Write(p []byte) (int, error) {
	w.logger.Write(log.InfoLevel, p)
	return len(p), nil
}

func main() {
	stdlog.SetOutput(&StdLogWriter{
		logger: log.GetLogger("root"),
	})
}
```

这段适配的语义是把标准库日志的字节内容作为 `INFO` 级别写入指定 Logger。这样第三方依赖通过标准库输出的日志也能进入统一日志管线，但字段结构已经在标准库输出阶段丢失，不能再恢复成 Go-Spring 的强类型字段。

## Zap 适配

Zap 可以通过实现 `zapcore.Core` 适配。这个适配点说明 Go-Spring 不要求项目一次性替换所有日志调用，而是可以让 Zap 继续暴露自己的 API，同时把最终写入动作交给 Go-Spring 日志管线。

- `Enabled` 委托给 Go-Spring Logger 判断级别。
- `Write` 将 Zap 事件编码后转发给 Go-Spring Logger。
- `Sync` 交由 Go-Spring 自身生命周期处理。

适配后的语义是：级别判断、事件写入和生命周期逐步转向 Go-Spring，调用点可以分批迁移。新代码使用 Go-Spring 原生日志 API，旧代码和依赖仍可通过 Zap 输出到同一目标。

## 日志治理

至此，Go-Spring 日志系统从调用 API、字段模型、Logger、Appender、Layout、上下文提取到配置治理已经完整展开。它不只是输出文本，而是围绕结构化观测、可配置路由和生态适配组织起来的一套能力。

日志治理的核心是让运行期拓扑可以被配置描述、被插件实例化、被刷新替换，并让旧入口逐步进入同一条管线。这样日志系统才能从一组调用函数，变成 Go-Spring 应用运行期可维护的观测基础设施。
