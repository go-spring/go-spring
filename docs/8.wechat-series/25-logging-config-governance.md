# Go-Spring 实战第 25 课 —— 日志治理：配置拓扑、插件注入与刷新

上一篇讲完上下文提取以后，Go-Spring 的日志事件已经可以带着业务字段和链路字段进入输出管线。到这里，单次日志输出已经说清楚了，但一套日志系统真正落到项目里，还会遇到另一类问题：运行期拓扑怎样维护。

Logger、Appender、Layout 不能散落在代码里临时创建，否则不同环境、不同项目和不同部署阶段很快会出现多套日志路径。配置怎样描述拓扑，插件怎样实例化，写入错误怎样上报，刷新时新旧实例怎样切换，这些问题决定日志系统能不能长期维护。

Go-Spring 的处理方式是把运行期拓扑交给配置描述，把实例创建交给插件系统，把拓扑替换交给刷新入口。这样日志治理仍然围绕同一条输出管线展开，而不是在项目里形成多套互不相干的日志路径。

## 日志拓扑

日志配置没有另起一套格式，它继续使用 Go-Spring 的扁平化 KV 模型。也就是说，日志拓扑仍然可以用 path 表达：`logger.*` 描述事件调度，`appender.*` 描述输出目标，普通配置 key 可以作为变量复用。

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

这几段配置最终会绑定到 Logger、Appender 和 Layout 插件上。Go-Spring 配置模型在这里继续发挥作用，日志系统不需要再发明另一套配置规则；环境变量、Profile、命令行覆盖和变量引用也都能沿用前面讲过的配置语义。

## level

日志拓扑里最容易影响输出结果的是 `level`。Go-Spring 的 `level` 支持单个级别和范围，同一个字段既可以表达“从某个级别开始输出”，也可以表达一个级别区间。

```properties
logger.root.level = INFO
logger.error_only.level = WARN~FATAL
logger.debug_info.level = DEBUG~WARN
```

单个级别表示输出该级别及以上。范围使用左闭右开区间 `[MinLevel, MaxLevel)`。因此 `WARN~FATAL` 会覆盖 `WARN`、`ERROR`、`PANIC`，但不包含 `FATAL`；如果要包含更高边界，可以使用 `MAX` 作为上限。

## appenderRef

如果一个 Logger 要输出到多个目标，`appenderRef` 用数组表达顺序和每个目标上的局部属性。下面的配置让同一个 Logger 把不同级别段分发给不同 Appender。

```properties
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[0].level = DEBUG~WARN
logger.root.appenderRef[1].ref = file
logger.root.appenderRef[1].level = INFO~MAX
```

这段配置的语义是：`console` 只接收 `DEBUG~WARN`，`file` 接收 `INFO~MAX`。Appender 引用本身是数组，因此顺序和每个目标上的局部属性都能稳定表达。

如果只是声明多个标签，简单字符串列表可以用逗号表达。

```properties
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
```

等价于下面这种写法。

```properties
logger.biz.tag[0]=_biz_order_*
logger.biz.tag[1]=_biz_user_*
logger.biz.tag[2]=_biz_pay_*
```

这两种写法进入 Go-Spring 以后都是同一组标签数组。短列表可以用逗号提高可读性；当数组元素有子属性时，仍然使用下标形式更清楚。

## 插件注入

日志拓扑写在配置里以后，还需要把配置变成实际对象。Go-Spring 用插件机制创建 Logger、Appender 和 Layout。插件字段通过 tag 声明配置注入，普通属性和子插件分别使用不同注入方式。

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

这些代码的语义是：`PluginAttribute` 注入字符串、布尔值、`time.Duration` 等普通属性；`PluginElement` 注入另一个插件对象。插件可以实现 `Start()` 和 `Stop()` 参与生命周期管理，资源型 Logger 或 Appender 适合把连接、文件句柄、后台 goroutine 的启动和停止放在这里。

## 错误边界

配置拓扑和插件实例化都属于启动或刷新阶段。插件如果需要自定义配置类型，可以注册转换器。转换器只负责把配置字符串转换成目标类型。

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

这段回调的语义是把日志系统自身的写入错误交给外部观测系统处理。回调保持轻量即可，避免 panic 和耗时操作。也就是说，配置错误应该阻止拓扑创建或刷新成功，写入错误则应该从日志系统外部上报。

## 配置刷新

当日志拓扑需要在运行期替换时，可以刷新配置。单独使用日志组件时调用 `log.RefreshConfig`，下面的例子用一组扁平化配置动态替换日志拓扑。

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

刷新不是修改某个字段那么简单，而是替换一组运行期对象。生产环境刷新日志配置时，应关注旧 Logger/Appender 的停止和新配置的启动是否成功，并先在预发环境验证配置合法性。

## 日志治理

日志治理的核心是让运行期拓扑可以被配置描述、被插件实例化、被刷新替换。这样日志系统才能从一组调用函数，变成 Go-Spring 应用运行期可维护的观测基础设施。
