# 日志

Go-Spring 提供了一套高性能、可扩展、基于标签路由的结构化日志系统。
它将日志分类、输出策略、格式化和上下文提取拆分为独立能力，既能满足高并发场景的性能要求，也方便在不同环境中统一配置和调整。

---

## 快速开始

下面是一个完整示例，展示如何定义标签、加载配置并输出日志：

```go
package main

import (
    "context"
    "os"

    "github.com/go-spring/log"
)

// 1. 定义标签：推荐在包级变量中注册，作为日志分类标识。
var (
    TagAppStartup  = log.RegisterAppTag("startup", "")     // 应用启动
    TagAppShutdown = log.RegisterAppTag("shutdown", "")    // 应用关闭
    TagBizOrder    = log.RegisterBizTag("order", "create") // 创建订单
    TagBizUser     = log.RegisterBizTag("user", "login")   // 用户登录
)

func main() {
    // 2. 配置日志：输出 INFO 及以上级别到控制台。
    config := map[string]string{
        "logger.root.type": "ConsoleLogger",
        "logger.root.level": "INFO",
    }

    if err := log.RefreshConfig(config); err != nil {
        panic("日志配置失败: " + err.Error())
    }

    ctx := context.Background()

    // 3. 输出日志：支持格式化日志和结构化日志。

    // 格式化日志
    log.Infof(ctx, TagAppStartup, "应用启动成功，版本: %s，PID: %d", "v1.0.0", os.Getpid())

    // 结构化日志（推荐生产环境使用）
    log.Info(ctx, TagBizOrder,
        log.Int64("order_id", 10001),
        log.String("user", "alice"),
        log.Float64("amount", 99.99),
        log.Bool("success", true),
        log.Strings("tags", []string{"vip", "new_user"}),
        log.Msg("订单创建成功"),
    )

    // 不同级别日志
    log.Debug(ctx, TagBizUser, log.String("trace", "user_login_flow"), log.Msg("用户登录流程开始"))
    log.Warnf(ctx, TagBizUser, "用户 %s 密码错误尝试 %d 次", "bob", 3)
    log.Errorf(ctx, TagBizOrder, "订单 %d 创建失败: %s", 10002, "库存不足")
}
```

---

## 架构概览

### 三大设计原则

Go-Spring 日志系统围绕三个原则设计：降低业务代码侵入、用语义标签路由日志、控制高并发场景下的日志开销。

#### 零侵入原则

业务代码只需要声明“这是什么类型的日志”，不需要关心 Logger 如何创建、输出到哪里、使用什么格式或保留哪些级别。

在传统日志库中，业务代码通常需要显式创建或传递 Logger。随着包和模块增多，Logger 的管理会变得分散，日志级别、输出位置和文件路径也容易混入业务逻辑。Go-Spring 通过标签机制把业务埋点和日志策略解耦：代码里只使用 tag 标识日志语义，具体处理方式由配置决定。

这样可以在不修改业务代码的前提下调整日志策略，例如把订单日志单独写入文件、把错误日志发送到告警系统，或者在排查问题时临时提高某类日志的级别。

#### 标签路由原则

标签路由用语义维度替代包名层级。

Log4j、Logback 等日志库常见的路由方式是基于包名继承，例如 `com.xxx.order.service` 继承 `com.xxx.order`，再继承 `com.xxx`。这种方式适合按代码目录管理日志，但不适合表达业务语义。订单相关日志可能分散在 controller、service、dao，甚至横跨订单、库存、支付等服务，仅靠包名很难统一路由。

Go-Spring 使用标签表达日志语义。标签不是层级，而是可组合的分类维度：可以按业务域切分所有订单日志，按技术域切分所有 Redis 调用日志，也可以把审计日志单独发送到 Kafka。日志路由不再受包名结构限制。

#### 高性能原则

高并发场景下，日志很容易成为性能瓶颈。一条日志从调用到落盘，可能涉及时间戳、调用栈、字段编码、内存分配、锁竞争和磁盘 I/O。Go-Spring 从这些环节分别优化：

- 通过对象池复用 Event 和 Buffer，降低 GC 压力。
- 通过异步缓冲解耦业务 goroutine 和写入 goroutine，减少请求阻塞。
- 通过原子指针刷新 Logger 绑定，保证配置切换时并发安全。
- 通过专用 JSON 编码器减少反射和中间对象分配。

日志库的目标是在保留详细调试信息的同时，把运行时开销控制在可接受范围内。

### 核心组件分层

```
┌───────────────────────────────────────────────────────────┐
│                     应用层 API                             │
│  Trace/Info/Warn/Error/Fatal · Record · GetLogger         │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                        标签层 (Tag)                        │
│  标签注册  →  标签查找  →  Logger 绑定  →  路由匹配        │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                      Logger 层                             │
│  SyncLogger  │  AsyncLogger  │  ConsoleLogger  │  Rolling │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                     Appender 层                           │
│  ConsoleAppender  │  FileAppender  │  RollingFileAppender │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                      Layout 层                            │
│  TextLayout (人类可读)  │  JSONLayout (结构化)            │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                      Encoder 层                           │
│  Field 编码  →  类型编码  →  数组/对象编码                │
└───────────────────────────────────────────────────────────┘
```

每一层都有独立职责，可以单独替换和扩展。例如，新增 Kafka 输出只需要实现 Appender；新增 Protobuf 等编码格式只需要实现 Layout，不需要改动其他层。

---

## 标签系统 (Tag)

标签是 Go-Spring 日志路由的核心。不同于基于包名层级继承的传统方式，Go-Spring 使用语义化标签描述日志类型。

标签本质上是日志的静态元数据，在代码编写时就已经确定。它用于回答“这条日志属于什么类别”，而不是描述某次请求的动态信息。

**为什么标签必须预先注册？**

预先注册标签主要有三点价值：

1. **编译期检查**：注册后的标签是强类型变量，编译器可以帮助发现拼写错误。如果直接使用字符串，`"_biz_order_create"` 写成 `"_biz_order_craete"`，通常要到排查日志时才会发现。
2. **可发现性**：所有注册的标签都可以通过 `GetAllTags()` 获取。这意味着可以编写工具来自动生成文档、检查配置文件中引用的标签是否存在、分析哪些标签没有被任何 Logger 绑定。
3. **原子绑定**：标签与 Logger 的绑定是通过 `atomic.Pointer` 实现的无锁原子操作。这保证了配置刷新时，日志输出不会出现中间状态。

在大型项目中，类型安全和可维护性通常比省略几行初始化代码更重要。

### 标签命名规范

标签必须符合以下规则：

1. **长度**：3-36 个字符
2. **字符集**：小写字母、数字、下划线
3. **格式**：`_<分类>_<子类型>_<动作>`
4. **分段**：下划线分隔，1-4 个分段

**为什么要限制命名格式？**

命名规范用于保证标签的一致性和可搜索性。在大型团队中，如果每个人都按自己的习惯命名标签，容易出现同一含义多种写法、无法按字符串归类检索、新成员难以理解等问题。

**格式约定说明**：

