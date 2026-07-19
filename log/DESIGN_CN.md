# log 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`go-spring.org/log` 是 Go-Spring 的结构化日志库，处在 `stdlib` 同层（零
业务依赖——只依赖 `go-spring.org/stdlib` 与 Go 标准库），被 `spring/` 与
所有 `starter-*` 消费。目标是一个可插拔、配置驱动、热路径上追求单事件
零分配的日志库。

## 1. 职责与边界

- 发射结构化日志事件：等级（`Trace` … `Fatal`）、tag、上下文字段、可
  插拔输出格式。
- 从扁平属性 map（`RefreshConfig`）或配置文件（`RefreshFile`）加载配
  置，让应用能不重启就热更新日志拓扑。
- **不**承担远端传输（Kafka、ES、Loki）。内置 sink 只有 console / file
  / rolling-file；其它形态各自实现 `Appender` 插件注册进来。

## 2. 关键抽象与接缝

- **插件注册表**。`RegisterPlugin[T](name)`（见 `log/plugin.go`）维护
  `name → reflect.Type` 映射。配置里的 `type=JSONLayout` 一类值通过它解
  析；结构体上的 `PluginAttribute` / `PluginElement` tag 声明如何从扁平
  存储里注入标量属性与子插件。库内自带三类插件：
  - **Appender**（`plugin_appender.go`）：`DiscardAppender`、
    `ConsoleAppender`、`FileAppender`、`RollingFileAppender`。
  - **Layout**（`plugin_layout.go`）：`TextLayout`、`JSONLayout`，都
    嵌入带 `fileLineMaxLength` 的 `BaseLayout`。
  - **Logger**（`plugin_logger.go`）：`SyncLogger`（`"Logger"` 别
    名）、`AsyncLogger`、`DiscardLogger`、`ConsoleLogger`、
    `FileLogger`、`RollingFileLogger`。`AppenderRef` 把 logger 关联到
    命名 appender。
- **Tag 系统**。`RegisterTag(name)`（`log/log_tag.go`）返回 `*Tag`，其
  `Logger` 在 refresh 时被原子替换。tag 是调用侧 API——代码只写
  `log.Infof(ctx, TagRequestIn, ...)`，从不持有 `Logger` 值。Refresh 用
  正则把 tag 名映射到配置里的 logger。
- **Refresh 管道**。`RefreshConfig(map)` → `parseExpr`（展开 `!` 内联
  map 表达式）→ `flatten.NewProperties` → `Refresh(storage)`。Refresh
  用 `sync/atomic` 指针原子替换全局 logger / appender 集合，读端无锁。
  `global.refreshed` 是单向闩锁：refresh 之后再调 `RegisterTag` 会
  panic——tag 只能在包 init 期声明。
- **上下文字段提取**。`StringFromContext` 和 `FieldsFromContext` 是包
  级函数变量，启动时设置一次（通常由 `starter-otel` 设为写入
  `trace_id`/`span_id`）。这是跨切面上下文数据的官方接入点。
- **字段编码**。`Field`（`log/field.go`）是值类型，包含 `Key`、`Type`
  （`ValueType`）、`Num`（数值载荷）、`Any`（指针/切片载荷）。基础类型
  helper（`Bool`、`Int64`、`String`、`Msg`、`Msgf`、`Reflect`、
  `Array`、`Object`、`FieldsFromMap`）造字段时不会每次分配 slice。
  `Event` 与编码 buffer 走 `sync.Pool`（`plugin_appender.go` 的
  `bufferPool`；超过 `bufferCap`（默认 10 KB、可用环境变量
  `GS_LOGGER_BUFFER_CAP` 覆盖）的 buffer 不回池）。
- **生命周期**。logger 与 appender 可实现 `Lifecycle`（`Start`/`Stop`）；
  `Refresh` 启动新插件并停掉被替换的旧插件。

## 3. 约束

- 零依赖 `spring/`（这是基础库）。耦合方向单向：`spring/` 与 starter 依
  赖 `log`，反向禁止。
- 所有 tag 名与 converter 必须在首次 `Refresh` 之前注册。之后再注册会
  panic——这是原子替换契约的一部分。
- Tag 字符串须满足 `isValidTag`（3–36 字符，仅小写字母 / 数字 / 下划线，
  1–4 段，允许可选单个开头下划线）；`RegisterAppTag` /
  `RegisterBizTag` / `RegisterRPCTag` 等 helper 强制 `_<main>_<sub>[_<action>]`
  的形式。
- Appender 写入拿的是池化 buffer；`EncodeTo` 之外不要留住 `Field.Any`
  的 slice 或 `*bytes.Buffer`。

## 4. 权衡与被否决的方案

- **不采用 Log4j2 XML 风格配置**。Go-Spring 采用扁平属性 map + `!` 内
  联表达式（`db!: "{host: localhost, port: 5432}"`），因为
  `flatten.Storage` 是全框架共享的配置原语。Layout / logger 插件走的注
  入路径与任何框架 bean 一致。
- **不暴露全局 logger 单例**。`GetLogger(name)` 仅为兼容老调用点
  （`Write(level, []byte)`）保留；新代码走 tag，让 refresh 能重绑。
- **不把 `zap.Sugar` 风格格式化包装作为主路径**。主 API 接收
  `func() []Field` 构造函数，等级被禁用的调用点不会为字段 slice 分配。
  `*f` 版本存在但走的是较慢路径。
- **只做内置 sink**。Kafka/OTel/HTTP sink 不进本包以维持 `log` 零依
  赖；这类实现放到 starter 里，由 starter 调 `RegisterPlugin` 注册自己
  的 appender 类型。
