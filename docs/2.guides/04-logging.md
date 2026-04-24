# 日志

Go-Spring 提供了一个**高性能、可扩展、标签路由**的结构化日志库。

## 日志模型

### 架构模型

Go-Spring 日志架构遵循 log4j2 的经典架构，但做了重要创新：

- **非树形结构**：不基于 logger name 的层级继承
- **Tag 路由**：通过标签打破传统继承规则，按业务语义路由
- **路由规则**：精确匹配 > 前缀匹配 > 默认 root logger
- 默认存在 root logger，其他 logger 由用户显式配置

### Logger

Logger 是日志路由层，负责将日志分发给关联的 appender。

**同步 Logger**：
- 在调用线程直接输出到 appender
- 日志顺序保证，适合低吞吐量场景

**异步 Logger**：
- 内部维护缓冲队列，后台线程消费
- 不阻塞业务主线程，适合高并发场景
- 队列满时支持三种策略：
  - `block` - 阻塞等待空间
  - `discard` - 丢弃新事件
  - `drop-oldest` - 丢弃最旧事件腾出空间

**集成 Logger（预制封装）**：开箱即用，内置 appender：

| Logger | 说明 |
|--------|------|
| `ConsoleLogger` | 输出到控制台，内置 `ConsoleAppender` |
| `FileLogger` | 输出到单个文件，内置 `FileAppender` |
| `RollingFileLogger` | 时间滚动文件，支持错误日志分离，内置 `RollingFileAppender` |
| `DiscardLogger` | 丢弃所有日志 |

你也可以自定义 Logger 实现。

**Appender 约束**：
- 每个 logger 至少关联一个 appender
- 多个 appender 按配置顺序依次执行
- appender 自行处理写入错误，logger 不感知错误状态

### Appender

Appender 是日志最终存储层，可对接多种存储系统。

**内置 Appender**：

| Appender | 说明 |
|----------|------|
| `ConsoleAppender` | 输出到标准输出 |
| `FileAppender` | 输出到单个文件，不支持轮转 |
| `RollingFileAppender` | 按时间间隔滚动切割，自动清理过期日志 |
| `DiscardAppender` | 忽略所有日志 |

你也可以自定义 Appender 实现。

**写入失败处理**：只通过 `ReportError` 钩子上报错误（用于监控告警），不递归写入错误日志，避免死循环或阻塞。

### Layout

Layout 负责日志内容格式化。

**内置 Layout**：

| Layout | 说明 |
|--------|------|
| `TextLayout` | 人类可读的纯文本格式，默认使用 `||` 分隔字段 |
| `JSONLayout` | 结构化 JSON 格式 |

你也可以自定义 Layout 实现。

### AppenderRef

`AppenderRef` 是 logger 与 appender 之间的连接组件，描述 logger 使用哪些 appender，还可以对 appender 做级别过滤。

### Level

日志级别定义（数值越小优先级越低）：

| 级别 | 数值 | 说明 |
|------|------|------|
| `NONE` | 0 | 关闭日志 |
| `TRACE` | 100 | 最详细的开发调试信息 |
| `DEBUG` | 200 | 开发调试信息 |
| `INFO` | 300 | 一般应用进程信息 |
| `WARN` | 400 | 警告，潜在问题 |
| `ERROR` | 500 | 错误，不影响应用继续运行 |
| `PANIC` | 600 | 严重错误，仅表示日志级别，框架不触发 panic |
| `FATAL` | 700 | 致命错误，仅表示日志级别，框架不触发进程退出 |
| `MAX` | 999 | 上限，用于范围比较 |

支持自定义日志级别。

### Tag（核心创新）

Tag 是 Go-Spring 日志的核心创新：

- **基于 tag 路由**：所有日志输出都通过 tag，实现统一 API
- 示例：`log.Infof(ctx, tag, "message")`
- **路由规则**：精确匹配优先，前缀匹配次之
- 与传统 logger name 的区别：
  - tag 按业务语义建模，可以全局定义，甚至可以定义在第三方包中
  - logger name 通常基于代码包路径，缺乏业务语义
  - 解决了基于 logger name 路由粒度过粗的问题

**便捷注册**：
```go
// 应用层标签
var TagAppStartup = log.RegisterAppTag("startup", "init")

// 业务层标签
var TagBizOrderCreate = log.RegisterBizTag("order", "create")

// RPC 标签
var TagRpcRedisQuery = log.RegisterRPCTag("redis", "query")
```