1. **下划线开头**：用于在视觉上区分标签和普通字符串。
2. **三段式结构**：`_<分类>_<子类型>_<动作>` 中，分类表示日志大类，子类型表示对象或领域，动作表示发生的操作。
3. **小写字母**：避免大小写不一致导致的匹配错误。
4. **长度限制**：避免标签过长影响性能和可读性。

**标准分类前缀**：

| 前缀 | 说明 | 示例 |
|------|------|------|
| `_app_` | 应用生命周期相关 | `_app_startup`、`_app_shutdown`、`_app_health` |
| `_biz_` | 业务逻辑相关 | `_biz_order_create`、`_biz_user_login`、`_biz_pay_success` |
| `_rpc_` | 远程调用相关 | `_rpc_redis_get`、`_rpc_http_request`、`_rpc_mysql_query` |
| `_db_` | 数据库操作 | `_db_mysql_insert`、`_db_transaction_begin`、`_db_connection_pool` |
| `_mq_` | 消息队列 | `_mq_kafka_produce`、`_mq_rocket_consume`、`_mq_retry` |
| `_cache_` | 缓存相关 | `_cache_hit`、`_cache_miss`、`_cache_invalidate` |

这些分类不是技术限制，而是推荐约定。统一约定可以降低理解成本，也方便后续按标签生成文档、检查配置和分析日志覆盖情况。

### 标签注册 API

Go-Spring 提供了多个层级的标签注册函数，适应不同的分类需求：

```go
// 应用层标签（子类型，无动作）
log.RegisterAppTag("startup", "")
log.RegisterAppTag("shutdown", "")
log.RegisterAppTag("health", "")

// 应用层标签（子类型 + 动作）
log.RegisterAppTag("config", "reload")
log.RegisterAppTag("module", "init")

// 业务层标签
log.RegisterBizTag("order", "create")
log.RegisterBizTag("order", "cancel")
log.RegisterBizTag("order", "query")
log.RegisterBizTag("user", "login")
log.RegisterBizTag("user", "register")

// RPC 层标签
log.RegisterRPCTag("redis", "get")
log.RegisterRPCTag("redis", "set")
log.RegisterRPCTag("http", "call")
log.RegisterRPCTag("grpc", "invoke")
log.RegisterRPCTag("mysql", "query")

// 通用注册（自定义分类）
log.RegisterTag("_cache_hit")
log.RegisterTag("_cache_miss")
log.RegisterTag("_mq_kafka_produce")
```

### 标签路由算法

日志输出时，系统会按照以下优先级查找对应的 Logger：

```
精确匹配 → 前缀匹配 → root Logger
```

**匹配过程**：

假设日志标签是 `_biz_order_create`，系统会按以下顺序尝试匹配：

1. **精确匹配**：查找是否有 Logger 的 `tag` 配置包含 `_biz_order_create`
2. **一级前缀匹配**：去掉最后一段，查找 `_biz_order_*`
3. **二级前缀匹配**：再去掉一段，查找 `_biz_*`
4. **兜底**：如果都没有匹配到，使用 `logger.root`

**配置示例**：

```properties
# 精确匹配：只处理 _biz_order_create 标签
logger.order_create.type = AsyncLogger
logger.order_create.tag = _biz_order_create
logger.order_create.level = DEBUG
logger.order_create.appenderRef.ref = order_file

# 前缀匹配：处理所有 _biz_order_ 开头的标签
logger.order_all.type = AsyncLogger
logger.order_all.tag = _biz_order_*
logger.order_all.level = INFO
logger.order_all.appenderRef.ref = biz_file

# 前缀匹配：处理所有 _biz_ 开头的标签
logger.biz_all.type = SyncLogger
logger.biz_all.tag = _biz_*
logger.biz_all.level = WARN
logger.biz_all.appenderRef.ref = error_file

# root logger：处理所有未匹配的标签
logger.root.type = ConsoleLogger
logger.root.level = INFO
```

**匹配优先级示例**：

| 日志标签 | 匹配到的 Logger |
|---------|----------------|
| `_biz_order_create` | logger.order_create（精确匹配优先） |
| `_biz_order_cancel` | logger.order_all（匹配 `_biz_order_*`） |
| `_biz_user_login` | logger.biz_all（匹配 `_biz_*`） |
| `_rpc_redis_get` | logger.root（都不匹配，兜底） |

### 标签与上下文

标签和上下文提取是两个互补的机制：

- **标签**描述的是"这是什么类型的日志"，是静态的，在代码编写时就确定了
- **上下文提取**描述的是"这条日志是在哪个请求/链路中产生的"，是动态的，每次请求都不一样

这两个机制配合后，可以从两个维度查询和分析日志：

- 按标签类型聚合：统计所有订单创建的日志
- 按请求链路聚合：追踪某一个 trace_id 下的所有日志

标签维度适合聚合分析，上下文维度适合链路追踪。两者组合后，既能看到某类业务行为的整体情况，也能定位某次请求的完整路径。

---

## 日志级别

日志级别用于控制一条日志是否输出。Go-Spring 使用常见的级别命名，并为每个级别分配数值，方便进行范围过滤和自定义扩展。

| 级别 | 数值 | 说明 | 典型使用场景 |
|------|------|------|-------------|
| NONE | 0 | 关闭日志 | 性能压测时临时关闭 |
| TRACE | 100 | 最详细调试信息 | 方法入参出参、逐行跟踪、循环内部 |
| DEBUG | 200 | 调试信息 | 模块初始化状态、配置详情、分支判断 |
| INFO | 300 | 一般信息 | 服务启动、请求完成、任务执行、重要状态变更 |
| WARN | 400 | 潜在问题 | 重试、超时、降级、非预期但可恢复的情况 |
| ERROR | 500 | 错误但不影响运行 | 单个请求失败、数据库操作异常、第三方调用出错 |
| PANIC | 600 | 严重错误 | 可能触发 panic 的错误、系统级异常 |
| FATAL | 700 | 致命错误 | 进程即将退出前的最后记录、不可恢复的系统错误 |
| MAX | 999 | 上限标记 | 用于范围比较，不直接使用 |

**级别数值说明**：

相邻标准级别之间相差 100（TRACE=100, DEBUG=200, INFO=300...），用于给自定义级别预留空间。例如，可以在 INFO 和 WARN 之间注册一个 `AUDIT` 级别，code 为 350。这样既能插入到正确顺序，也不会破坏标准级别比较逻辑。

Go-Spring 使用**左闭右开**区间 `[MinLevel, MaxLevel)` 来控制输出范围。

**级别范围配置示例**：

```properties
# 输出 INFO 及以上（INFO, WARN, ERROR, PANIC, FATAL）
logger.root.level = INFO

# 只输出 WARN 和 ERROR
logger.error_only.level = WARN~FATAL

# 只输出 DEBUG 和 INFO
logger.debug_info.level = DEBUG~WARN

# 自定义范围：只输出 INFO 和 WARN
logger.info_warn.level = INFO~ERROR
```

## 结构化日志 (Field)

结构化日志把日志内容表示为有类型的字段，而不是先拼接成普通字符串再由日志平台解析。

传统格式化日志通常写成：

```go
log.Printf("user %d login from %s, success: %v", userID, ip, success)
```

这种方式便于阅读，但不利于机器解析。即使用 `user_id=%d ip=%s` 这类约定格式，本质上仍然是字符串，后续检索和统计需要依赖正则或日志平台的二次解析。

