# Go-Spring 实战第 26 课 —— 日志适配：统一旧入口与第三方日志

上一篇介绍了怎样沿着 Logger、Appender、Layout 和 Encoder 的边界扩展日志系统。还有另一类问题不是增加新能力，而是项目里已经存在标准库 `log`、`slog`、Zap 或历史日志封装，短期内无法全部改成 Go-Spring 的 Tag API。

这时需要做的是适配：保留原来的调用入口，把日志重新接入 Go-Spring 的级别过滤、输出目标、异步调度和生命周期。适配可以统一输出管线，但不同接入方式能够保留的信息并不相同。

## 两种适配路径

已有日志进入 Go-Spring，大致有两条路径。

| 路径 | 做法 | 能保留的内容 |
| --- | --- | --- |
| 字节转发 | 先由原日志库编码，再写入命名 Logger | 原日志库生成的完整字节 |
| 事件转换 | 把级别、消息和字段转换成 Go-Spring Event | Tag、上下文和结构化 Field |

字节转发接入最简单。标准库 `log` 已经生成文本，Zap 或 `slog` 也可以先生成 JSON，然后把结果交给 Go-Spring。

但这些内容进入 Go-Spring 时已经是 `RawBytes`。Logger 仍然会执行级别过滤和同步异步调度，Appender 仍然负责输出目标，Layout 和 Encoder 则不会再次处理这段字节。

事件转换需要实现更深的适配器，把外部日志库的 Level、Message 和 Field 转换成 Go-Spring 的 Level 和 Field，再调用 `Record`。它能保留结构化能力，但也要处理字段类型、分组、调用位置和 Panic/Fatal 语义等差异。

## GetLogger

字节转发的基础入口是 `GetLogger`。它按照名称返回一个稳定的 LoggerWrapper。

```go
var legacyLogger = log.GetLogger("legacy")

func WriteLegacyMessage(b []byte) {
	legacyLogger.Write(log.InfoLevel, append([]byte(nil), b...))
}
```

LoggerWrapper 内部持有当前 Logger 的原子引用。日志配置刷新以后，已有调用方会自动使用新的 Logger，不需要重新获取 Wrapper。

`GetLogger` 只能在初始化阶段调用。它维护的名称注册表不是为运行期动态增删设计的。刷新日志配置时，所有已经获取过的名称都必须存在于新配置中，否则刷新会失败。

在 Go-Spring App 中，可以为旧入口提供一个普通 Logger。

```properties
logging.logger.legacy.type = FileLogger
logging.logger.legacy.tag = _legacy_*
logging.logger.legacy.level = INFO
logging.logger.legacy.dir = .
logging.logger.legacy.file = legacy.log
```

命名 Logger 和 Tag 路由是两套入口。这里的 `tag` 仍然是非 Root Logger 的必填配置，但 `LoggerWrapper.Write` 不会根据它重新路由，名称只用于在刷新时绑定对应 Logger。

`Write` 创建的是 RawBytes Event。Appender 会直接写出传入字节，因此不会自动补充时间、Tag、调用位置、上下文字段，也不会再使用配置中的 Layout 编码。

## 标准库 log

标准库 `log.Logger` 通过 `io.Writer` 输出。实现一个 Writer，就可以把现有调用接到命名 Logger。

```go
type LogWriter struct {
	logger *log.LoggerWrapper
	level  log.Level
}

func (w *LogWriter) Write(p []byte) (int, error) {
	b := append([]byte(nil), p...)
	w.logger.Write(w.level, b)
	return len(p), nil
}

var stdLogger = log.GetLogger("standard")

func init() {
	stdlog.SetOutput(&LogWriter{
		logger: stdLogger,
		level:  log.InfoLevel,
	})
}
```

配置中也要定义 `logging.logger.standard`。它可以和其他命名 Logger 使用相同的输出目标，但名称不能缺失。

Writer 中需要复制字节。标准库只保证 `Write` 调用期间可以使用 `p`，而命名 Logger 可能异步处理 Event。直接保留原切片，会把缓冲区复用问题带进后台写入。

