# Go-Spring Log 高性能日志库

<p>
<img src="https://img.shields.io/github/license/go-spring/log" alt="license"/>
<img src="https://img.shields.io/github/go-mod/go-version/go-spring/log" alt="go-version"/>
<img src="https://img.shields.io/github/v/release/go-spring/log?include_prereleases" alt="release"/>
<a href="https://codecov.io/gh/go-spring/log">
   <img src="https://codecov.io/gh/go-spring/log/graph/badge.svg?token=QBCHVEK97Q" alt="test-coverage"/>
</a>
<a href="https://deepwiki.com/go-spring/log"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

[English](README.md) | 中文

> 项目已正式发布，欢迎使用！

**Go-Spring Log** 是专为 Go 语言设计的**高性能、可扩展**结构化日志库。它提供灵活的标签分类系统、上下文链路信息抽取、多级日志配置和多种输出方式，非常适合服务端应用开发。

## 特性

- **多级日志体系**：支持 `Trace`、`Debug`、`Info`、`Warn`、`Error`、`Panic`、`Fatal` 标准日志级别，满足开发调试和线上监控各种场景
- **结构化日志**：以键值对格式记录日志，天然支持 `trace_id`、`user_id` 等链路信息，便于日志系统聚合分析
- **原生上下文集成**：可配置从 `context.Context` 中自动抽取链路追踪信息（如请求ID、用户ID），自动附加到日志条目中
- **基于 Tag 的日志分类**：创新的标签系统，通过标签区分不同模块/业务线的日志，支持层级后缀通配符匹配，无需显式创建 logger 实例即可使用统一 API
- **插件化架构**：
  - **Appender**：支持控制台、普通文件、时间滚动文件多种输出目标
  - **Layout**：提供纯文本和 JSON 两种输出格式，满足不同场景需求
  - **Logger**：同时支持同步和异步日志，异步模式不阻塞业务主线程
- **灵活的滚动日志**：按时间间隔自动切割，支持自动清理过期日志，可将警告及以上级别日志分离到独立文件
- **性能优化**：使用缓冲池复用、日志事件对象池，最小化内存分配开销，基准测试中表现优异
- **动态配置重载**：支持运行时通过 `RefreshConfig` 重新加载日志配置，无需重启应用；配置文件的读取与解析由调用方完成
- **完善测试覆盖**：核心模块均有单元测试覆盖，保证稳定可靠

## 核心概念

### Tag（标签）

Tag 是本日志库的核心概念，用于对日志进行分类。通过 `RegisterTag` 注册标签后，配置中可以使用后缀通配符进行分组匹配（例如 `_app_request_*` 匹配所有 `_app_request_` 开头的标签）。

这种设计使得无需显式创建 logger 实例即可使用统一 API 打日志，即使第三方库也能以标准化方式输出日志。框架会根据标签自动匹配到最具体的 logger。

官方推荐三类前缀，覆盖绝大多数后端场景：

| 前缀 | 适用场景 | 示例 |
|------|----------|------|
| `_app_` | 应用生命周期与基础设施（启动、配置、连接池、熔断等） | `_app_startup` |
| `_biz_` | 业务流程与领域事件 | `_biz_order_create` |
| `_rpc_` | 外部依赖调用（数据库、缓存、MQ、下游服务） | `_rpc_redis_get` |

```go
// 按分类便捷注册标签
var (
  TagAppStartup     = log.RegisterAppTag("startup", "init")     // 应用启动阶段
  TagBizOrderCreate = log.RegisterBizTag("order", "create")   // 订单创建业务
  TagRpcRedisQuery  = log.RegisterRPCTag("redis", "query")    // Redis 查询 RPC
)
```

#### 什么时候注册自定义 tag

大多数用户并不关心日志 tag，这是有意为之：未配置的 tag 回落到 default logger，这是基线行为而非降级。
tag 不是模块身份标识，而是调控句柄。只有当你能回答"谁需要把这批日志和主日志分开单独调整"时，
才应该注册自定义 tag。具体原则：