Go-Spring 推荐直接输出键值字段：

```go
log.Info(ctx, tag,
    log.Int("user_id", userID),
    log.String("ip", ip),
    log.Bool("success", success),
)
```

这样写既清晰，又便于日志系统直接解析字段，还能保留类型信息并支持嵌套结构。

### 基础类型

```go
// 布尔
log.Bool("success", true)
log.Bool("retry", false)

// 整数（支持 int/8/16/32/64）
log.Int("user_id", 10001)
log.Int8("age", 25)
log.Int16("port", 8080)
log.Int32("code", 200)
log.Int64("timestamp", time.Now().Unix())
log.Int64("duration_us", duration.Microseconds())

// 无符号整数
log.Uint("count", uint(100))
log.Uint32("ip_addr", 0xC0A80001)
log.Uint64("bytes_transferred", 1024*1024*1024)

// 浮点数
log.Float32("cpu_usage", 45.5)
log.Float64("amount", 99.99)
log.Float64("latency_ms", 123.45)

// 字符串
log.String("order_no", "ORD202401010001")
log.String("ip", "192.168.1.1")
log.String("user_agent", "Mozilla/5.0 ...")

// nil 值
log.Nil("deleted_at")
log.Nil("error")
```

### 指针类型

对于可能为 nil 的值，使用指针类型的构造函数可以避免额外的判断代码：

```go
// 布尔指针
var enabled *bool
// ... 可能设置 enabled，也可能不设置 ...
log.BoolPtr("enabled", enabled) // nil 时输出 null

// 整数指针
var userID *int64
log.Int64Ptr("user_id", userID)

// 字符串指针
var remark *string
log.StringPtr("remark", remark)
```

这适合处理数据库查询结果、可选配置字段等可能为空的值。

### 消息字段

`Msg` 和 `Msgf` 用于输出人类可读的消息：

```go
// 简单消息
log.Msg("订单创建成功")
log.Msg("数据库连接池初始化完成")
log.Msg("定时任务执行完毕")

// 格式化消息
log.Msgf("订单 %s 创建成功，金额 %.2f，用户 %d", orderNo, amount, userID)
log.Msgf("处理了 %d 条记录，成功 %d，失败 %d", total, success, failed)
log.Msgf("启动耗时 %d ms，加载配置 %d ms，初始化组件 %d ms",
    totalMs, configMs, initMs)
```

这两个函数创建的 Field Key 固定为 `msg`，与 `log.MsgKey` 常量对应。

### 数组和嵌套对象

```go
// 整数数组
log.Ints("item_ids", []int{1, 2, 3, 4, 5})
log.Int64s("user_ids", []int64{10001, 10002, 10003})

// 字符串数组
log.Strings("tags", []string{"vip", "new_user", "first_order"})
log.Strings("skus", []string{"SKU001", "SKU002", "SKU003"})

// 布尔数组
log.Bools("flags", []bool{true, false, true, true})

// 浮点数数组
log.Float64s("prices", []float64{9.99, 19.99, 29.99})

// 嵌套对象
log.Object("order",
    log.String("order_no", "ORD001"),
    log.Int64("user_id", 10001),
    log.Float64("amount", 99.99),
    log.Bool("paid", true),
    log.Object("items",
        log.String("sku", "ITEM001"),
        log.Int("quantity", 2),
        log.Float64("price", 49.99),
    ),
    log.Object("shipping",
        log.String("address", "北京市朝阳区..."),
        log.String("phone", "138****0001"),
    ),
)

// 数组 + 对象组合
log.Object("result",
    log.Int("total", 100),
    log.Int("success", 98),
    log.Int("failed", 2),
    log.Object("items",
        log.Ints("success_ids", []int{1, 2, 3}),
        log.Ints("failed_ids", []int{99, 100}),
    ),
)
```

### Map 展开

`FieldsFromMap` 可以将一个 `map[string]any` 展开为多个独立字段：

```go
data := map[string]any{
    "order_id": "ORD001",
    "amount":   99.99,
    "user_id":  int64(10001),
    "success":  true,
    "tags":     []string{"vip", "new"},
}

// 输出时会展开为多个独立字段，而不是一个 map 对象
log.Info(ctx, tag, log.FieldsFromMap(data))
```

这适合以下场景：
- 动态生成的日志字段
- 从配置文件读取的属性
- 桥接其他日志系统时的数据转换

### 自动类型推断

`Any` 函数会自动检测值的类型，并选择合适的编码方式：

```go
log.Any("order_id", "ORD001")    // 自动识别为字符串
log.Any("amount", 99.99)         // 自动识别为浮点数
log.Any("user_id", int64(10001)) // 自动识别为整数
log.Any("success", true)          // 自动识别为布尔值
log.Any("tags", []string{"a", "b"}) // 自动识别为字符串数组
```

如果类型无法识别，会回退到反射编码。`Any` 使用方便，但在性能敏感路径上推荐优先使用强类型构造函数，因为它们更快，也更明确。

---

## Logger 详解

### Logger 接口

所有 Logger 都实现了统一的接口：

```go
type Logger interface {
    Lifecycle             // 生命周期管理：Start / Stop
    GetName() string      // 获取 Logger 名称
    GetTags() []string    // 获取绑定的标签列表
    GetLevel() LevelRange // 获取日志级别范围
    Append(e *Event)      // 处理日志事件
}
```

### SyncLogger（同步）

在调用 goroutine 中直接写入，阻塞直到写入完成。

**适用场景**：
- 低吞吐量应用，QPS 不高
- 对日志可靠性要求极高，不能丢失任何日志
- 开发环境，调试时希望看到实时输出
- 日志量小，性能不是问题

**配置示例**：

```properties
logger.console.type = SyncLogger
logger.console.level = INFO
logger.console.appenderRef[0].ref = console
logger.console.appenderRef[1].ref = file
```

**优点**：
- 实现简单，容易调试
- 日志不会丢失
- 日志顺序严格保证

**缺点**：
- 阻塞业务 goroutine
- 写入慢时影响业务性能
- Appender 需要是并发安全的

### AsyncLogger（异步）

日志先写入内存缓冲，后台 goroutine 异步写入。不阻塞业务，适合高并发生产环境。

**配置示例**：

```properties
logger.async.type = AsyncLogger
logger.async.level = DEBUG
logger.async.tag = _biz_*
logger.async.bufferSize = 10000
logger.async.onBufferFull = discard
logger.async.appenderRef[0].ref = file
logger.async.appenderRef[1].ref = monitor
```

**缓冲满策略**：

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `block` | 阻塞直到有空间 | 日志不能丢，宁愿影响业务 |
| `discard` | 丢弃新日志（默认） | 保证业务性能优先，日志丢失可以接受 |
| `drop-oldest` | 丢弃最旧的，腾出空间 | 保留最新日志更重要 |

**优点**：
- 不阻塞业务，性能好
- 平滑应对流量毛刺
- Appender 不需要并发安全
- 可以单独控制每个 Logger 的异步性

**缺点**：
- 极端情况下（kill -9）可能丢失缓冲中的日志
- 内存占用略高
- 需要正确配置缓冲大小和策略
- 实现相对复杂

### 集成式 Logger

