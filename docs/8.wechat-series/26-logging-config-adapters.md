# Go-Spring 实战第 26 课：日志配置与生态适配：热更新、标准库 log 和 Zap 接入

Go-Spring 的日志 API、字段、Logger、输出管线和上下文提取都搭起来之后，问题就会进入工程治理层面。

配置如何组织，插件如何注入，错误如何上报，配置如何刷新，已有日志入口如何接入，这些问题决定日志系统能不能在真实项目里长期维护。所以这一篇收束 Go-Spring 日志系统的配置和生态适配能力。

这一篇不用按 API 列表死记。可以先抓三件事：配置负责描述日志拓扑，插件负责把 Logger、Appender、Layout 实例化，适配层负责让标准库 `log`、Zap 这类既有入口进入同一条管线。

如果只是给应用落一套日志配置，重点看命名空间、level、数组配置和插件字段注入；如果要做框架或 Starter，再关注错误回调、刷新和生态适配。

## 日志配置先按命名空间拆开

日志配置采用扁平化 KV 模型，主要分为三类。我们先从命名空间看起。

`logger.*` 配置 Logger：

```properties
logger.async.type = AsyncLogger
logger.async.tag = _app_*
logger.async.level = INFO
logger.async.appenderRef[0].ref = console
logger.async.appenderRef[1].ref = file
```

`appender.*` 配置 Appender：

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

无命名空间前缀的配置可以作为变量复用：

```properties
log.dir = /var/log/app
log.level = INFO

appender.file.dir = ${log.dir}
logger.root.level = ${log.level}
```

我们前面讲过的 Go-Spring 配置模型在这里继续发挥作用：日志配置本质上也是一组 path，最终绑定到 Logger、Appender 和 Layout 插件上。这样日志系统不需要再发明另一套配置规则。

## level 支持单级别和范围

`level` 支持单个级别和范围：

```properties
logger.root.level = INFO
logger.error_only.level = WARN~FATAL
logger.debug_info.level = DEBUG~WARN
```

单个级别表示输出该级别及以上。范围使用左闭右开区间 `[MinLevel, MaxLevel)`。

## 数组配置表达多目标和多标签

如果配置比较复杂，数组可以使用索引方式：

```properties
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[0].level = DEBUG~WARN
logger.root.appenderRef[1].ref = file
logger.root.appenderRef[1].level = INFO~MAX
```

简单字符串列表可以用逗号：

```properties
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
```

等价于：

```properties
logger.biz.tag[0]=_biz_order_*
logger.biz.tag[1]=_biz_user_*
logger.biz.tag[2]=_biz_pay_*
```

## 插件字段由配置注入

Logger、Appender、Layout 都通过插件机制创建。插件字段通过 tag 声明配置注入。

普通属性使用 `PluginAttribute`：

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

子插件使用 `PluginElement`：

```go
type ConsoleLogger struct {
	log.LoggerBase

	Layout log.Layout `PluginElement:"layout,default=TextLayout"`
}
```

配置中只需要声明子插件类型和它自己的属性，插件系统会递归创建 Layout：

```properties
logger.console.type = ConsoleLogger
logger.console.layout.type = JSONLayout
logger.console.layout.fileLineMaxLength = 60
```

插件使用前需要先注册：

```go
func init() {
	log.RegisterPlugin[SamplingAppender]("SamplingAppender")
}
```

插件可以实现 `Start()` 和 `Stop()` 参与生命周期管理。

## 转换失败和写入错误要单独处理

插件如果需要自定义配置类型，可以注册转换器：

```go
type Converter[T any] func(string) (T, error)
```

日志级别范围这类配置，就是典型的自定义类型。

日志写入错误就不能再通过日志系统记录，否则可能造成递归。因此 Go-Spring 使用全局错误回调：

```go
log.ReportError = func(err error) {
	metric.Incr("log_write_error_total")
}
```

回调应保持轻量，不能 panic，也不应做耗时操作。

## 刷新配置要关注新旧实例切换

单独使用日志组件时，可以调用 `log.RefreshConfig`：

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

Go-Spring 应用框架内部使用 `log.Refresh` 从配置系统刷新日志配置。

刷新日志配置时，应关注旧 Logger/Appender 的停止和新配置的启动是否成功。如果是生产环境，通常需要先在预发环境验证配置合法性。

## GetLogger 兼容按名称取 logger

已有项目或第三方库可能按 name 获取 logger。Go-Spring 提供 `GetLogger`：

```go
rootLogger := log.GetLogger("root")
rootLogger.Write(log.InfoLevel, []byte("hello world\n"))
```

使用它之前，配置中必须存在同名 Logger：

```properties
logger.root.type = FileLogger
logger.root.level = INFO
logger.root.dir = ./logs
logger.root.file = app.log
logger.root.layout.type = JSONLayout
```

## 标准库 log 通过 Writer 接入

标准库 `log` 通过 `io.Writer` 输出。因此，实现 Writer 即可转发到 Go-Spring：

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

这样第三方依赖通过标准库输出的日志也能进入统一日志管线。

## Zap 通过 Core 接入

Zap 可以通过实现 `zapcore.Core` 适配。核心思路是：

- `Enabled` 委托给 Go-Spring Logger 判断级别。
- `Write` 将 Zap 事件编码后转发给 Go-Spring Logger。
- `Sync` 交由 Go-Spring 自身生命周期处理。

适配后，项目就可以逐步迁移：新代码使用 Go-Spring 原生日志 API，旧代码和依赖仍可通过 Zap 输出到同一目标。这样迁移不需要一次性改完所有日志调用点。

## 日志板块收束到配置治理

至此，Go-Spring 日志系统从调用 API、字段模型、Logger、Appender、Layout、上下文提取到配置治理已经完整展开。它不只是输出文本，而是围绕结构化观测、可配置路由和生态适配组织起来的一套能力。

日志让运行状态可观测。回到服务入口，内置 HTTP Server 则负责把 Web 服务接入 Go-Spring 的启动、就绪和关闭生命周期。