1. **默认使用 `log.TagAppDef` / `log.TagBizDef`。** 一次性的装配、生命周期和错误日志——框架和 starter
   日志的绝大多数——都属于这里。不要因为"这是个模块"就注册 tag。
2. **只有需要独立调控的日志人群才注册自定义 tag**：访问日志（量大、需独立采样或落盘）、重试/熔断事件、
   后台中继任务、第三方框架桥接进来的日志（用 `RegisterRPCTag` 单独收口）。说不出谁会单独调它，就用默认 tag。
3. **注册的 tag 是公开的配置契约。** `logger.<tag>.*` 从此成为用户可见的配置面，必须写进文档（如 starter
   的 README）；没写进文档的 tag 不应该存在。

命名：`subType` 用短技术/产品名（不加 `starter_` 或家族前缀——`_app_` 前缀已经表达了这层含义，且
4 段的长度预算很紧张）；可选的 `action` 用于区分同模块内的多类日志人群（如 `gin` 的 `access` 与生命周期
日志），单一人群留空。

### Logger（日志处理器）

Logger 是实际处理日志的组件。不同标签可以匹配到不同 logger，每个 logger 可以独立设置级别和输出。

### 日志级别

支持 `Trace`、`Debug`、`Info`、`Warn`、`Error`、`Panic`、`Fatal` 七个级别。注意：`Panic`
和 `Fatal` 只是级别语义（表示"伴随 panic/致命错误的日志"），本库记录日志后**不会**主动
panic 或退出进程，是否终止由调用方决定。

logger 的 `level` 支持两种形式：

- 单级别：`DEBUG`，表示该级别及以上（如 `INFO` 等价于 `INFO~FATAL`）
- 级别范围：`DEBUG~INFO`，只输出落在区间内的日志，可用来做"只看 WARN 以下"之类的过滤

未配置任何 logger 时，默认级别为 `INFO`（可用环境变量 `GS_LOGGER_DEFAULT_LEVEL` 覆盖）。

### 调用方信息

每条日志自动附带调用方的文件名和行号。默认使用 `fast` 模式（按调用点缓存，结果与
`default` 一致但更快），可通过环境变量 `GS_LOGGER_CALLER_TYPE` 调整：

| 取值 | 说明 |
|------|------|
| `fast` | 默认。按调用点 PC 缓存定位结果，输出与 `default` 完全一致，更快 |
| `default` | 每次直接走 `runtime.Caller` 解析，无缓存 |
| `none` | 不采集调用方信息，性能最优 |

### 上下文字段抽取

你可以配置钩子函数从 `context.Context` 中抽取上下文数据并自动加入每条日志：

- `log.StringFromContext`：从 context 抽取一个字符串（如请求 ID），以独立段落展示
- `log.FieldsFromContext`：从 context 返回结构化字段列表（如 `trace_id`、`span_id`）

两个钩子都是包级函数变量，在进程启动时设置一次即可（接入 `starter-otel` 后会自动设为
写入链路追踪字段）。抽取的字段排在业务字段之前，每条日志都会触发调用，请保持钩子轻量、
尽量读缓存值。

## 日志 API

各级别均提供两种风格：结构化版本与格式化版本。注意字段参数的形式分两档：

- `Trace` / `Debug` 接收**惰性构造函数** `func() []Field`--级别禁用时闭包不执行、零分配，
  适合高频调试日志
- `Info` ~ `Fatal` 接收**现成字段** `...Field`--调用前字段已构造

```go
log.Debug(ctx, tag, func() []log.Field {
	return []log.Field{log.String("user_id", "10001")} // 级别禁用时不执行
})
log.Info(ctx, tag, log.String("user_id", "10001"), log.Msg("登录成功"))
log.Infof(ctx, tag, "用户 %s 登录成功", userID)
```

常用字段构造函数：