ConsoleLogger、FileLogger、RollingFileLogger 这类“集成式 Logger”本质上是预置配置，内部仍由 SyncLogger/AsyncLogger 与 Appender 组合而成。它们用于简化常见场景的配置。

#### ConsoleLogger

直接输出到标准输出，内置 ConsoleAppender。

```properties
# 最简配置
logger.console.type = ConsoleLogger
logger.console.level = INFO

# 带自定义 Layout
logger.console.type = ConsoleLogger
logger.console.level = DEBUG
logger.console.layout.type = TextLayout
logger.console.layout.fileLineMaxLength = 64
```

适用场景：开发环境、本地调试、容器化应用输出到 stdout。

#### FileLogger

输出到单个文件，内置 FileAppender。

```properties
logger.file.type = FileLogger
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
```

适用场景：简单应用、日志轮转由外部工具（如 logrotate）管理。

#### RollingFileLogger

按时间滚动切割的文件 Logger，适合生产环境使用。

```properties
logger.prod.type = RollingFileLogger
logger.prod.level = INFO
logger.prod.tag = *
logger.prod.dir = /var/log/app
logger.prod.file = app.log
logger.prod.separate = true         # WARN+ 单独输出到 app.log.wf
logger.prod.interval = 24h          # 每天滚动一次
logger.prod.maxAge = 168h           # 保留最近 7 天的日志
logger.prod.async = true            # 启用异步写入
logger.prod.bufferSize = 20000      # 异步缓冲大小
logger.prod.onBufferFull = drop-oldest # 缓冲满时丢弃最旧的
logger.prod.layout.type = JSONLayout
```

##### 分离模式（separate）

开启分离模式后，RollingFileLogger 会创建两个独立的 RollingFileAppender：

- 正常日志文件：`app.log.YYYYMMDDHH`，级别范围 `[MinLevel, WARN)`
- 错误日志文件：`app.log.wf.YYYYMMDDHH`，级别范围 `[WARN, MaxLevel)`

分离模式适合日常运维排障。警告和错误会进入独立的 `.wf` 文件，查看异常日志时不需要再从大量 INFO 日志中过滤。

##### 异步模式（async）

RollingFileLogger 内置支持异步写入，无需额外配置 AsyncLogger。设置 `async = true` 后，日志会先写入内存缓冲，由后台 goroutine 异步写入文件，不阻塞业务线程。

- `bufferSize`：缓冲大小，默认 10000，高并发场景建议调大
- `onBufferFull`：缓冲满策略，支持 `block`/`discard`/`drop-oldest`，生产环境推荐 `drop-oldest`

生产环境通常建议开启异步模式，以减少日志写入对业务请求的阻塞。

---

## Appender 详解

### Appender 接口

```go
type Appender interface {
    Lifecycle             // Start / Stop
    GetName() string      // 获取 Appender 名称
    Append(e *Event)      // 写入日志事件
    ConcurrentSafe() bool // 是否并发安全
}
```

#### 并发安全检查

Appender 接口有一个 `ConcurrentSafe()` 方法，返回该 Appender 是否可以被多个 goroutine 同时调用。

**关键设计**：SyncLogger 绑定的 Appender 必须是并发安全的。如果绑定了非并发安全的 Appender，配置加载时会直接报错，避免运行时出现数据竞争。

数据竞争通常难以复现，也不一定有清晰的堆栈信息。Go-Spring 在配置加载阶段完成检查，让配置错误尽早暴露，降低排查成本。

### DiscardAppender

丢弃所有日志，主要用于性能压测和测试场景。

```properties
appender.dev_null.type = DiscardAppender
```

### ConsoleAppender

输出到标准输出 `os.Stdout`，始终并发安全。

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout
```

**注意**：ConsoleAppender 直接写入 `os.Stdout`，没有额外缓冲。高并发场景下，大量控制台输出可能成为性能瓶颈。生产环境建议使用文件输出或异步 Logger。

### FileAppender

输出到单个文件，支持多个 Appender 写入同一文件。

**文件引用计数机制**：

多个 Appender 写入同一个文件时，底层的文件句柄是共享的，通过引用计数管理：

1. 第一个打开文件的 Appender 使引用计数 = 1
2. 后续打开同一文件时，复用已有句柄，计数 +1
3. 每个 Appender 关闭时，计数 -1
4. 计数归零时，真正关闭文件

该机制可以避免文件句柄泄漏，也能避免多个句柄对同一文件互相覆盖写入。

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

### RollingFileAppender

按时间间隔滚动切割，并自动清理过期文件，适合生产环境使用。

```properties
appender.rolling.type = RollingFileAppender
appender.rolling.dir = ./logs
appender.rolling.file = app.log
appender.rolling.interval = 1h      # 每小时滚动一次
appender.rolling.maxAge = 168h      # 保留 7 天
appender.rolling.syncLock = false   # 异步模式下不需要锁
appender.rolling.layout.type = JSONLayout
```

**常用间隔配置**：

| 配置 | 说明 | 文件名示例 |
|------|------|-----------|
| `interval = 1h` | 每小时滚动 | app.log.2024010112 |
| `interval = 24h` | 每天滚动 | app.log.2024010100 |
| `interval = 720h` | 每月滚动（30天） | app.log.2024010100 |


---

## Layout 详解

### Layout 接口

```go
type Layout interface {
    EncodeTo(e *Event, w Writer) // 将事件编码写入到 writer
}
```

### TextLayout

人类可读格式，适合开发环境和本地调试。

```properties
appender.console.layout.type = TextLayout
appender.console.layout.fileLineMaxLength = 48
```

**输出格式**：

```
[级别][时间][文件:行号] 标签||上下文字符串||key1=value1||key2=value2
```

**示例**：

```
[INFO][2024-01-01T12:00:00.000][main.go:42] _biz_order_create||trace-12345||order_id=ORD001||success=true
```

**分隔符设计**：

TextLayout 使用 `||` 作为字段分隔符，而不是逗号或空格。因为：

1. `||` 几乎不会出现在正常的日志内容中
2. 视觉上显眼，容易区分字段边界
3. 输入方便，不需要特殊输入法

**文件路径截断**：

当文件路径超过配置的 `fileLineMaxLength` 时，头部会被截断并用 `...` 替换，保证日志行不会太长：

```
.../order/service/create_order.go:123
```

在深度嵌套的项目中，文件路径可能很长。截断后可以避免单条日志过宽，提升控制台阅读体验。

### JSONLayout

结构化 JSON 格式，适合生产环境和日志收集系统（ELK、Loki 等）。

```properties
appender.file.layout.type = JSONLayout
```

**输出示例**：

```json
{"level":"info","time":"2024-01-01T12:00:00.000","fileLine":"main.go:42","tag":"_biz_order_create","trace_id":"trace-12345","order_id":"ORD001","success":true}
```

**性能优化**：

JSONLayout 不使用标准库的 `encoding/json`，而是使用专门面向日志字段的编码器，减少反射和中间对象分配。

优化点：
1. **不需要反射**：每种字段类型有专门的编码路径
2. **不需要中间内存分配**：直接写入输出 buffer，不需要先生成一个大字符串再复制
3. **可以跳过不需要的字段**：如果某个字段是空的，直接跳过不输出
4. **浮点数精度控制**：避免输出不必要的小数位

标准库的 `encoding/json` 是通用编码器，需要处理 struct、interface{}、嵌套对象等复杂情况。日志字段的结构更明确，因此可以使用更轻量的专用编码路径。

---

## 上下文提取

在微服务架构下，日志通常需要携带 `trace_id`、`span_id`、`request_id` 等上下文字段，方便按请求或调用链检索。

一种直接方式是在每次日志调用时手动传入这些字段：

```go
log.Info(ctx, tag, traceID, fields...)
```

但这种方式会让业务代码重复传递上下文字段，也容易遗漏。Go-Spring 提供全局钩子，从 `context.Context` 中自动提取上下文信息：

- 日志调用保持简洁，只需要传入 `ctx`、`tag` 和业务字段。
- 提取逻辑集中管理，便于统一接入链路追踪、网关请求 ID 或用户身份字段。
- 新增上下文字段时，只需要调整提取函数，不必修改所有日志调用点。

需要注意的是，提取函数是全局设置。通常应在应用启动阶段完成配置，并在测试中做好恢复或隔离，避免不同测试之间互相影响。

### 单值提取（StringFromContext）

适用于只需要 traceID 的场景。提取的字符串会在 TextLayout 中单独显示，比普通字段更显眼：

```go
log.StringFromContext = func(ctx context.Context) string {
    if traceID, ok := ctx.Value("trace_id").(string); ok {
        return traceID
    }
    return ""
}
```

### 多字段提取（FieldsFromContext）

适用于需要多个上下文字段的场景（traceID、spanID、userID、requestID 等）：

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
    var fields []log.Field

    if traceID, ok := ctx.Value("trace_id").(string); ok {
        fields = append(fields, log.String("trace_id", traceID))
    }

    if spanID, ok := ctx.Value("span_id").(string); ok {
        fields = append(fields, log.String("span_id", spanID))
    }

    if userID, ok := ctx.Value("user_id").(int64); ok {
        fields = append(fields, log.Int64("user_id", userID))
    }

    if requestID, ok := ctx.Value("request_id").(string); ok {
        fields = append(fields, log.String("request_id", requestID))
    }

    return fields
}
```

