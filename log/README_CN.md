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
- **动态配置重载**：支持运行时从外部配置文件重新加载日志配置，无需重启应用
- **完善测试覆盖**：核心模块均有单元测试覆盖，保证稳定可靠

## 核心概念

### Tag（标签）

Tag 是本日志库的核心概念，用于对日志进行分类。通过 `RegisterTag` 注册标签后，配置中可以使用后缀通配符进行分组匹配（例如 `_app_request_*` 匹配所有 `_app_request_` 开头的标签）。

这种设计使得无需显式创建 logger 实例即可使用统一 API 打日志，即使第三方库也能以标准化方式输出日志。框架会根据标签自动匹配到最具体的 logger。

```go
// 按分类便捷注册标签
var (
  TagAppStartup     = log.RegisterAppTag("startup", "init")     // 应用启动阶段
  TagBizOrderCreate = log.RegisterBizTag("order", "create")   // 订单创建业务
  TagRpcRedisQuery  = log.RegisterRPCTag("redis", "query")    // Redis 查询 RPC
)
```

### Logger（日志处理器）

Logger 是实际处理日志的组件。不同标签可以匹配到不同 logger，每个 logger 可以独立设置级别和输出。

### 上下文字段抽取

你可以配置钩子函数从 `context.Context` 中抽取上下文数据并自动加入每条日志：

- `log.StringFromContext`：从 context 抽取字符串（如 request ID）
- `log.FieldsFromContext`：从 context 返回结构化字段列表（如 trace ID、span ID）

## 安装

```bash
go get github.com/go-spring/log
```

## 快速开始

```go
package main

import (
  "context"

  "github.com/go-spring/log"
)

func main() {
  // 配置从 context 抽取链路字段
  log.FieldsFromContext = func(ctx context.Context) []log.Field {
    return []log.Field{
      log.String("trace_id", "0a882193682db71edd48044db54cae88"),
      log.String("span_id", "50ef0724418c0a66"),
    }
  }

  // 从配置文件加载日志配置
  err := log.RefreshFile("log.properties")
  if err != nil {
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

Go-Spring Log 支持属性文件、JSON、YAML 等多种配置格式：

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
logger.request.tag=_app_request*,_rpc_*
logger.request.bufferSize=${bufferSize}
logger.request.onBufferFull=block
logger.request.appenderRef[0].ref=file
```

**配置说明**：
- `appender.xxx.type` - 输出器类型
- `logger.yyy.type` - 日志器类型
- `logger.yyy.level` - 日志级别范围，支持 `DEBUG`、`DEBUG~INFO` 格式
- `logger.yyy.tag` - 匹配的标签列表，支持后缀通配符
- 支持 `${property}` 变量引用

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

## 自定义扩展

你可以通过实现以下接口自定义扩展：

- `Appender` 接口：自定义输出目标
- `Layout` 接口：自定义输出格式
- 实现后通过 `RegisterPlugin` 注册插件，即可在配置中使用

## License

Go-Spring Log 基于 [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0) 开源。