| 类别 | 函数 |
|------|------|
| 消息 | `Msg` / `Msgf` |
| 标量 | `Nil` / `Bool` / `Int` / `Uint` / `Float` / `String`（各带 `XxxPtr` 可空版本） |
| 切片 | `Bools` / `Ints` / `Uints` / `Floats` / `Strings` |
| 复合 | `Reflect`（反射编码任意值）/ `Any` / `Array` / `Object`（嵌套字段）/ `FieldsFromMap` |

## 输出格式

**TextLayout**（默认，`||` 分隔的纯文本）：

```
[INFO][2026-08-19T10:23:45.123][main.go:42] _app_def||event=user_login||user_id=10001||msg=用户登录成功
```

**JSONLayout**（日志聚合系统友好）：

```json
{"level":"info","time":"2026-08-19T10:23:45.123","fileLine":"main.go:42","tag":"_app_def","event":"user_login","user_id":10001,"msg":"用户登录成功"}
```

上下文字段（`StringFromContext` / `FieldsFromContext` 的产出）会排在业务字段之前，
两种格式下均如此。

## 安装

```bash
go get go-spring.org/log
```

## 快速开始

```go
package main

import (
  "context"
  "encoding/json"
  "os"

  "go-spring.org/log"
  "go-spring.org/stdlib/flatten"
)

func main() {
  // 配置从 context 抽取链路字段
  log.FieldsFromContext = func(ctx context.Context) []log.Field {
    return []log.Field{
      log.String("trace_id", "0a882193682db71edd48044db54cae88"),
      log.String("span_id", "50ef0724418c0a66"),
    }
  }

  // 从配置文件加载日志配置。文件的读取与解析由调用方完成：
  // 先解码成 map，再扁平化，最后交给 RefreshConfig。
  b, err := os.ReadFile("log.json")
  if err != nil {
    panic(err)
  }
  var m map[string]any
  if err := json.Unmarshal(b, &m); err != nil {
    panic(err)
  }
  if err := log.RefreshConfig(flatten.Flatten(m)); err != nil {
    panic(err)
  }

  ctx := context.Background()

  // 简单格式化日志
  log.Infof(ctx, log.TagAppDef, "应用启动完成，版本: %s", "v1.0.0")
  log.Errorf(ctx, log.TagBizDef, "处理订单请求失败: %v", err)

  // 结构化日志
  log.Info(ctx, log.TagAppDef,
    log.String("event", "user_login"),
    log.Int("user_id", 10001),
    log.Msg("用户登录成功"),
  )
}
```

## 配置示例

Go-Spring Log 支持属性文件、JSON、YAML 等多种配置格式（由调用方解析成扁平属性 map 后传给 `RefreshConfig`，例如借助 `stdlib/flatten`）：

```properties
# 异步日志缓冲区大小
bufferSize=1000

# 定义文件输出器
appender.file.type=FileAppender
appender.file.dir=./logs
appender.file.file=app.log
appender.file.layout.type=JSONLayout

# 定义控制台输出器
appender.console.type=ConsoleAppender
appender.console.layout.type=TextLayout

# 根日志配置
logger.root.type=Logger
logger.root.level=INFO
logger.root.appenderRef.ref=console

# 给匹配的标签配置独立异步日志
logger.request.type=AsyncLogger
logger.request.level=DEBUG
logger.request.tag=_app_request_*,_rpc_*
logger.request.bufferSize=${bufferSize}
logger.request.onBufferFull=block
logger.request.appenderRef[0].ref=file
```

**配置说明**：
- `appender.xxx.type` - 输出器类型
- `logger.yyy.type` - 日志器类型
- `logger.yyy.level` - 日志级别范围，支持 `DEBUG`、`DEBUG~INFO` 格式
- `logger.yyy.tag` - 匹配的标签列表，支持后缀通配符
- `logger.yyy.appenderRef[n].ref` - 关联的输出器名称，可关联多个
- `logger.yyy.appenderRef[n].level` - 该输出器可选的级别过滤
- 支持 `${property}` 变量引用

### 常用配置项