**性能提示**：上下文提取函数在每次日志输出时都会调用。请避免在函数中进行复杂计算、IO 操作或内存分配。如果需要计算，提前在 ctx 中计算好并存起来。只提取真正需要的字段，不要提取多余的东西。

---

## 配置系统

Go-Spring 使用扁平化的 KV 配置模型，支持 properties、yaml、toml 等多种格式转换。

```properties
# ==================== 环境变量引用 ====================
log.dir = ${LOG_DIR:./logs}          # 优先从环境变量读取，默认 ./logs
log.level = ${LOG_LEVEL:INFO}        # 优先从环境变量读取，默认 INFO

# ==================== Layout 配置 ====================
layout.text.type = TextLayout
layout.text.fileLineMaxLength = 64

layout.json.type = JSONLayout

# ==================== Appender 配置 ====================
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = RollingFileAppender
appender.file.dir = ${log.dir}
appender.file.file = app.log
appender.file.interval = 24h
appender.file.maxAge = 168h
appender.file.syncLock = false
appender.file.layout.type = JSONLayout

appender.error_file.type = RollingFileAppender
appender.error_file.dir = ${log.dir}
appender.error_file.file = error.log
appender.error_file.interval = 24h
appender.error_file.maxAge = 720h
appender.error_file.layout.type = JSONLayout

appender.audit_file.type = FileAppender
appender.audit_file.dir = ${log.dir}
appender.audit_file.file = audit.log
appender.audit_file.layout.type = JSONLayout

# ==================== Logger 配置 ====================
logger.root.type = AsyncLogger
logger.root.level = ${log.level}
logger.root.bufferSize = 50000
logger.root.onBufferFull = drop-oldest
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[1].ref = file

# 业务日志专用 Logger
logger.biz.type = AsyncLogger
logger.biz.level = DEBUG
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
logger.biz.bufferSize = 20000
logger.biz.onBufferFull = discard
logger.biz.appenderRef.ref = file

# RPC 日志专用 Logger
logger.rpc.type = AsyncLogger
logger.rpc.level = INFO
logger.rpc.tag = _rpc_redis_*,_rpc_http_*,_rpc_mysql_*
logger.rpc.bufferSize = 30000
logger.rpc.onBufferFull = drop-oldest
logger.rpc.appenderRef.ref = file

# 错误日志独立输出
logger.error.type = SyncLogger
logger.error.level = WARN
logger.error.tag = _*
logger.error.appenderRef.ref = error_file

# 审计日志专用 Logger
logger.audit.type = SyncLogger
logger.audit.level = INFO
logger.audit.tag = _biz_audit_*
logger.audit.appenderRef.ref = audit_file
```

### 属性引用

支持 `${key}` 语法引用其他属性的值，支持嵌套引用：

```properties
# 定义公共属性
log.dir = /var/log/app
log.level = INFO
log.retention = 168h

# 引用公共属性
appender.file.dir = ${log.dir}
appender.file.maxAge = ${log.retention}
logger.root.level = ${log.level}

# 嵌套引用
app_name = myapp
log_file = ${app_name}.log
appender.file.file = ${log_file}
```

### 内联 Map 表达式

对于复杂配置，可以使用 `!` 后缀表示内联 Map：

```properties
# 普通写法
db.host = localhost
db.port = 3306
db.name = app

# 内联 Map 写法（效果相同）
db! = {host: localhost, port: 3306, name: app}
```

这种写法会在解析配置时展开。它适合配置结构较复杂、但希望在配置文件中保持紧凑表达的场景。

### 数组配置

数组配置支持两种方式：

```properties
# 索引方式（推荐，清晰）
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[1].ref = file
logger.root.appenderRef[2].ref = monitor

# 逗号分隔方式（简单值适用）
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
```

索引方式更通用，支持任意数量的数组元素，也支持数组元素是复杂对象。逗号分隔方式只适合简单字符串数组。

### 插件注入机制

Go-Spring 日志配置基于插件化架构，核心是两个 Struct Tag：

**PluginAttribute** - 注入简单属性：

```go
type RollingFileAppender struct {
    AppenderBase

    FileDir  string        `PluginAttribute:"dir,default=./logs"`
    FileName string        `PluginAttribute:"file"`
    Interval time.Duration `PluginAttribute:"interval,default=1h"`
    MaxAge   time.Duration `PluginAttribute:"maxAge,default=168h"`
    SyncLock bool          `PluginAttribute:"syncLock,default=false"`
}
```

**PluginElement** - 注入子插件对象：

```go
type ConsoleLogger struct {
    LoggerBase

    // Layout 是一个插件接口，根据 type 字段实例化
    Layout Layout `PluginElement:"layout,default=TextLayout"`
}

type SyncLogger struct {
    LoggerBase

    // AppenderRef 数组，每个元素也是插件
    AppenderRefs []*AppenderRef `PluginElement:"appenderRef"`
}
```

**类型转换器**：

系统内置了常用类型的转换器，也可以注册自定义转换器：