### 可观测性支持

Go-Spring 天然支持链路追踪，可以从 `context.Context` 中自动提取可观测信息：

```go
// 提取单字符串（如 request ID）
log.StringFromContext = func(ctx context.Context) string {
	return traceID(ctx)
}

// 提取多个结构化字段（如 trace ID、span ID、user ID）
log.FieldsFromContext = func(ctx context.Context) []log.Field {
	return []log.Field{
		log.String("trace_id", traceID(ctx)),
		log.String("span_id", spanID(ctx)),
	}
}
```

提取的信息会自动附加到每条日志。

## 结构化日志

Go-Spring 提供完整的结构化日志支持，所有日志内容都以 `Field` 形式表达。

### 内置 Field 类型

| 类型 | 构造函数 | 说明 |
|------|----------|------|
| 消息 | `Msg(msg)` / `Msgf(format, args)` | key 固定为 `msg` |
| 空值 | `Nil(key)` |  |
| 布尔 | `Bool(key, val)` / `BoolPtr(key, ptr)` / `Bools(key, []bool)` |  |
| 整数 | `Int(key, val)` / `IntPtr(key, ptr)` / `Ints(key, []int)` | 支持所有整数类型 |
| 无符号整数 | `Uint(key, val)` / `UintPtr(key, ptr)` / `Uints(key, []uint)` | 支持所有无符号整数类型 |
| 浮点数 | `Float(key, val)` / `FloatPtr(key, ptr)` / `Floats(key, []float)` |  |
| 字符串 | `String(key, val)` / `StringPtr(key, ptr)` / `Strings(key, []string)` |  |
| 数组 | `Array(key, ArrayValue)` | 自定义数组编码 |
| 对象 | `Object(key, ...Field)` | 嵌套对象 |
| Map 转换 | `FieldsFromMap(map[string]any)` | 从 map 展开多个字段 |
| 任意类型 | `Any(key, val)` / `Reflect(key, val)` | 自动检测类型转换 |

### Encoder

Encoder 将 Fields 编码为最终输出格式：

- 内置 `JSONEncoder` 和 `TextEncoder`
- 支持自定义 Encoder

## 使用示例

```go
package main

import (
	"context"

	"github.com/go-spring/log"
)

// 注册标签
var (
	TagAppInit   = log.RegisterAppTag("init", "startup")
	TagBizLogin  = log.RegisterBizTag("user", "login")
)

func main() {
	// 配置上下文抽取
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		return []log.Field{
			log.String("trace_id", "trace-123"),
		}
	}

	// 加载配置
	if err := log.RefreshFile("log.properties"); err != nil {
		panic(err)
	}

	ctx := context.Background()

	// 格式化日志
	log.Infof(ctx, TagAppInit, "应用启动，版本: %s", "v1.0.0")

	// 结构化日志
	log.Info(ctx, TagBizLogin,
		log.String("user", "alice"),
		log.Int("user_id", 10001),
		log.Msg("用户登录成功"),
	)
}
```

## 日志配置

Go-Spring 使用 KV 配置模型：

```properties
# 控制台输出
appender.console.type=ConsoleAppender
appender.console.layout.type=TextLayout

# 文件输出
appender.file.type=RollingFileAppender
appender.file.dir=./logs
appender.file.file=app.log
appender.file.interval=24h
appender.file.maxAge=168h
appender.file.layout.type=JSONLayout

# 根 logger
logger.root.type=Logger
logger.root.level=INFO
logger.root.appenderRef.ref=console

# 为匹配标签配置独立异步日志
logger.request.type=AsyncLogger
logger.request.level=DEBUG
logger.request.tag=_app_request*,_rpc_*
logger.request.bufferSize=10000
logger.request.onBufferFull=discard
logger.request.appenderRef[0].ref=file
```

**插件化设计**：
- 基于依赖注入模型
- 支持 `element` 注入（如给 appender 注入 layout）
- 支持 `property` 注入（如给 async logger 注入 bufferSize）
- 插件自注册，自定义插件只需要实现 + 注册即可

**生命周期管理**：
- `Start()` - 启动组件（打开文件等）
- `Stop()` - 停止组件（关闭文件，flush 缓冲）

**启动期校验**：配置校验失败则启动失败，早发现早处理。

> 当前版本暂不支持日志配置热更新。

## 与其他日志库适配

Go-Spring 提供获取指定名称 logger 的能力，用于兼容第三方日志库或项目迁移。