各插件除 `type` 外的常用属性如下（未列出的即用默认值）：

**FileAppender / RollingFileAppender**

| 属性 | 默认值 | 说明 |
|------|--------|------|
| `dir` | `./logs` | 日志目录 |
| `file` | - | 日志文件名（必填） |
| `layout` | `TextLayout` | 格式化插件 |
| `interval` | `1h` | 滚动切割间隔（仅 Rolling，如 `24h`） |
| `maxAge` | `168h` | 旧日志最大保留时长，超期自动清理（仅 Rolling） |
| `syncLock` | `false` | 多 goroutine 写同一文件时建议开启（仅 Rolling） |

**Logger（SyncLogger / AsyncLogger）**

| 属性 | 默认值 | 说明 |
|------|--------|------|
| `level` | 全部 | 日志级别范围 |
| `tag` | `*` | 匹配的标签列表 |
| `bufferSize` | `10000` | 异步缓冲事件数，最小 100（仅 Async） |
| `onBufferFull` | `discard` | 缓冲区满策略：`block` / `discard` / `drop-oldest`（仅 Async） |

**TextLayout / JSONLayout**

| 属性 | 默认值 | 说明 |
|------|--------|------|
| `fileLineMaxLength` | `48` | 调用方"文件:行号"字段的最大展示宽度，超长截断 |

便利型 logger（`ConsoleLogger`、`FileLogger`、`RollingFileLogger`）内置输出器，直接在
logger 上配置 `dir` / `file` / `layout` 等同名字段即可；`RollingFileLogger` 额外支持
`separate`（WARN 及以上写入 `.wf` 文件）和 `async` + `bufferSize` + `onBufferFull`
（内置异步输出）。

## 内置插件

### Appender（输出器）

| 插件 | 说明 |
|------|------|
| `ConsoleAppender` | 输出到标准输出 |
| `FileAppender` | 输出到单个文件 |
| `RollingFileAppender` | 按时间间隔滚动切割文件，自动清理过期日志 |
| `DiscardAppender` | 丢弃所有日志 |

### Layout（格式化）

| 插件 | 说明 |
|------|------|
| `TextLayout` | 人类可读的纯文本格式 |
| `JSONLayout` | 结构化 JSON 格式 |

### Logger（处理器）

| 插件 | 说明 |
|------|------|
| `Logger` / `SyncLogger` | 同步日志处理器，在调用线程直接输出 |
| `AsyncLogger` | 异步日志处理器，后台线程处理输出，不阻塞业务。支持三种缓冲区满策略：`block`（阻塞等待）、`discard`（丢弃新事件）、`drop-oldest`（丢弃最旧事件） |
| `ConsoleLogger` | 快捷方式：直接输出到控制台的便利日志器 |
| `FileLogger` | 快捷方式：直接输出到文件的便利日志器 |
| `RollingFileLogger` | 快捷方式：时间滚动文件日志，支持错误日志分离 |
| `DiscardLogger` | 丢弃所有日志 |

**RollingFileLogger 特性**：
- 按指定时间间隔自动切割日志文件
- 自动清理超过最大保留天数的旧日志
- 支持 `separate=true` 将 WARN 及以上级别日志分离到独立的 `.wf` 文件，方便问题排查

## 性能对比

项目内置了与主流日志库（zap、logrus、zerolog、slog 等）的基准测试，本库在保持 API 简洁和扩展性的同时，性能表现优异。你可以执行以下命令查看对比结果：

```bash
go test -bench=. ./benchmarks/logs
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GS_LOGGER_DEFAULT_LEVEL` | `INFO` | 未配置任何 logger 时的默认级别，支持级别范围写法 |
| `GS_LOGGER_BUFFER_CAP` | `10KB` | 编码 buffer 回池上限，超过该大小的 buffer 直接丢弃 |
| `GS_LOGGER_CALLER_TYPE` | `fast` | 调用方信息采集模式：`fast` / `default` / `none` |