```go
// 内置转换器
log.RegisterConverter(time.ParseDuration)
log.RegisterConverter(strconv.ParseBool)
log.RegisterConverter(strconv.ParseInt)
// ...

// 自定义转换器
func ParseBufferFullPolicy(s string) (BufferFullPolicy, error) {
    switch s {
    case "block":
        return BufferFullPolicyBlock, nil
    case "discard":
        return BufferFullPolicyDiscard, nil
    case "drop-oldest":
        return BufferFullPolicyDropOldest, nil
    default:
        return -1, fmt.Errorf("invalid policy: %s", s)
    }
}

log.RegisterConverter(ParseBufferFullPolicy)
```

### 配置刷新

```go
// 从 map 加载
err := log.RefreshConfig(configMap)

// 从 properties 文件加载
err := log.RefreshFile("log.properties")
```

**原子刷新流程**：

配置刷新是原子操作，用于保证配置切换期间日志处理行为一致：

1. **加锁**：获取全局互斥锁，保证同一时间只有一个刷新在进行
2. **解析与验证**：解析新配置，验证所有插件类型、属性引用等是否正确
3. **创建新组件**：创建所有新的 Appender 和 Logger 实例
4. **启动新组件**：调用所有新组件的 Start() 方法
5. **原子切换**：通过 `atomic.Pointer` 原子地替换所有 tag 绑定的 Logger 引用
6. **停止旧组件**：优雅停止所有旧的 Logger 和 Appender
7. **释放锁**：刷新完成

这个流程保证了：
- 旧组件在切换完成后才停止，尽量避免刷新过程中丢失日志。
- 不会出现“部分新配置，部分旧配置”的中间状态。
- 刷新失败时不会切换到无效配置。

---

## 错误处理

### 错误上报钩子

Appender 写入错误不会 panic，也不会递归写错误日志，而是通过全局钩子上报：

```go
log.ReportError = func(err error) {
    // 1. 增加监控指标
    metric.Incr("log_write_error_total")

    // 2. 分类告警
    errStr := err.Error()

    switch {
    case strings.Contains(errStr, "no space left"):
        alert.Emergency("日志磁盘空间不足！请立即处理。")
    case strings.Contains(errStr, "permission denied"):
        alert.Critical("日志文件权限错误，无法写入")
    case strings.Contains(errStr, "file already closed"):
        alert.Warning("日志文件句柄已关闭，可能是滚动异常")
    default:
        alert.Info("日志写入错误: %s", errStr)
    }

    // 3. 写入独立的错误记录文件（不要使用 log 本身，避免循环）
    fmt.Fprintf(os.Stderr, "[LOG ERROR] %v\n", err)
}
```

**设计原则**：日志系统故障不应导致业务请求失败。日志写入失败时应通过监控、告警或标准错误输出暴露问题，但不应把错误递归写回日志系统本身。

---

## 与其他日志库的兼容

### GetLogger API

提供 `GetLogger` API 用于兼容第三方日志库或项目迁移：

```go
// 获取名为 "std" 的 Logger 包装器
stdLogger := log.GetLogger("std")

// 直接写入字节流（适配 log/stdlog 等）
stdLogger.Write(log.InfoLevel, []byte("hello world\n"))
```

### 适配标准库 log

```go
type GoSpringWriter struct {
    logger *log.LoggerWrapper
    level  log.Level
}

func (w *GoSpringWriter) Write(p []byte) (n int, err error) {
    w.logger.Write(w.level, p)
    return len(p), nil
}

// 使用方式
import stdlog "log"

stdlog.SetOutput(&GoSpringWriter{
    logger: log.GetLogger("stdlog"),
    level:  log.InfoLevel,
})

// 现在标准库的 log 输出都会走 Go-Spring 的日志系统
stdlog.Println("hello from standard library log")
```

### 适配 Zap

```go
type ZapCore struct {
    logger *log.LoggerWrapper
    tag    *log.Tag
}

func (c *ZapCore) Write(p []byte, level zapcore.Level) error {
    var l log.Level
    switch level {
    case zapcore.DebugLevel:
        l = log.DebugLevel
    case zapcore.InfoLevel:
        l = log.InfoLevel
    case zapcore.WarnLevel:
        l = log.WarnLevel
    case zapcore.ErrorLevel:
        l = log.ErrorLevel
    default:
        l = log.InfoLevel
    }
    c.logger.Write(l, p)
    return nil
}

// ... 实现其他 zapcore.Core 接口方法 ...
```

---

## 最佳实践

### 1. 标签统一定义

在公共包中统一定义所有标签，避免重复和拼写错误：

```go
// pkg/logtags/tags.go
package logtags

import "github.com/go-spring/log"

var (
    // 应用层
    AppStartup  = log.RegisterAppTag("startup", "")
    AppShutdown = log.RegisterAppTag("shutdown", "")
    AppConfigReload = log.RegisterAppTag("config", "reload")

    // 业务层 - 订单
    BizOrderCreate = log.RegisterBizTag("order", "create")
    BizOrderCancel = log.RegisterBizTag("order", "cancel")
    BizOrderQuery  = log.RegisterBizTag("order", "query")
    BizOrderPay    = log.RegisterBizTag("order", "pay")

    // 业务层 - 用户
    BizUserLogin    = log.RegisterBizTag("user", "login")
    BizUserRegister = log.RegisterBizTag("user", "register")
    BizUserUpdate   = log.RegisterBizTag("user", "update")

    // RPC 层 - Redis
    RpcRedisGet    = log.RegisterRPCTag("redis", "get")
    RpcRedisSet    = log.RegisterRPCTag("redis", "set")
    RpcRedisDel    = log.RegisterRPCTag("redis", "del")
    RpcRedisHGet   = log.RegisterRPCTag("redis", "hget")

    // RPC 层 - HTTP
    RpcHttpRequest  = log.RegisterRPCTag("http", "request")
    RpcHttpResponse = log.RegisterRPCTag("http", "response")

    // RPC 层 - MySQL
    RpcMysqlQuery = log.RegisterRPCTag("mysql", "query")
    RpcMysqlExec  = log.RegisterRPCTag("mysql", "exec")
    RpcMysqlTx    = log.RegisterRPCTag("mysql", "transaction")
)
```

这样做有几个好处：
- IDE 可以自动补全标签名，减少拼写错误
- 可以写工具扫描所有标签，生成文档
- 可以检查配置文件中引用的标签是否存在
- 新增标签时，所有人都可以看到

### 2. 生产环境配置推荐

```properties
# 生产环境推荐配置
logger.root.type = AsyncLogger
logger.root.level = INFO
logger.root.bufferSize = 50000
logger.root.onBufferFull = drop-oldest
logger.root.appenderRef.ref = rolling

appender.rolling.type = RollingFileAppender
appender.rolling.dir = /var/log/app
appender.rolling.file = app.log
appender.rolling.separate = true        # WARN+ 分离到 .wf 文件
appender.rolling.interval = 24h         # 每天滚动
appender.rolling.maxAge = 720h          # 保留 30 天
appender.rolling.syncLock = false       # 异步模式不需要锁
appender.rolling.layout.type = JSONLayout
```

