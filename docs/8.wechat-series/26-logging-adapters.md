# Go-Spring 实战第 26 课 —— 日志适配：让标准库 log 和 Zap 进入统一管线

上一课把日志拓扑、插件注入和刷新说清楚以后，Go-Spring 日志系统已经可以由配置统一治理。但真实项目里还有一个更现实的问题：已有代码和第三方库不一定会立刻改成 Go-Spring 的标签 API。

有些历史模块按 name 获取 logger，有些依赖直接使用标准库 `log`，还有些项目已经围绕 Zap 建立了调用习惯。如果迁移要求一次性替换所有日志入口，成本会很高，也容易在迁移期形成两套输出路径。

Go-Spring 的日志适配不是再造一套调用 API，而是把旧入口逐步接回同一条日志管线。新代码继续使用标签和结构化字段，旧代码通过适配层进入已经配置好的 Logger、Appender 和 Layout。

## GetLogger

Go-Spring 提供 `GetLogger`，用于兼容按 name 获取 logger 的代码。

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

`GetLogger` 适合迁移和适配，不适合替代新代码里的标签路由。新代码如果直接按 name 写日志，就会丢掉标签路由在模块隔离、级别控制和输出分发上的表达力。

因此，`GetLogger` 更像一个桥接入口：当某段代码暂时只能输出字节，或者某个旧接口要求传入固定 Logger 时，可以先接入统一输出目标，再逐步迁移调用点。

## 标准库 log

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

这段适配的语义是把标准库日志的字节内容作为 `INFO` 级别写入指定 Logger。这样第三方依赖通过标准库输出的日志也能进入统一日志管线。

这个入口有一个天然边界：字段结构已经在标准库输出阶段丢失，不能再恢复成 Go-Spring 的强类型字段。适配层能统一输出目标、级别和生命周期，但不能把普通字符串重新还原成结构化事件。

如果某个内部模块仍然大量使用标准库 `log`，可以先通过 Writer 接入统一输出，再逐步把关键业务事件改成 Go-Spring 原生日志 API。这样迁移不会影响输出收集，同时高价值事件也能逐渐恢复成结构化字段。

## Zap 适配

Zap 可以通过实现 `zapcore.Core` 适配。这个适配点说明 Go-Spring 不要求项目一次性替换所有日志调用，而是可以让 Zap 继续暴露自己的 API，同时把最终写入动作交给 Go-Spring 日志管线。

- `Enabled` 委托给 Go-Spring Logger 判断级别。
- `Write` 将 Zap 事件编码后转发给 Go-Spring Logger。
- `Sync` 交由 Go-Spring 自身生命周期处理。

适配后的语义是：级别判断、事件写入和生命周期逐步转向 Go-Spring，调用点可以分批迁移。已有 Zap 调用仍然保留原来的代码形态，但输出目标、刷新和停止流程回到 Go-Spring 的运行期拓扑。

Zap 适配和标准库 `log` 适配的差异在于，Zap 调用点原本就可能携带结构化字段。具体适配实现可以选择把 Zap 事件编码成字节后写入 Go-Spring Logger，也可以在更深的适配层保留字段信息。无论选择哪种方式，边界都应该清楚：适配层负责接入，不应该重新定义日志治理规则。

## 迁移边界

日志适配最容易被误用成“新老 API 随便混用”。更稳妥的做法是把入口分层：新业务代码使用 Go-Spring 原生日志 API，历史模块和第三方依赖通过适配层进入统一管线。

适配入口要尽量少而稳定。比如统一使用 `root` 或少量命名 Logger 接收旧日志，再通过配置决定它们写向控制台、文件或远端目标。这样旧日志不会散落成很多不可治理的临时 Logger。

迁移过程中还要接受一个事实：旧入口写入的通常是字节或已有编码结果，它们最多能进入统一输出、级别和生命周期管理，不一定能获得 Go-Spring 原生字段模型的全部能力。真正需要检索和聚合的业务事件，仍然应该逐步迁移到结构化调用点。

## 日志适配

日志适配的价值是降低迁移成本，而不是扩大日志入口。`GetLogger`、标准库 `log` 和 Zap 适配都应该服务同一个目标：让旧代码先进入统一日志管线，让新代码继续使用标签和结构化字段，最终让项目只维护一套日志拓扑和运行期治理规则。