环境变量在进程启动时读取，运行期修改不生效；日志拓扑的运行期调整请使用 `RefreshConfig`。

## 自定义扩展

你可以通过实现 `Appender` / `Layout` 接口自定义输出目标与格式：

```go
type KafkaAppender struct {
	log.AppenderBase // 内置 Name / Layout 通用字段

	Topic string `PluginAttribute:"topic"`
	// ... 你的 Kafka producer
}

func (c *KafkaAppender) Start() error         { return producer.Connect() }
func (c *KafkaAppender) Stop()                { producer.Close() }
func (c *KafkaAppender) ConcurrentSafe() bool { return true }
func (c *KafkaAppender) Append(e *log.Event) {
	// 用 c.Layout 把事件编码后发送；不要在 Append 之外持有 Event
}

// 包 init 期注册，配置里即可使用 appender.kafka.type=KafkaAppender
func init() {
	log.RegisterPlugin[KafkaAppender]("KafkaAppender")
}
```

注意两点：

- `ConcurrentSafe` 返回 `false` 的 appender 只能挂在 `AsyncLogger` 下（由后台单线程
  串行写入）；声明为并发安全的 appender 也可用于同步 logger
- `Append` 不得修改或滞留 `Event` -- 它是池化对象，返回后随时被复用

## 架构设计

本库是 Go-Spring 的基础日志库，零业务依赖（仅依赖 `stdlib` 与标准库），被 `spring/` 与所有
`starter-*` 消费。设计目标：**可插拔、配置驱动、热路径单事件零分配**。

### 分层模型

一条日志从产生到落盘经过三层：

```
调用方                    配置（RefreshConfig）              输出
log.Info(ctx, tag, ...) → Tag → Logger → Layout → Appender
```

- **Tag**：调用侧唯一入口。业务代码只持有 `*Tag`，从不直接持有 `Logger`，因此配置刷新时
  框架可以原子地重新绑定 logger，调用方无感知。
- **Logger**：按 tag 匹配，独立设置级别与输出。
- **Layout / Appender**：格式化与输出目标，均通过 `RegisterPlugin` 注册，配置中的
  `type=JSONLayout` 等值即由此解析。

### 热更新机制

`RefreshConfig` 接收扁平属性 map，在内部完成解析后用原子指针整体替换全局 logger /
appender 集合：读端完全无锁，刷新瞬间不丢日志。同时 `Refresh` 会启动新插件、停掉被替换的
旧插件（实现 `Lifecycle` 即可参与）。配置文件的读取与解析由调用方完成，本库不绑定任何
配置格式。

### 性能设计

- **零分配字段构造**：主 API 接收 `func() []Field` 构造函数，级别被禁用的调用点不会为字段
  分配任何内存；`Infof` 等格式化版本更易用，但属于较慢路径。
- **对象池复用**：日志事件与编码 buffer 均来自 `sync.Pool`，超过 10 KB 的 buffer 不回池
  （可用环境变量 `GS_LOGGER_BUFFER_CAP` 调整）。
- **字段值类型**：`Field` 为值类型，`Bool`/`Int`/`String`/`Msg` 等基础 helper 构造字段时
  无需额外分配 slice。

### 使用约束

- Tag 只能在包 init 阶段注册：首次 `RefreshConfig` 之后再调 `RegisterTag` 会 panic，
  这是原子替换契约的一部分。
- Tag 命名规则：3–36 字符，仅小写字母 / 数字 / 下划线，1–4 段；`RegisterAppTag` /
  `RegisterBizTag` / `RegisterRPCTag` 会自动生成符合规范的名称。
- 自定义 `Appender` 中不要在 `EncodeTo` 之外持有 `Field.Any` 切片或 `*bytes.Buffer`——
  它们是池化对象，随时会被复用。
- 本库只内置 console / file / rolling-file 三种 sink。Kafka、Loki 等远端输出请在各自
  starter 中实现 `Appender` 并注册，以保持本库零依赖。

## License

Go-Spring Log 基于 [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0) 开源。