**关键点**：
- 使用 AsyncLogger，降低日志写入对业务请求的影响。
- 使用 RollingFileAppender，自动轮转和清理日志文件。
- 使用 JSON 格式，方便日志系统解析。
- 开启 separate 模式，将错误日志单独写入文件，方便排查。

### 3. 开发环境配置推荐

```properties
# 开发环境推荐配置
logger.root.type = SyncLogger
logger.root.level = DEBUG
logger.root.appenderRef.ref = console

appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout
appender.console.layout.fileLineMaxLength = 64
```

**关键点**：
- 使用 SyncLogger + Console，便于实时查看输出。
- 将级别设置为 DEBUG，方便本地调试。
- 使用 Text 格式，提高可读性。

### 4. 字段命名约定

```go
// ✅ 推荐：蛇形命名，简洁清晰，语义明确
log.String("order_id", orderID)
log.Int64("user_id", userID)
log.Float64("total_amount", amount)
log.Bool("is_success", success)
log.String("error_message", errMsg)
log.Int64("duration_us", duration.Microseconds())
log.String("client_ip", clientIP)
log.String("user_agent", userAgent)

// ❌ 避免：驼峰、前缀冗余、过度缩写、含义模糊
log.String("orderId", orderID)              // 驼峰
log.String("order_order_id", orderID)        // 重复前缀
log.String("oid", orderID)                   // 过度缩写
log.String("data", something)                // 含义模糊
log.String("value", value)                   // 含义模糊
```

**命名原则**：
- 使用全小写和下划线分隔（snake_case）。
- 字段名应准确反映含义。
- 避免缩写，除非是非常通用的缩写（如 ip、id、ua）。
- 不要添加不必要的前缀。

### 5. 敏感信息处理

不要在日志中输出敏感信息：

```go
// ❌ 错误：明文密码、手机号、身份证号
log.Info(ctx, tag,
    log.String("password", password),
    log.String("phone", "13800138000"),
    log.String("id_card", "110101199001011234"),
)

// ✅ 正确：脱敏或不输出
log.Info(ctx, tag,
    log.String("phone", maskPhone(phone)),
    log.String("id_card", maskIDCard(idCard)),
    // 密码直接不输出
)

// 脱敏函数示例
func maskPhone(phone string) string {
    if len(phone) != 11 {
        return "***"
    }
    return phone[:3] + "****" + phone[7:]
}

func maskIDCard(idCard string) string {
    if len(idCard) != 18 {
        return "***"
    }
    return idCard[:6] + "********" + idCard[14:]
}
```

**需要脱敏的常见信息**：
- 密码、token、密钥
- 手机号、邮箱、身份证号
- 银行卡号、支付信息
- 精确的地理位置信息
- 用户的其他个人隐私信息

### 6. 性能敏感场景优化

在 QPS 很高的热点路径上，日志开销需要重点关注。Go-Spring 提供两种主要优化方式。

**第一种：先检查级别，再构造字段**

```go
// ✅ 推荐：先判断级别是否启用，再构造字段
logger := log.GetLoggerForTag(tag)
if logger.GetLevel().Enable(log.DebugLevel) {
    // 只有级别启用时，这里面的代码才会执行
    log.Debug(ctx, tag,
        log.String("key", key),
        log.Any("value", heavySerialize(value)),
    )
}
```

**第二种：惰性求值（推荐）**

对于开销较大的日志内容，可以使用惰性求值避免不必要的计算。当日志级别未启用时，闭包内代码不会执行。

```go
// ✅ 推荐：使用惰性求值，闭包内代码只有级别启用时才执行
log.Trace(ctx, tag, func() []log.Field {
    // 复杂的序列化操作，开销很大
    detail := json.Marshal(complexObject)

    // 循环计算统计数据
    stats := calculateExpensiveStats()

    // 可能还需要调用外部系统
    extraInfo := fetchExternalInfo()

    return []log.Field{
        log.String("detail", string(detail)),
        log.Any("stats", stats),
        log.String("extra", extraInfo),
        log.Int("item_count", len(complexObject.Items)),
    }
})
```

**常见适用场景**：
- 调试日志需要序列化复杂对象
- 需要循环计算统计数据
- 需要调用外部系统获取额外信息
- 循环中构造大量字段

**反面教材**：

```go
// ❌ 避免：不管级别是否启用，先构造所有字段
log.Debug(ctx, tag,
    log.String("key", key),
    log.Any("value", heavySerialize(value)), // 即使 DEBUG 关闭，这行也会执行！
)
```

惰性求值适合性能敏感路径。它允许保留详细调试日志，同时避免在日志级别关闭时产生额外计算开销。

### 7. 进程退出处理

确保进程退出前所有缓冲日志都落地：

```go
func main() {
    // ... 初始化 ...

    // 监听退出信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan,
        syscall.SIGINT,  // Ctrl+C
        syscall.SIGTERM, // Kubernetes 发送
        syscall.SIGQUIT, // Ctrl+\
    )

    sig := <-sigChan
    log.Infof(ctx, logtags.AppShutdown, "收到信号 %s，正在优雅退出...", sig)

    // 先销毁日志系统，确保缓冲刷出
    log.Destroy()

    // 再退出
    os.Exit(0)
}
```

**特别注意**：
- `log.Destroy()` 必须在 `os.Exit()` 之前调用
- 不能放在 defer 中，因为 `os.Exit()` 不会执行 defer
- 如果进程被 `kill -9` 杀死，任何日志库都无法挽救

### 8. 日志量控制

日志不是越多越好。过多日志会带来以下问题：
- 增加磁盘成本
- 降低应用性能
- 增加日志收集和分析的成本
- 真正重要的信息被淹没在噪音中

**控制日志量的方法**：

1. **合理设置日志级别**：生产环境默认 INFO，不要全开 DEBUG
2. **采样**：对于高频请求，只采样输出部分日志
3. **限流**：同一类错误不要反复输出（1 分钟内最多输出 10 次）
4. **聚合**：优先输出统计结果，而不是逐条输出明细

```go
// 采样示例：每 100 条请求只输出 1 条
if requestCount % 100 == 0 {
    log.Info(ctx, tag, log.Int("count", requestCount), log.Msg("请求处理"))
}

// 限流失效示例：使用 golang.org/x/time/rate
if errorLimiter.Allow() {
    log.Error(ctx, tag, log.String("err", err.Error()))
}
```

---

## 常见问题解答

### Q: 为什么我的日志没有输出？

按照以下步骤排查：

1. **检查 Logger 的 level 配置**：确认日志级别在范围内。例如，Logger 配置为 DEBUG 时不会输出 TRACE 日志。
2. **检查标签是否匹配**：确认 tag 配置包含你的标签。可以在配置中先试试 `*` 通配符，如果这样能输出，说明是标签匹配的问题。
3. **检查 Appender 是否正确配置并启动**：Appender 的 type 拼写是否正确？Appender 的配置是否完整？路径是否存在？权限是否正确？
4. **检查是否有错误上报**：设置 ReportError 钩子，确认是否存在写入错误。磁盘满、权限不足等问题都会导致日志写入失败。
5. **确认 Refresh 没有返回错误**：配置文件有语法错误时，Refresh 会返回错误。如果忽略返回值，日志系统可能仍使用旧配置，或处于未正确配置的状态。

### Q: 异步模式下进程退出时丢日志怎么办？