标准库日志经过适配后，可以共享 Go-Spring 的输出文件、滚动策略和关闭流程。但 Writer 返回 nil 只表示数据已经交给 Logger，异步缓冲区丢弃或目标写入失败不会通过这次 `Write` 返回给标准库调用方。

标准库默认生成的时间和前缀也已经包含在 RawBytes 中。要避免重复格式，应当由标准库决定这段文本的完整样式，不要期待 Go-Spring Layout 再补充一层。

## slog

`log/slog` 同时支持字节转发和事件转换。

最简单的方式是让 TextHandler 或 JSONHandler 把编码结果写入前面的 LogWriter。

```go
writer := &LogWriter{
	logger: log.GetLogger("slog"),
	level:  log.InfoLevel,
}

logger := slog.New(slog.NewJSONHandler(writer, nil))
```

这里的 `slog` 名称同样需要出现在日志配置中。

这种方式保留 slog 生成的 JSON 字段，但对 Go-Spring 来说仍然是一段 RawBytes。Go-Spring 无法再按字段检索、重命名或补充自己的上下文字段，而且 Writer 上固定的 level 不能表达每条 slog Record 的真实级别。

需要保留级别和结构化字段时，应当实现自定义 `slog.Handler`。Handler 可以把 `slog.Record.Level` 映射成 Go-Spring Level，把 Message 和 Attr 转换成 Field，再选择固定 Tag 调用 `log.Record`。

这种转换还要明确几类差异：`WithGroup` 怎样映射成嵌套 Object，`LogValuer` 什么时候求值，slog 的 source PC 怎样处理，以及 error 等值怎样编码。只有项目确实依赖统一字段模型时，才值得实现完整 Handler。

## Zap

Zap 的扩展入口是 `zapcore.Core` 和 `zapcore.WriteSyncer`。

如果目标只是统一输出位置，可以让 Zap Encoder 先生成文本或 JSON，再通过 WriteSyncer 或自定义 Core 写入 LoggerWrapper。这和标准库适配相同，改造范围小，但 Go-Spring 看到的仍然是 RawBytes。

如果要保留结构化模型，自定义 Core 需要完成下面几件事：

1. 把 `zapcore.Level` 映射成 Go-Spring Level。
2. 合并 `With` 保存的公共字段和本次 Write 的字段。
3. 把 Zap Field 转换成 Go-Spring Field。
4. 选择 Tag，并调用 `Record` 或对应的原生日志 API。
5. 明确 DPanic、Panic、Fatal 和 Sync 的语义。

Zap Field 包含对象、数组、命名空间、延迟求值和自定义 Marshaler。简单地把所有值转成字符串虽然容易实现，却会丢失字段类型。完整转换则需要逐类处理，并为无法直接映射的字段保留降级策略。

Go-Spring 日志库提供的是 `GetLogger` 和 `Record` 这类基础桥接入口，不会自动完成所有第三方字段转换。适配深度应由迁移目标决定。

## 迁移边界

适配不是把两套日志模型变成完全相同，而是在迁移期选择哪些能力需要保留。

| 场景 | 建议 |
| --- | --- |
| 第三方依赖只接受 `io.Writer` | 使用 RawBytes 转发 |
| 历史模块短期不能改调用 API | 使用少量命名 Logger 收口 |
| 需要统一文件、滚动和生命周期 | 字节转发通常已经足够 |
| 需要统一 Tag、上下文和字段检索 | 实现事件转换适配器 |
| 新业务代码 | 直接使用 Tag 和 Field |

RawBytes 路径仍然会经过 Logger、AppenderRef、Appender 和异步缓冲，因此可以统一输出目标和运行策略。但它绕过 Layout 和 Encoder，也无法自动恢复已经丢失的结构。

事件转换路径可以继续使用 Go-Spring 的 Tag 路由、上下文提取和 Field 编码，但适配器本身会成为需要长期维护的协议层。外部日志库升级字段模型或级别语义时，这一层也要跟着验证。

因此，适配更适合作为边界，而不是新的默认入口。历史代码和第三方组件通过 `GetLogger`、Writer、Handler 或 Core 接入统一管线；新代码直接使用 Go-Spring 原生日志 API。这样既能逐步收敛输出治理，也不会因为一次性迁移扩大改造范围。