确保退出前调用 `log.Destroy()`，它会：
1. 停止所有 AsyncLogger，等待缓冲清空
2. 停止所有 Appender，刷出所有待写数据
3. 关闭所有文件句柄

如果进程被 `kill -9` 杀死，进程没有机会执行清理代码，缓冲中的日志可能丢失。这是操作系统级别的限制，日志库无法规避。

### Q: 如何动态调整日志级别？

可以实现配置热更新，然后调用 `Refresh` 重新加载：

```go
// 监听配置文件变化
watcher, err := fsnotify.NewWatcher()
if err != nil { /* 处理错误 */ }

go func() {
    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                log.Infof(ctx, tag, "配置文件变化，重新加载日志配置")
                if err := log.RefreshFile("log.properties"); err != nil {
                    log.Errorf(ctx, tag, "重新加载日志配置失败: %v", err)
                }
            }
        case err := <-watcher.Errors:
            log.Errorf(ctx, tag, "配置监听错误: %v", err)
        }
    }
}()
```

因为 `Refresh` 是原子操作，可以在运行时安全调用。

### Q: 标签和日志级别是什么关系？

标签和日志级别是两个正交的维度：

- 标签决定了日志由哪个 Logger 处理
- 日志级别决定了这个 Logger 处理哪些级别的日志

匹配流程：先通过标签找到 Logger，然后 Logger 根据自己的级别范围决定是否输出这条日志。

这意味着，你可以给不同标签的日志配置不同的级别。比如：
- 业务日志：INFO 及以上
- RPC 日志：WARN 及以上（平时不需要看，出问题时再调低）
- 调试日志：DEBUG 及以上（开发环境开启，生产环境关闭）

### Q: 如何统计每种标签的日志量？

可以通过包装 Appender 来实现统计：

```go
type StatsAppender struct {
    log.Appender
    counter *prometheus.CounterVec
}

func (s *StatsAppender) Append(e *log.Event) {
    s.counter.WithLabelValues(e.Tag, e.Level.String()).Inc()
    s.Appender.Append(e)
}
```

然后在配置刷新时，用 StatsAppender 包装原始 Appender。通过插件化扩展，可以在不修改核心代码的情况下增加统计能力。

### Q: 一个 Logger 可以绑定多个 Appender 吗？

可以。每个 Logger 可以绑定多个 AppenderRef，每个 AppenderRef 还可以有自己的级别过滤。例如：

```properties
logger.root.appenderRef[0].ref = console  # 所有级别都输出到控制台
logger.root.appenderRef[1].ref = file     # 所有级别都写入文件
logger.root.appenderRef[2].ref = alert    # 只有 ERROR 及以上才触发告警
logger.root.appenderRef[2].level = ERROR
```

### Q: 多个 Logger 可以写入同一个文件吗？

可以。FileAppender 内部有引用计数机制，多个 Appender 写入同一个文件时，会共享同一个文件句柄，不会出现句柄泄漏或互相覆盖的问题。

---

## 扩展开发

### 自定义 Appender

实现 `Appender` 接口并注册：

```go
package myappender

import (
    "strings"
    "github.com/go-spring/log"
    "github.com/Shopify/sarama"
)

// KafkaAppender 将日志发送到 Kafka
type KafkaAppender struct {
    log.AppenderBase

    Brokers string `PluginAttribute:"brokers"`
    Topic   string `PluginAttribute:"topic"`
    Async   bool   `PluginAttribute:"async,default=true"`

    producer sarama.SyncProducer
}

func (k *KafkaAppender) Start() error {
    config := sarama.NewConfig()
    config.Producer.RequiredAcks = sarama.WaitForLocal
    config.Producer.Return.Successes = !k.Async

    var err error
    k.producer, err = sarama.NewSyncProducer(strings.Split(k.Brokers, ","), config)
    return err
}

func (k *KafkaAppender) Stop() {
    if k.producer != nil {
        _ = k.producer.Close()
    }
}

func (k *KafkaAppender) Append(e *log.Event) {
    buf := log.GetBuffer()
    defer log.PutBuffer(buf)

    k.Layout.EncodeTo(e, buf)

    msg := &sarama.ProducerMessage{
        Topic: k.Topic,
        Value: sarama.ByteEncoder(buf.Bytes()),
    }

    _, _, err := k.producer.SendMessage(msg)
    if err != nil {
        log.ReportError(err)
    }
}

func (k *KafkaAppender) ConcurrentSafe() bool {
    return true // sarama 的 producer 是并发安全的
}

// 注册插件
func init() {
    log.RegisterPlugin[KafkaAppender]("KafkaAppender")
}
```

**使用配置**：

```properties
appender.kafka.type = KafkaAppender
appender.kafka.brokers = kafka1:9092,kafka2:9092,kafka3:9092
appender.kafka.topic = app-logs
appender.kafka.layout.type = JSONLayout

logger.root.appenderRef.ref = kafka
```

### 自定义 Layout

实现 `Layout` 接口：

```go
package mylayout

import (
    "github.com/go-spring/log"
)

// LTSVLayout 输出 LTSV (Labeled Tab-separated Values) 格式
// 格式: key:value<TAB>key:value...
type LTSVLayout struct {
    log.BaseLayout
}

func (l *LTSVLayout) EncodeTo(e *log.Event, w log.Writer) {
    // 级别
    w.WriteString("level:")
    w.WriteString(e.Level.LowerName())
    w.WriteByte('\t')

    // 时间
    w.WriteString("time:")
    w.WriteString(e.Time.Format("2006-01-02T15:04:05.000Z07:00"))
    w.WriteByte('\t')

    // 文件行号
    w.WriteString("file:")
    w.WriteString(l.GetFileLine(e))
    w.WriteByte('\t')

    // 标签
    w.WriteString("tag:")
    w.WriteString(e.Tag)
    w.WriteByte('\t')

    // 上下文字符串
    if e.CtxString != "" {
        w.WriteString("ctx:")
        w.WriteString(e.CtxString)
        w.WriteByte('\t')
    }

    // 上下文字段
    enc := log.NewTextEncoder(w, '\t')
    enc.AppendSeparator = true
    log.EncodeFields(enc, e.CtxFields)
    log.EncodeFields(enc, e.Fields)

    // 换行
    w.WriteByte('\n')
}

func init() {
    log.RegisterPlugin[LTSVLayout]("LTSVLayout")
}
```

**使用配置**：

```properties
appender.file.layout.type = LTSVLayout
```

### 自定义级别

```go
// 注册自定义 AUDIT 级别，code=350（在 INFO 和 WARN 之间）
var AuditLevel = log.RegisterLevel(350, "AUDIT")

// 使用自定义级别输出
log.Record(ctx, AuditLevel, TagBizAudit, skip,
    log.String("user_id", "10086"),
    log.String("action", "modify_password"),
    log.String("ip", "192.168.1.1"),
)
```

---

Go-Spring 日志系统的核心目标是让业务代码保持简单，同时让日志策略具备足够的配置能力和扩展空间。

在实际项目中，建议先统一标签和字段命名，再根据环境选择合适的 Logger、Appender 和 Layout。这样既能满足本地调试需求，也能平滑接入生产环境的日志收集、告警和链路追踪系统。
