# 日志

Go-Spring 提供了一套高性能、可扩展、基于标签路由的结构化日志系统。
它借鉴了 Log4j2 的分层插件架构，把日志分类、路由、输出、格式化和上下文提取拆成独立的可插拔组件，
同时融入 Go 生态对简洁和性能的追求，避免了 Log4j2 那类臃肿复杂的配置方式。

从此业务代码只需要说清楚"这是什么日志"和"记录哪些字段"，而不用操心最终写到哪里、用什么格式、是不是异步写入。
无论输出的是文本还是 JSON、同步还是异步、单输出还是多目标分发，都可以通过同一套配置语言灵活表达。

---

## 快速开始

下面的示例展示了完整的使用流程：先注册标签，再加载配置，最后分别输出格式化日志和结构化日志。

```go
package main

import (
	"context"
	"os"

	"go-spring.org/log"
)

// 注册标签，标签用于实现日志路由，在配置文件中指定输出目标，同时具备业务语义。
var (
	TagAppStartup  = log.RegisterAppTag("startup", "")     // 应用启动
	TagAppShutdown = log.RegisterAppTag("shutdown", "")    // 应用关闭
	TagBizOrder    = log.RegisterBizTag("order", "create") // 创建订单
	TagBizUser     = log.RegisterBizTag("user", "login")   // 用户登录
)

func main() {
	// 配置要求输出 INFO 及以上级别的日志到控制台。
	config := map[string]string{
		"logger.root.type":  "ConsoleLogger",
		"logger.root.level": "INFO",
	}

	// 使用 KV 配置，支持从多种配置来源加载配置。
	if err := log.RefreshConfig(config); err != nil {
		panic("日志配置失败: " + err.Error())
	}

	ctx := context.Background()

	// 格式化日志，format 模式，与标准库的 fmt.Sprintf 一致。
	log.Infof(ctx, TagAppStartup, "应用启动成功，版本: %s，PID: %d", "v1.0.0", os.Getpid())

	// 结构化日志，field 模式，与 zerolog、logrus 等日志库一致。
	log.Info(ctx, TagBizOrder,
		log.Int("order_id", 10001),
		log.String("user", "alice"),
		log.Float("amount", 99.99),
		log.Bool("success", true),
		log.Strings("tags", []string{"vip", "new_user"}),
		log.Msg("订单创建成功"),
	)

	// 支持惰性计算，避免计算耗时但最终不必要的字段。
	log.Debug(ctx, TagBizUser, func() []log.Field {
		return []log.Field{
			log.String("trace", "user_login_flow"),
			log.Msg("用户登录流程开始"),
		}
	})

	// 支持不同级别的日志 API，日志级别符合常见惯例。
	log.Warnf(ctx, TagBizUser, "用户 %s 密码错误尝试 %d 次", "bob", 3)
	log.Errorf(ctx, TagBizOrder, "订单 %d 创建失败: %s", 10002, "库存不足")
}
```

运行上面的示例，会看到控制台输出类似下面这样的内容。
由于 root logger 的级别是 `INFO`，因此上面的 `Debug` 调用不会真正构造和输出日志。

```text
[INFO][2026-05-02T09:36:40.801][/Users/didi/ccc/myapp/main.go:31] _app_startup||msg=应用启动成功，版本: v1.0.0，PID: 87684
[INFO][2026-05-02T09:36:40.802][/Users/didi/ccc/myapp/main.go:34] _biz_order_create||order_id=10001||user=alice||amount=99.99||success=true||tags=["vip","new_user"]||msg=订单创建成功
[WARN][2026-05-02T09:36:40.802][/Users/didi/ccc/myapp/main.go:52] _biz_user_login||msg=用户 bob 密码错误尝试 3 次
[ERROR][2026-05-02T09:36:40.802][/Users/didi/ccc/myapp/main.go:53] _biz_order_create||msg=订单 10002 创建失败: 库存不足
```

---

## 核心组件

Go-Spring 日志在设计上参考了 Log4j2 的分层插件架构，把日志从产生到落地的整条链路拆成了六个独立层次。
每个层次只负责一类职责，彼此之间通过接口协作，修改其中任何一层都不会影响其他部分。

```
┌───────────────────────────────────────────────────────────┐
│                     应用层 API                            │
│  Trace/Debug/Info/Warn/Error/Panic/Fatal · Record · GetLogger
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                        标签层 (Tag)                       │
│  标签注册  →  标签查找  →  Logger 绑定  →  路由匹配        │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                       Logger 层                           │
│  SyncLogger  │  AsyncLogger  │  Console/File/RollingFile  │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                      Appender 层                          │
│  ConsoleAppender  │  FileAppender  │  RollingFileAppender │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                       Layout 层                           │
│  TextLayout (人类可读)  │  JSONLayout (结构化)             │
└─────────────────────────────┬─────────────────────────────┘
                              │
┌─────────────────────────────▼─────────────────────────────┐
│                      Encoder 层                           │
│  Field 编码  →  类型编码  →  数组/对象编码                │
└───────────────────────────────────────────────────────────┘
```

- **应用层 API**：这是面向业务代码的统一入口，也是整个日志系统的对外门面。
应用层提供了从 `Trace` 到 `Fatal` 的各级别方法，同时支持格式化输出、结构化字段和惰性求值三种调用风格。
它的核心价值在于让业务代码与底层实现完全解耦——
无论后续如何调整输出目标、切换同步异步、更换日志格式，业务侧的日志调用代码都不需要任何改动。
同时，这一层还负责从 `context.Context` 中自动提取链路信息，确保上下文字段无需在每个调用点手动传递。

- **标签层 (Tag)**：这是 Go-Spring 日志系统最具特色的一层，彻底改变了传统日志按包名继承的路由方式。
标签是日志的静态语义标记，在初始化阶段集中注册，配置刷新后与 Logger 完成绑定。
当一条日志产生时，标签层会按照"精确匹配 → 前缀匹配 → root Logger"的优先级进行路由查找，最终决定这条日志进入哪条输出链路。
这种设计可以让同一个代码文件输出的不同语义日志走向完全不同的目的地——
启动日志打印到控制台，业务日志写入文件，审计日志发到 Kafka，错误日志推送到告警系统。

- **Logger 层**：这是日志流的调度中枢，承上启下连接着标签路由与实际写入。
Logger 首先执行级别过滤：如果日志事件的级别不在配置范围内，事件会在这里被直接丢弃，避免后续不必要的编码和 IO 开销。
级别通过后，Logger 负责把事件分发给它绑定的一个或多个 Appender。
Logger 层还决定了写入模型：
同步 Logger 在调用 goroutine 内完成全部写入；
异步 Logger 则把事件先写入内存队列，由后台 goroutine 负责后续处理，从而将日志 IO 与请求路径隔离开。

- **Appender 层**：这是日志真正落地的地方，负责把编码完成的日志事件写入具体的目标系统。
控制台、本地文件、滚动文件、Kafka、HTTP 接口、远程日志服务，每一种输出目标都对应一个 Appender 实现。
一个 Logger 可以同时绑定多个 Appender，实现"一份日志，多路输出"——
例如同一条错误日志既写入本地文件，又上报到告警平台。
如果需要新的输出目标，我们只要实现一个新的 Appender 并注册到插件系统，就可以在配置中启用了。

- **Layout 层**：这是日志的格式编排层，决定了最终输出长什么样。
Layout 不关心字段从哪里来、最终写到哪里去，它只专注一件事：把日志事件转换成目标格式的字节流。
内置的 TextLayout 面向人类可读性，把字段组织成 `key=value` 的形式；
而 JSONLayout 则面向机器解析和检索，输出标准的单行 JSON，直接对接日志采集系统。
如果需要 Protobuf、CSV 等其他格式，我们只要新增对应的 Layout 实现并注册到插件系统，就可以在配置中启用了。

- **Encoder 层**：这是性能优化的关键层次，也是最贴近底层编码细节的一层。
Encoder 的设计目标是尽可能避开反射和中间对象，让字段直接编码写入目标 Writer。
例如基础类型字段（Int、String、Bool 等）都携带自己的类型信息，编码时直接走类型专属的编码路径，不需要动态推断。
数组和嵌套对象则采用流式编码，边遍历边写入，避免先构建完整的中间结构。
这种设计在高并发场景下可以显著减少内存分配和 GC 压力，让日志系统的开销尽可能小。

---

## 标签系统

标签是 Go-Spring 日志系统的核心创新，它从根本上改变了日志的路由方式。

传统日志系统通常按照包名或类名的层级继承关系来路由日志——
`a.b.c` 包的日志继承 `a.b` 的配置，`a.b` 又继承 `a` 的配置。
这种设计在 Java 等面向对象语言中显得自然而然，因为类名本身就代表了代码的语义边界。
但在 Go 中，没有类的层级，包名也往往反映的是代码组织结构，而不是业务语义边界。
更本质的问题是：包名与日志语义从来就不是一一对应的关系——
同一个 `dao` 包里，可能同时输出数据库连接池初始化日志、业务操作日志、以及调用下游的 RPC 日志，
这些日志的重要程度、输出目的地、保留周期应该完全不同，但在包名路由模型下却只能共享同一条输出策略。

Go-Spring 的标签系统正是对这个问题的答案。
标签是日志的静态语义标记，它回答的是"这条日志是什么性质"，而不是"这条日志来自哪个文件"。
通过标签，代码可以显式地声明日志的业务语义，让路由逻辑真正与业务意图对齐，而不是被代码的组织方式束缚。

标签通常在初始化阶段集中注册，并在配置刷新后自动绑定到对应的 Logger。
业务代码只会持有 `*log.Tag`，而不直接持有 Logger，这样后续调整日志输出策略时就不需要改业务代码。

### 标签命名规范

标签命名不只是语法规则，更是团队协作的基础。好的命名能让代码更清晰、日志更好查、排障更高效。
尤其是团队规模扩大后，统一的命名规范能避免"每个人都有自己的一套"的混乱局面。

#### 命名规则

| 序号 | 规则 | 具体说明 | 设计初衷 |
|-----|------|---------|----------|
| 1 | **长度范围 3-36** | 最短 3 个字符，最长不超过 36 个字符 | 太短缺乏语义，太长不利于配置和检索 |
| 2 | **字符集限制** | 只允许小写字母 `a-z`、数字 `0-9` 和下划线 `_` | 避免大小写混乱，确保在文件名、配置键等各种场景下都能兼容 |
| 3 | **分段约束** | 可以有一个前导下划线，去掉前导下划线后最多分成 4 段，段之间用下划线分隔 | 强制形成层级结构，避免毫无章法的随意命名 |
| 4 | **格式约束** | 不允许连续下划线，也不允许以下划线结尾 | 保证格式统一，减少不必要的解析歧义 |
| 5 | **推荐三段式** | 建议遵循 `_<分类>_<子类型>_<动作>` 的格式：分类表示日志大类，子类型表示对象或领域，动作表示发生的操作 | 从源头保证命名一致性，大幅提升跨团队协作时的可理解性 |

#### 分类前缀

Go-Spring 官方推荐以下四类前缀，它们覆盖了绝大多数后端应用场景：

| 分类前缀 | 适用场景 | 典型示例 |
|---------|----------|----------|
| `_app_` | 应用生命周期与基础设施 | 启动、关闭、配置加载、健康检查、定时任务调度 |
| | | `_app_startup`、`_app_shutdown`、`_app_config_reload` |
| `_biz_` | 业务流程与领域事件 | 订单创建、用户登录、支付回调、状态变更通知 |
| | | `_biz_order_create`、`_biz_user_login`、`_biz_pay_success` |
| `_rpc_` | 外部依赖调用 | 数据库操作、缓存读写、消息队列发送、HTTP 下游调用、gRPC 服务请求 |
| | | `_rpc_redis_get`、`_rpc_mysql_query`、`_rpc_http_call` |
| `_infra_` | 框架与中间件内部 | 连接池耗尽、熔断器打开、重试触发、降级逻辑执行 |
| | | `_infra_pool_exhausted`、`_infra_circuit_open` |

> 这些分类只是推荐约定，不是技术限制。你完全可以根据项目特点自定义其他分类，
> 但请记住：**同一个项目内应保持分类的一致性**。

### 标签注册

Go-Spring 提供了 `RegisterAppTag`、`RegisterBizTag` 和 `RegisterRPCTag` 用于生成规范化标签，
也提供了 `RegisterTag` 用于注册自定义标签，但使用时需要注意遵守标签命名规范。

**使用示例：**

```go
// 应用层标签
log.RegisterAppTag("startup", "")      // _app_startup
log.RegisterAppTag("shutdown", "")     // _app_shutdown
log.RegisterAppTag("config", "reload") // _app_config_reload

// 业务层标签
log.RegisterBizTag("order", "create") // _biz_order_create
log.RegisterBizTag("order", "cancel") // _biz_order_cancel
log.RegisterBizTag("user", "login")   // _biz_user_login

// RPC 层标签
log.RegisterRPCTag("redis", "get")   // _rpc_redis_get
log.RegisterRPCTag("http", "call")   // _rpc_http_call
log.RegisterRPCTag("grpc", "invoke") // _rpc_grpc_invoke

// 自定义标签
log.RegisterTag("_cache_hit")        // _cache_hit
log.RegisterTag("_mq_kafka_produce") // _mq_kafka_produce
```

### 标签路由

标签在绑定 Logger 时，采用**精确优先、最长优先**的匹配策略，按优先级从高到低依次查找，匹配到第一个后立即停止。
例如对于标签 `_biz_order_create`，系统会按以下顺序尝试匹配：

| 匹配阶段 | 匹配目标 | 说明 |
|---------|---------|------|
| 1. 精确匹配 | `_biz_order_create` | 完全一致的标签名，优先级最高 |
| 2. 三段前缀 | `_biz_order_*` | 去掉最后一段，保留前三段的前缀匹配 |
| 3. 两段前缀 | `_biz_*` | 保留前两段的前缀匹配 |
| 4. 兜底匹配 | `logger.root` | 所有日志的最终归宿 |

利用前缀匹配的层级特性，我们可以实现"大类走通用配置、小类走特殊配置"的灵活路由。
当新增业务标签时，只要命名符合规范，就能自动归入正确的输出链路。

---

## 日志级别

日志级别用于决定一条日志是否应该输出。
Go-Spring 为每个级别分配了数值，这样我们既可以在标准级别之间插入自定义级别，
又可以用范围表达过滤条件。

| 级别 | 数值 | 说明 |
|------|------|------|
| NONE | 0 | 关闭所有日志输出，适合性能压测等需要临时屏蔽日志的场景。 |
| TRACE | 100 | 最细粒度的跟踪信息，用于记录函数入参出参、循环迭代状态、逐行执行轨迹等，生产环境通常不开启。 |
| DEBUG | 200 | 调试信息，用于记录组件初始化细节、条件分支走向、配置加载详情、中间状态快照等开发排障所需的上下文。 |
| INFO | 300 | 常规运行信息，用于记录服务启动完成、请求处理结束、关键状态变更、定时任务执行结果等正常流程中的里程碑事件。 |
| WARN | 400 | 可恢复的异常警告，用于记录重试发生、调用超时、服务降级、资源接近阈值等非正常但系统仍能继续运行的情况。 |
| ERROR | 500 | 业务或系统错误，用于记录请求处理失败、下游依赖异常、数据校验不通过等需要关注和排查的问题，进程本身仍可正常运行。 |
| PANIC | 600 | 严重系统错误，用于记录即将触发 panic 的异常、核心组件初始化失败、资源耗尽等影响服务可用性的致命问题。 |
| FATAL | 700 | 致命错误，用于记录进程退出前的最后一条日志，通常是无法恢复的核心故障，记录完成后进程将终止。 |
| MAX | 999 | 级别上限标记，仅用于级别范围比较，不直接输出日志。 |

### 自定义级别

我们可以在 Go-Spring 定义的标准日志级别之间插入自定义级别。
例如我们可以定义一个审计级别 `AUDIT`，放在 `INFO` 和 `WARN` 之间：

**示例代码：**

```go
var AuditLevel = log.RegisterLevel(350, "AUDIT")
var TagBizAudit = log.RegisterBizTag("audit", "record")

log.Record(ctx, AuditLevel, TagBizAudit, 2,
	log.String("user_id", "10086"),
	log.String("action", "modify_password"),
	log.String("ip", "192.168.1.1"),
)
```

---

## 日志输出

Go-Spring 提供了格式化日志和结构化日志两类输出 API。
简单文本信息可使用 `*f` 格式化方法；业务日志建议优先采用结构化字段。

**API 示例：**

```go
// 格式化日志
log.Infof(ctx, TagAppStartup, "应用启动成功，版本: %s，PID: %d", "v1.0.0", os.Getpid())
log.Warnf(ctx, TagBizUser, "用户 %s 密码错误尝试 %d 次", "bob", 3)
log.Errorf(ctx, TagBizOrder, "订单 %d 创建失败: %s", 10002, "库存不足")

// 结构化日志
log.Info(ctx, TagBizOrder,
	log.Int("order_id", orderID),
	log.String("status", "created"),
	log.Int("duration_us", duration.Microseconds()),
	log.Msg("订单创建完成"),
)
```

格式化日志写法直观，适合本地调试和简单提示信息。
缺点是字段被拼成字符串，后续检索和统计需要日志平台二次解析。

结构化日志保留字段类型，便于日志系统直接索引和聚合。
在性能敏感路径上，应优先使用强类型字段构造函数。

### 惰性求值

Go-Spring **强制要求** `Trace` 和 `Debug` 使用惰性求值。
这两个级别通常包含复杂序列化、聚合计算等耗时操作，而且线上环境往往不会开启。
**强制** 使用惰性求值从 API 层面保证了级别关闭时不会产生任何无意义的计算开销。

**使用示例：**

```go
log.Trace(ctx, TagBizUser, func() []log.Field {
	detail, _ := json.Marshal(complexObject)
	stats := calculateExpensiveStats()

	return []log.Field{
		log.String("detail", string(detail)),
		log.Any("stats", stats),
		log.Int("item_count", len(complexObject.Items)),
	}
})
```

### 调整堆栈深度

当用户需要封装自己的日志工具函数时，`Info`、`Warn` 这类 API 输出的文件名和行号会指向封装函数本身，
而不是真正的业务调用位置。此时我们可以通过 `Record` 函数的 `skip` 参数调整调用栈深度，跳过封装层的栈帧。

**示例代码：**

```go
func Audit(ctx context.Context, tag *log.Tag, fields ...log.Field) {
	log.Record(ctx, AuditLevel, tag, 3, fields...)
}
```

---

## 结构化日志

传统日志将所有信息拼接成自由文本，检索和统计都依赖正则表达式匹配，既不精确也缺乏类型信息。
结构化日志将日志内容拆解为带类型的键值对（字段），让日志数据变得可直接索引、可聚合计算、可机器理解。

Go-Spring 的 Field 系统参考了 zerolog、zap 等主流日志库，采用**强类型**设计：
每个字段携带类型信息，编码时直接走专属路径，避免反射开销，提升了编解码的性能。

几乎所有 Field 构造函数的第一个参数都是字段名（key），如果有第二个参数，则表示字段值（value）。
只有少数特殊情况例外，例如 `Nil` 字段和 `Msg` 字段。

**基础使用示例：**

```go
log.Info(ctx, tag,
	log.Int("user_id", userID),
	log.String("ip", ip),
	log.Bool("success", success),
	log.Msg("用户登录完成"),
)
```

Go-Spring 提供了丰富的字段类型，覆盖基础类型、指针、数组、嵌套对象等常见场景：

### 基础类型

Go-Spring 为所有基础类型提供了对应的字段构造函数，每个函数直接编码对应类型，避免类型推断开销。

**基础类型字段示例：**

```go
log.Bool("success", true)

log.Int("user_id", 10001)
log.Int("duration_us", duration.Microseconds())

log.Uint("bytes_transferred", uint64(1024*1024))

log.Float("amount", 99.99)
log.Float("latency_ms", 123.45)

log.String("order_no", "ORD202401010001")
log.String("ip", "192.168.1.1")
```

### 指针类型

Go-Spring 为所有基础类型的指针版本提供了对应的字段构造函数，直接处理指针的解引用和空值判断。
如果指针为 `nil`，字段值会输出 `null`；非 `nil` 时则输出指向的实际值。

**指针类型字段示例：**

```go
var enabled *bool
log.BoolPtr("enabled", enabled)

var userID *int64
log.IntPtr("user_id", userID)

var remark *string
log.StringPtr("remark", remark)

log.Nil("deleted_at")
```

### 消息字段

Go-Spring 提供了两个特殊的字段函数 `Msg` 和 `Msgf`。它们的 key 都是 `msg`。
可以用于存放人类可读的日志摘要。
对于结构化信息（用户 ID、订单号、状态码等）仍应拆成独立字段，便于检索和聚合。

**使用示例：**

```go
log.Msg("订单创建成功")
log.Msgf("处理了 %d 条记录，成功 %d，失败 %d", total, success, failed)
```

### 数组和嵌套对象

Go-Spring 也为数组和复杂嵌套对象提供了对应的字段构造函数。

**数组和对象字段示例：**

```go
log.Ints("item_ids", []int{1, 2, 3})
log.Strings("tags", []string{"vip", "new_user"})
log.Bools("flags", []bool{true, false})
log.Floats("prices", []float64{9.99, 19.99})

log.Object("order",
	log.String("order_no", "ORD001"),
	log.Int("user_id", int64(10001)),
	log.Float("amount", 99.99),
	log.Bool("paid", true),
	log.Object("item",
		log.String("sku", "ITEM001"),
		log.Int("quantity", 2),
	),
)
```

### Map 展开

Go-Spring 提供了一个字段函数 `FieldsFromMap`，它会把 `map[string]any` 展开成多个字段，
而不是作为一个 map 字段输出。

**使用示例：**

```go
data := map[string]any{
	"order_id": "ORD001",
	"amount":   99.99,
	"user_id":  int64(10001),
	"success":  true,
}

log.Info(ctx, tag, log.FieldsFromMap(data))
```

### 自动类型推断

Go-Spring 提供了 `Any` 字段函数，它会根据字段值自动选择合适的字段构造函数。
无法识别时，会按照动态类型选择合适的编码路径，最终回退到反射编码。

**使用示例：**

```go
log.Any("order_id", "ORD001")
log.Any("amount", 99.99)
log.Any("user_id", int64(10001))
log.Any("tags", []string{"a", "b"})
```

虽然 `Any` 使用方便，但强类型字段更明确，也能避开反射路径。因此，在可能的情况下，建议使用强类型字段。

---

## Logger

Logger 是整个日志系统的核心调度单元，承上启下连接着标签路由与 Appender 写入。
当一条日志事件到达时，Logger 首先执行级别过滤——
如果事件级别不在配置范围内，会在这里直接丢弃，避免后续不必要的编码和 IO 开销。
级别校验通过后，Logger 负责把事件分发给它绑定的一个或多个 Appender，完成最终的写入操作。

Go-Spring 将 Logger 设计为两大类，分别对应不同的使用场景：

- **组合式 Logger**：包括 `SyncLogger` 和 `AsyncLogger`，本身不包含写入逻辑，
通过 `appenderRef` 引用一个或多个 Appender，支持灵活的多路输出和策略组合。

- **集成式 Logger**：包括 `ConsoleLogger`、`FileLogger`、`RollingFileLogger`，
内部封装了常见的输出目标和 Layout 配置，用更短的配置满足大多数场景需求。

两类 Logger 在功能上完全等价，选择的核心考量是配置的简洁性与灵活性的平衡。
简单场景优先选择集成式 Logger，需要多路输出或复杂策略时再使用组合式 Logger。

### SyncLogger（同步）

`SyncLogger` 是最基础的 Logger 实现，它在业务调用的 goroutine 中直接执行写入流程，
从级别过滤、字段编码到 Appender 写入，全程在同一个调用栈内完成，写入完成后才返回。

同步写入最大的特点是 **确定性强**：
日志要么成功写入，要么立即报错，不会存在"飘在内存缓冲区里"的中间状态。

它特别适合以下场景：
- 应用启动、初始化等低吞吐量阶段，希望错误尽早暴露
- 审计日志、交易流水等关键路径，日志不能丢失
- 本地开发和调试环境，希望日志立即输出便于断点跟踪

下面的配置示例展示了如何用 `SyncLogger` 同时输出到控制台和文件，实现"一源双写"：

**完整配置示例：**

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout

logger.sync.type = SyncLogger
logger.sync.tag = _app_*
logger.sync.level = INFO
logger.sync.appenderRef[0].ref = console
logger.sync.appenderRef[1].ref = file
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `tag` | 是 | Logger 绑定的标签表达式 | `_app_*`、`_biz_order_create` |
| `level` | 是 | 日志级别或级别范围 | `INFO`、`DEBUG~WARN` |
| `appenderRef[n].ref` | 是 | 引用的 Appender 名称 | `console`、`file` |
| `appenderRef[n].level` | 否 | 该 Appender 的级别过滤范围 | `WARN~MAX` |

**使用注意：**
- **同步模式下，绑定的 Appender 必须是并发安全的**。
  因为多个 goroutine 可能同时调用同一个 SyncLogger，底层 Appender 的 `Append` 方法会被并发调用，必须确保并发安全。
- 如果 Appender 本身不保证并发安全（例如未加锁的文件写入），可以在外层配合 `AsyncLogger` 使用，
  由异步 Logger 的单 goroutine 消费来保证写入串行化。
- 同步模式写入会阻塞业务 goroutine，高并发场景建议优先使用 `AsyncLogger`。

### AsyncLogger（异步）

`AsyncLogger` 是面向生产环境高并发场景的 Logger 实现。
它将日志的"产生"与"写入"彻底解耦：
业务 goroutine 只需要把日志事件放入内存缓冲区就立即返回，
真正的字段编码和 Appender 写入由独立的后台 goroutine 异步完成。

这种"生产者-消费者"模式带来两个主要优势：
- **业务请求不受 IO 抖动影响**：磁盘刷盘、网络延迟等慢 IO 操作不会阻塞业务线程，请求响应时间会更稳定
- **写入吞吐量更高**：后台单 goroutine 串行写入可以省去并发锁开销，批量写入也能更好地利用系统缓存

下面的配置示例展示了面向业务日志的典型异步 Logger 配置：

**完整配置示例：**

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout

logger.async.type = AsyncLogger
logger.async.tag = _app_*
logger.async.level = INFO
logger.async.appenderRef[0].ref = console
logger.async.appenderRef[1].ref = file
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `tag` | 是 | Logger 绑定的标签表达式 | `_app_*`、`_biz_order_create` |
| `level` | 是 | 日志级别或级别范围 | `INFO`、`DEBUG~WARN` |
| `bufferSize` | 否 | 缓冲区大小，默认 `10000` | `10000`、`50000` |
| `onBufferFull` | 否 | 缓冲区满时的策略，默认 `block` | `block`、`discard`、`drop-oldest` |
| `appenderRef[n].ref` | 是 | 引用的 Appender 名称 | `console`、`file` |
| `appenderRef[n].level` | 否 | 该 Appender 的级别过滤范围 | `WARN~MAX` |

**缓冲区满策略：**

| 策略 | 行为 | 适用场景 |
|------|------|----------|
| `block` | 业务 goroutine 阻塞直到缓冲区有空间 | 日志不能丢，允许业务请求延迟 |
| `discard` | 直接丢弃新到达的日志事件 | 业务性能优先，允许极端情况下丢日志 |
| `drop-oldest` | 丢弃缓冲区中最旧的事件，为新日志腾空间 | 排查问题更关注最新现场信息 |

**使用注意：**
- 缓冲区大小建议根据日志产生速率和峰值持续时间估算，高并发场景建议调大到 20000-50000
- 异步 Logger 在进程正常 `Stop` 时会触发优雅关闭，尽力写完缓冲中的事件；
  但如果进程被强制杀死（如 `kill -9`），缓冲中未写入的日志仍可能丢失
- 由于 AsyncLogger 内部保证单 goroutine 串行写入，它绑定的 Appender 不需要是并发安全的
- 异步模式下日志输出有毫秒级延迟，调试时需要注意时序问题

### ConsoleLogger

`ConsoleLogger` 是最常用的集成式 Logger，专门面向标准输出（stdout）场景设计，开箱即用。
它内部集成了 `ConsoleAppender` 和 `TextLayout`，无需额外配置 Appender，仅用一组简单配置即可启用控制台输出。

**配置示例：**

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = TextLayout
logger.console.layout.fileLineMaxLength = 30
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `tag` | 是 | Logger 绑定的标签表达式 | `_app_*`、`_biz_order_create` |
| `level` | 是 | 日志级别或级别范围 | `INFO`、`DEBUG~WARN` |
| `layout.*` | 否 | 输出格式配置，默认 `TextLayout` | |

它是本地开发和调试的首选 Logger，也是新项目搭建时的默认配置方案。

**使用注意：**
- **生产环境高并发场景下大量控制台输出可能成为性能瓶颈**。
  因为标准输出是系统全局共享资源，并发写入时内核会加锁保护，大量 goroutine 同时写控制台可能产生严重的锁竞争。
- 建议生产环境优先使用 `FileLogger` 或 `RollingFileLogger` 写本地文件，控制台仅输出启动阶段的关键信息。
- 容器环境中建议输出 JSON 格式，配合采集器直接结构化入库。

### FileLogger

`FileLogger` 是面向单文件输出的集成式 Logger，内部直接封装了 `FileAppender`，
无需额外配置 Appender 就能写入本地文件。
它默认使用面向机器解析的 `JSONLayout`，也可以按需切换为 `TextLayout`。

它比较适合以下场景：
- 小型服务或单体应用，日志量不大，不需要按时间滚动
- 自动化测试和 CI 环境，需要收集日志文件进行断言分析
- 临时调试场景，快速将特定标签的日志输出到独立文件

**配置示例：**

```properties
logger.file.type = FileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
logger.file.layout.fileLineMaxLength = 60
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `tag` | 是 | Logger 绑定的标签表达式 | `_app_*`、`_biz_order_create` |
| `level` | 是 | 日志级别或级别范围 | `INFO`、`DEBUG~WARN` |
| `dir` | 是 | 日志文件目录，启动时自动创建 | `./logs`、`/var/log/app` |
| `file` | 是 | 日志文件名 | `app.log`、`audit.log` |
| `layout.*` | 否 | 输出格式配置，默认 `JSONLayout` | |

**使用注意：**
- 不会自动滚动文件，日志会持续追加到同一个文件，长时间运行可能导致单个文件过大
- 如果需要按时间切割和自动清理过期日志，请使用 `RollingFileLogger`
- 日志目录需要确保应用进程有写入权限，否则会在启动时报错

### RollingFileLogger

`RollingFileLogger` 是面向生产环境设计的全功能集成式 Logger，也是大多数在线服务的推荐选择。
它内部封装了 `RollingFileAppender`，支持按时间自动滚动切割文件，并且可以按级别拆分普通日志和告警日志。
同时它还内置了异步写入能力，可以开箱即用地满足生产环境的绝大多数需求。

它的核心特性包括：
- **时间驱动滚动**：按固定时间间隔自动创建新的日志文件，避免单个文件过大
- **自动过期清理**：超过保留时间的日志文件自动删除，无需外部脚本清理
- **级别分离输出**：普通日志和告警日志分开存储，排查问题时优先查看告警文件
- **内置异步支持**：通过配置开启异步写入，无需在外层包裹 `AsyncLogger`

下面的配置示例展示了生产环境的典型配置：

**生产环境配置示例：**

```properties
logger.file.type = RollingFileLogger
logger.file.tag = _app_*
logger.file.level = INFO
logger.file.dir = ./logs
logger.file.file = app.log
logger.file.layout.type = JSONLayout
logger.file.layout.fileLineMaxLength = 60
logger.file.interval = 24h
logger.file.maxAge = 168h
logger.file.separate = true
logger.file.async = true
logger.file.bufferSize = 50000
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `tag` | 是 | Logger 绑定的标签表达式 | `_app_*`、`_biz_order_create` |
| `level` | 是 | 日志级别或级别范围 | `INFO`、`DEBUG~WARN` |
| `dir` | 是 | 日志文件目录，启动时自动创建 | `./logs`、`/var/log/app` |
| `file` | 是 | 日志文件名前缀 | `app.log`、`audit.log` |
| `interval` | 否 | 滚动时间间隔，默认 `1h`，采用整点对齐 | `1h`、`24h`、`168h` |
| `maxAge` | 否 | 日志最大保留时间，默认 `168h` (7天) | `24h`、`168h`、`720h` |
| `separate` | 否 | 是否启用级别分离输出，默认 `false` | `true`、`false` |
| `async` | 否 | 是否启用内置异步写入，默认 `false` | `true`、`false` |
| `bufferSize` | 否 | 异步缓冲区大小，启用异步时生效 | `10000`、`50000` |
| `layout.*` | 否 | 输出格式配置，默认 `JSONLayout` | |

**使用注意：**
- `interval` 滚动间隔：建议按日志量选择，高流量服务用 `1h`，普通服务用 `24h`
- `maxAge` 保留时间：根据合规要求和磁盘容量决定，通常保留 7 天（168h）或 30 天
- `separate = true` 级别分离：开启后普通日志写入 `app.log.<time>`，`WARN` 及以上级别写入 `app.log.wf.<time>`，可以大幅提升排障效率
- `async = true` 异步写入：开启后内部直接使用异步写入机制，无需再额外包裹一层 `AsyncLogger`
- `bufferSize` 缓冲区大小：高并发场景建议调大到 20000-50000，避免高峰期缓冲满丢弃

### 自定义 Logger

Go-Spring 日志的插件化架构允许用户自定义 Logger 实现，以满足业务系统的特殊需求。

下面实现一个基于百分比采样的自定义 Logger：

**完整实现代码：**

```go
type SamplingLogger struct {
	log.AsyncLogger

	SampleRate float64 `PluginAttribute:"sampleRate,default=0.01"`
	rand       *rand.Rand
}

func (l *SamplingLogger) Start() error {
	l.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return l.AsyncLogger.Start()
}

func (l *SamplingLogger) Stop() {
	l.AsyncLogger.Stop()
}

func (l *SamplingLogger) Append(e *log.Event) {
	// ERROR 及以上级别全部保留
	if e.Level.Code() >= log.ErrorLevel.Code() {
		l.AsyncLogger.Append(e)
		return
	}

	// 低级日志按采样率保留
	if l.rand.Float64() < l.SampleRate {
		e.Fields = append(e.Fields, log.Bool("sampled", true))
		l.AsyncLogger.Append(e)
	}
}

func init() {
	log.RegisterPlugin[SamplingLogger]("SamplingLogger")
}
```

**配置使用示例：**

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout

logger.sampling.type = SamplingLogger
logger.sampling.tag = _app_*
logger.sampling.level = INFO
logger.sampling.appenderRef[0].ref = file
logger.sampling.sampleRate = 0.01
```

示例中的采样 Logger 通过组合（内嵌 `log.AsyncLogger`）复用了异步写入的完整能力，
只在 `Append` 入口处增加了采样过滤逻辑。
这种组合方式让自定义扩展只需要关注差异化逻辑，而不用重新实现缓冲管理、并发控制、优雅关闭等复杂的基础功能。

---

## Appender

Appender 是日志落地的最终执行单元，负责将编码后的日志事件写入各类目标系统。
通过在一个 Logger 中同时绑定多个 Appender，可以灵活实现"一源多输出"的分发策略——
例如同一条错误日志既能持久化到本地文件，又能实时推送到告警平台或监控系统。

Go-Spring 内置了四类核心 Appender，覆盖了后端服务从开发、测试到生产的全场景需求：

| Appender 类型 | 输出目标 |
|---------------|----------|
| `DiscardAppender` | 空输出，所有日志事件被静默丢弃，不产生任何持久化或 IO 操作 |
| `ConsoleAppender` | 进程标准输出（stdout），直接打印到终端或容器日志采集通道 |
| `FileAppender` | 单个本地文件，日志持续追加写入，不自动滚动或清理 |
| `RollingFileAppender` | 按时间自动滚动切割的文件序列，支持自动清理过期日志文件 |

### DiscardAppender

`DiscardAppender` 会静默丢弃所有写入的日志事件，不产生任何实际输出。
大多数情况下不会用到，仅在某些特殊测试场景下使用。

```properties
appender.discard.type = DiscardAppender
```

### ConsoleAppender

`ConsoleAppender` 将编码完成的日志事件直接写入进程标准输出流（stdout），
是本地开发和调试场景的首选输出目标。
它通常配合 `TextLayout` 使用，输出人类可读的格式化文本。

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `layout.*` | 否 | 输出格式配置，默认 `TextLayout` | |

**使用注意：**
- 标准输出是系统全局共享资源，并发写入时内核会加锁保护，大量 goroutine 同时写控制台可能产生严重的锁竞争，导致性能下降。
- 生产环境高并发场景下，建议仅在启动阶段输出关键信息，业务日志统一写入本地文件。
- 容器环境中建议使用 JSON 格式输出，配合日志采集器直接结构化入库。

### FileAppender

`FileAppender` 将日志以追加模式持续写入单个本地文件，是最基础、可靠的持久化输出方案。
它采用操作系统级追加语义保证每条日志的原子完整性，即使进程异常崩溃，已写入的日志也不会损坏。

**它适合在以下场景中使用：**
- **小型低流量服务**：日均日志量不大，不需要按时间滚动切割
- **CI/CD 测试环境**：测试完成后日志文件可用于断言分析和问题回溯
- **定向调试输出**：特定标签的日志输出到独立文件，避免与其他日志混杂
- **短生命周期进程**：定时任务、CLI 工具等，运行结束后日志完整保留
- **审计合规日志**：需要永久归档的审计数据，独立文件便于备份管理

```properties
appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `dir` | 是 | 日志目录 | `./logs`、`/var/log/app` |
| `file` | 是 | 日志文件名 | `app.log`、`audit.log` |
| `layout.*` | 否 | 输出格式，默认 `TextLayout` | |

**使用注意：**
- 启动时会前置检查目录写入权限，权限不足直接失败，避免运行期间静默丢日志
- 不会自动滚动切割文件，服务长期运行可能导致单文件体积过大
- 不提供自动清理机制，历史日志累积需要配合 logrotate 等外部脚本或人工定期处理
- 多个 goroutine 并发写入时，建议配合 `AsyncLogger` 保证串行化
- 当日志量增长需要滚动切割时，可无缝切换到 `RollingFileAppender`，配置高度兼容

### RollingFileAppender

`RollingFileAppender` 是专门面向生产环境设计的全功能持久化输出组件，也是绝大多数在线服务的首选方案。
它按固定时间间隔自动滚动切割文件，内置过期日志自动清理机制，彻底解决单文件无限增长导致的运维难题。
与 `FileAppender` 相比，它增加了生命周期管理能力，在可靠性、性能和可运维性之间取得了更好的平衡。

```properties
appender.rolling.type = RollingFileAppender
appender.rolling.dir = ./logs
appender.rolling.file = app.log
appender.rolling.interval = 1h
appender.rolling.maxAge = 168h
appender.rolling.syncLock = false
appender.rolling.layout.type = JSONLayout
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `dir` | 否 | 日志文件根目录，默认 `./logs`，滚动文件均创建在此目录下 | `./logs`、`/data/logs/app` |
| `file` | 是 | 日志文件名前缀，滚动时自动追加时间戳后缀 | `app.log`、`audit.log` |
| `interval` | 否 | 滚动时间间隔，默认 `1h`，支持小时级精度，采用整点对齐策略 | `1h`、`6h`、`24h` |
| `maxAge` | 否 | 日志最大保留时间，默认 `168h` (7天)，超过后在下一次滚动时自动删除 | `24h`、`168h`、`720h` |
| `syncLock` | 否 | 是否启用内置互斥锁，默认 `false`，开启后保证多写入者的并发安全 | `true`、`false` |
| `layout.*` | 否 | 输出格式配置，默认 `TextLayout` | |

**使用注意：**
- 滚动时间采用整点对齐策略，确保多实例的滚动时机一致，便于日志收集和管理。
  例如配置 `interval = 1h`，第一次滚动会发生在下一个小时的 0 分 0 秒，之后每小时准点滚动一次。
- 同步 Logger 场景下多 goroutine 写入时，建议开启 `syncLock = true` 锁保护
- 配合 `AsyncLogger` 使用时建议设为 `syncLock = false`，由异步 Logger 的单 goroutine 消费保证写入串行化，省去锁开销
- 多进程同时写入同一日志目录时，必须开启 `syncLock = true`，否则可能出现日志内容交错
- 日志保留时间需根据磁盘容量合理规划，避免日志文件耗尽存储空间

### 自定义 Appender

Go-Spring 日志的插件化架构允许用户自定义 Appender，将日志输出到各类目标系统。

下面实现一个基于百分比采样的自定义 Appender：

```go
type SamplingAppender struct {
	log.FileAppender

	SampleRate float64 `PluginAttribute:"sampleRate,default=0.01"`
	rand       *rand.Rand
}

func (a *SamplingAppender) Start() error {
	a.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return a.FileAppender.Start()
}

func (a *SamplingAppender) Stop() {
	a.FileAppender.Stop()
}

func (a *SamplingAppender) Append(e *log.Event) {
	// ERROR 及以上级别全部保留
	if e.Level.Code() >= log.ErrorLevel.Code() {
		a.FileAppender.Append(e)
		return
	}

	// 低级日志按采样率保留
	if a.rand.Float64() < a.SampleRate {
		e.Fields = append(e.Fields, log.Bool("sampled", true))
		a.FileAppender.Append(e)
	}
}

func (a *SamplingAppender) ConcurrentSafe() bool {
	return a.FileAppender.ConcurrentSafe()
}

func init() {
	log.RegisterPlugin[SamplingAppender]("SamplingAppender")
}
```

**配置使用示例：**

```properties
appender.sampling.type = SamplingAppender
appender.sampling.dir = ./logs
appender.sampling.file = app.log
appender.sampling.layout.type = TextLayout
appender.sampling.sampleRate = 0.01

logger.sync.type = SyncLogger
logger.sync.tag = _app_*
logger.sync.level = INFO
logger.sync.appenderRef[0].ref = sampling
```

这个例子和前面的采样 Logger 相似，只是把采样功能移动到了 Appender 层。

---

## Layout

Layout 是日志事件的格式编排层，决定日志事件如何被编码成最终的字节流输出。
它不关心日志从哪里来、最终写到哪里去，只专注一件事：把结构化的 `Event` 对象转换成目标格式。

Layout 采用流式编码设计，字段直接写入输出 `Writer`，避免了先构造完整中间对象再序列化的开销。
这种流式架构在高并发场景下可以显著减少 GC 压力。

Go-Spring 提供了 `TextLayout` 和 `JSONLayout` 两种内置 Layout 实现。

### TextLayout

`TextLayout` 输出面向人类阅读的单行文本格式，是本地开发和控制台排障的默认选择。
它采用 `||` 作为分段分隔符，避免了与日志内容中可能出现的空格、逗号、等号等字符冲突。

它的输出格式如下：

```text
[级别][时间][文件:行号] 标签||上下文字符串||key1=value1||key2=value2||msg=日志消息
```

各分段的含义：

| 分段 | 说明 |
|------|------|
| `[级别]` | 日志级别，INFO / WARN / ERROR 等 |
| `[时间]` | 事件发生时间，默认格式带毫秒精度 |
| `[文件:行号]` | 日志调用点的源文件和行号，过长时自动截断头部 |
| `标签` | 日志的语义标签，用于路由匹配 |
| `上下文字符串` | `StringFromContext` 钩子提取的链路标识 |
| `key=value` 字段 | 结构化字段，按添加顺序输出 |

配置示例：

```properties
appender.console.layout.type = TextLayout
appender.console.layout.fileLineMaxLength = 48
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `fileLineMaxLength` | 否 | 文件路径最大显示长度，超过时头部以 `...` 截断，默认 `48` | `30`、`60` |

---

### JSONLayout

`JSONLayout` 输出标准单行 JSON，是生产环境的推荐配置。
它保留字段的原始类型信息，便于日志系统直接索引、聚合和检索。

配置示例：

```properties
appender.console.layout.type = JSONLayout
appender.console.layout.fileLineMaxLength = 48
```

| 配置项 | 必填 | 说明 | 示例 |
|--------|------|------|------|
| `fileLineMaxLength` | 否 | 文件路径最大显示长度，超过时头部以 `...` 截断，默认 `48` | `30`、`60` |

JSONLayout 没有使用标准库 `encoding/json` 的通用序列化，而是采用面向日志字段的专用编码器，
因此在性能和内存分配方面都有显著优势。

---

### 自定义 Layout 扩展

Go-Spring 日志的 Layout 是完全可插拔的。
需要其他格式（如 CSV、Protobuf、自定义分隔符）时，只需要实现 `Layout` 接口并注册到插件系统，就可以在配置中启用它了。

下面实现一个输出 CSV 格式的自定义 Layout：

```go
// CSVLayout 输出逗号分隔的 CSV 格式
type CSVLayout struct {
	log.BaseLayout
}

func (l *CSVLayout) EncodeTo(e *log.Event, w log.Writer) {
	// 转义 CSV 特殊字符
	escape := func(s string) string {
		if strings.ContainsAny(s, ",\"\n") {
			return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
		}
		return s
	}

	w.WriteString(e.Level.UpperName())
	w.WriteByte(',')
	w.WriteString(e.Time.Format("2006-01-02T15:04:05.000"))
	w.WriteByte(',')
	w.WriteString(escape(l.GetFileLine(e)))
	w.WriteByte(',')
	w.WriteString(e.Tag)
	w.WriteByte(',')
	if e.CtxString != "" {
		_, _ = w.WriteString(escape(e.CtxString))
		_, _ = w.WriteString(",")
	}

	var buf bytes.Buffer
	enc := log.NewTextEncoder(&buf, " ")
	enc.AppendEncoderBegin()
	log.EncodeFields(enc, e.CtxFields)
	log.EncodeFields(enc, e.Fields)
	enc.AppendEncoderEnd()

	w.WriteString(escape(buf.String()))
	w.WriteByte('\n')
}

func init() {
	log.RegisterPlugin[CSVLayout]("CSVLayout")
}
```

**配置使用示例：**

```properties
logger.console.type = ConsoleLogger
logger.console.tag = _app_*
logger.console.level = INFO
logger.console.layout.type = CSVLayout
```

---

## Encoder

Encoder 是字段编码层，负责把 `Field` 写成文本或 JSON 字节流。它的设计目标是：

- 基础类型走专门编码路径，减少反射
- 字段携带类型信息，编码时无需重新推断
- 直接写入目标 `Writer`，减少中间对象

通常业务代码不需要直接操作 Encoder。只有在实现自定义 Layout 时，才需要选择并组合 Encoder。

---

## 上下文提取

上下文提取是微服务日志系统的核心功能之一，用来回答"这条日志属于哪个请求或链路"的问题。

在分布式系统中，一个请求可能经过多个服务、多个组件，产生大量日志。
如果没有统一的链路标识，排查问题时根本无法把这些日志串联起来。
上下文提取的核心目标就是：**让每一条日志都能自动携带链路标识，无需业务代码手动传递**。

常见的上下文字段包括：

| 字段名 | 说明 | 典型场景 |
|--------|------|----------|
| `trace_id` | 全局链路追踪 ID，贯穿整个请求生命周期 | 分布式链路追踪 |
| `span_id` | 当前跨度 ID，标识单次调用或操作 | 细分链路阶段 |
| `request_id` | HTTP 请求 ID，由网关或入口层生成 | Web 服务排障 |
| `user_id` | 当前操作用户 ID | 用户行为审计 |
| `client_ip` | 客户端 IP 地址 | 安全分析和流量统计 |
| `tenant_id` | 多租户场景下的租户标识 | SaaS 系统隔离 |

如果每次日志调用都由业务代码手动传递这些字段，不仅重复繁琐，而且极易遗漏。
Go-Spring 提供了两个全局钩子函数，可以从 `context.Context` 中自动提取上下文信息，
并注入到每一条输出的日志中。

---

### FieldsFromContext

`FieldsFromContext` 钩子用于提取多个结构化字段，返回 `[]log.Field`。
提取出的字段会被注入到最终日志事件中，和业务代码添加的字段同等对待。

这是**推荐优先使用**的提取方式，因为它保留了字段的类型信息，便于日志系统直接索引和聚合。

#### 基础使用示例

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
	var fields []log.Field

	// 提取链路追踪字段
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		fields = append(fields, log.String("trace_id", traceID))
	}
	if spanID, ok := ctx.Value("span_id").(string); ok {
		fields = append(fields, log.String("span_id", spanID))
	}

	// 提取业务上下文字段
	if userID, ok := ctx.Value("user_id").(int64); ok {
		fields = append(fields, log.Int("user_id", userID))
	}
	if requestID, ok := ctx.Value("request_id").(string); ok {
		fields = append(fields, log.String("request_id", requestID))
	}

	return fields
}
```

设置完成后，业务代码只需要正常调用日志 API：

```go
// 业务代码不需要显式传递 trace_id、user_id 等字段
log.Info(ctx, TagBizOrder,
    log.String("order_no", "ORD001"),
    log.Msg("订单创建成功"),
)
```

最终输出的日志会自动包含上下文字段：

```json
{
    "level": "info",
    "time": "2026-05-03T11:27:16.214",
    "fileLine": ".../myapp/main.go:46",
    "tag": "_biz_order",
    "trace_id": "123456",
    "span_id": "123456",
    "request_id": "123456",
    "order_no": "ORD001",
    "msg": "订单创建成功"
}
```

#### 与 OpenTelemetry 集成示例

这是生产环境中常见的集成方式，直接从 OTel Context 中提取链路信息：

```go
import "go.opentelemetry.io/otel/trace"

log.FieldsFromContext = func(ctx context.Context) []log.Field {
	var fields []log.Field

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID()
		spanID := span.SpanContext().SpanID()

		fields = append(fields,
			log.String("trace_id", traceID.String()),
			log.String("span_id", spanID.String()),
		)

		// 如果 span 有错误标记，也可以提取出来
		if span.SpanContext().IsSampled() {
			fields = append(fields, log.Bool("sampled", true))
		}
	}

	return fields
}
```

---

### StringFromContext

`StringFromContext` 钩子用于提取一个已格式化好的字符串。
配合 `TextLayout` 使用时，需要符合 `TextLayout` 的格式要求；
配合 `JSONLayout` 使用时，需要符合 `JSONLayout` 的格式要求。

#### 使用示例

```go
type traceCtxType struct{}

log.StringFromContext = func(ctx context.Context) string {
	trace, _ := ctx.Value(traceCtxType{}).(string)
	return trace
}
```

### 性能注意事项

这两个钩子函数会在**每一次日志输出时执行**，因此必须严格控制它们的执行成本。
例如，可以把所有需要提取的值在请求入口处一次性放入 `ctx`，提取时只做简单的类型断言和读取。
同时，应该尽量避免在钩子内创建新对象，尤其是高频调用的场景。
如果钩子内有 `if` 逻辑，应该把最常见、最可能存在的字段放在前面。

以下是绝对禁止的操作：
- **禁止复杂计算**：不要在钩子内做字符串拼接、哈希计算、序列化等操作。
- **禁止网络或磁盘 IO**：绝对不要在钩子内调用外部接口或读取文件。
- **禁止锁操作**：不要在钩子内获取互斥锁或进行同步操作。
- **禁止反射**：不要通过反射遍历 Context 中的值。

---

## 配置系统

Go-Spring 日志系统采用**扁平化 KV 配置模型**。无论是配置文件、环境变量、配置中心还是命令行参数，
都可以统一建模，而且不同来源的配置可以按优先级进行合并。

### 配置分类

Go-Spring 日志系统的配置按命名空间分为三大类：

**1. `logger.*` — Logger 实例配置**

每个 Logger 实例在配置中以 `logger.<name>.` 为前缀。`<name>` 是自定义的实例名，用于标识不同的 Logger。
每个 Logger 必须配置 `type` 字段指定插件类型，如 `AsyncLogger`、`ConsoleLogger` 等。
除 root Logger 外，其他 Logger 必须配置 `tag` 字段指定绑定的标签表达式。
需要特别说明的是，root logger 不需要绑定任何标签，因为它是兜底的 Logger 实例。

**典型示例**：

```properties
logger.async.type = AsyncLogger
logger.async.tag = _app_*
logger.async.level = INFO
logger.async.appenderRef[0].ref = console
logger.async.appenderRef[1].ref = file
```

---

**2. `appender.*` — Appender 实例配置**

每个 Appender 实例在配置中以 `appender.<name>.` 为前缀。`<name>` 是自定义的实例名，供 Logger 的 `appenderRef` 引用。
每个 Appender 必须配置 `type` 字段指定插件类型，如 `ConsoleAppender`、`RollingFileAppender` 等。
每个 Appender 都可以内嵌自己的 Layout 配置。

**典型示例**：

```properties
appender.console.type = ConsoleAppender
appender.console.layout.type = TextLayout

appender.file.type = FileAppender
appender.file.dir = ./logs
appender.file.file = app.log
appender.file.layout.type = JSONLayout
```

---

**3. 全局变量和自定义属性**

没有命名空间前缀的配置项视为全局变量，可以通过 `${key}` 语法在其他配置中引用。
通过在配置中多处引用，可以做到修改一处、全局生效。

**典型示例**：

```properties
log.dir = /var/log/app
log.level = INFO
log.retention = 168h

appender.file.dir = ${log.dir}
appender.rolling.maxAge = ${log.retention}
logger.root.level = ${log.level}
```

---

### 日志级别

日志级别配置项 `level` 支持两种表达方式：
- 单个级别表示输出该级别及以上所有日志；
- 级别范围采用左闭右开区间 `[MinLevel, MaxLevel)`，用 `~` 分隔，仅输出此区间内的日志。

```properties
# 单个级别：输出 INFO 及以上
logger.root.level = INFO

# 范围：输出 WARN、ERROR、PANIC（不含 FATAL）
logger.error_only.level = WARN~FATAL

# 范围：输出 DEBUG、INFO（不含 WARN）
logger.debug_info.level = DEBUG~WARN
```

### 数组配置

配置项数组有两种配置方式，可以根据内容的复杂度进行选择：一种是索引方式，一种是逗号分隔。

#### 方式一：索引方式（通用）

这种方式适用于对象数组或复杂结构：

```properties
logger.root.appenderRef[0].ref = console
logger.root.appenderRef[0].level = DEBUG~WARN
logger.root.appenderRef[1].ref = file
logger.root.appenderRef[1].level = INFO~MAX
logger.root.appenderRef[2].ref = kafka
logger.root.appenderRef[2].level = ERROR~MAX
```

#### 方式二：逗号分隔（简单字符串）

这种方式适用于简单的字符串列表：

```properties
logger.biz.tag = _biz_order_*,_biz_user_*,_biz_pay_*
```

等价于：

```properties
logger.biz.tag[0]=_biz_order_*
logger.biz.tag[1]=_biz_user_*
logger.biz.tag[2]=_biz_pay_*
```

---

### 插件注入

Go-Spring 日志系统的所有核心组件（Logger、Appender、Layout）都是通过插件机制进行管理的。
插件机制统一封装了组件的配置解析、类型转换、实例创建和生命周期管理，使得日志系统具备高度的可扩展性。

插件配置通过 Struct Tag 声明式注入到插件实例，主要由两种标签实现：
- `PluginAttribute` 用于注入字符串、数值、布尔值、duration 等基础类型属性
- `PluginElement` 用于注入嵌套的插件实例，支持递归组合

这种设计使得插件作者只需要声明字段元数据，而无需手动编写任何配置解析代码。

---

#### 普通属性注入

`PluginAttribute` 用于注入字符串、数值、布尔值、duration 等基础类型属性。

**Tag 语法**：`属性名,default=默认值`

- `属性名`：必填，配置文件中对应的键名
- `default=默认值`：可选，未配置时使用的默认值

```go
type RollingFileAppender struct {
	log.AppenderBase

	FileDir  string        `PluginAttribute:"dir,default=./logs"`
	FileName string        `PluginAttribute:"file"` // 必填字段，无默认值
	Interval time.Duration `PluginAttribute:"interval,default=1h"`
	MaxAge   time.Duration `PluginAttribute:"maxAge,default=168h"`
	SyncLock bool          `PluginAttribute:"syncLock,default=false"`
}
```

Go 结构体字段名与配置键名保持一致，通常建议使用驼峰命名。

---

#### 子插件注入

`PluginElement` 用于注入嵌套的插件实例，支持递归组合。
子插件本身也可以包含 `PluginAttribute` 和其他 `PluginElement`。

**Tag 语法**：`子插件前缀,default=默认插件类型`

```go
type ConsoleLogger struct {
	log.LoggerBase

	Layout log.Layout `PluginElement:"layout,default=TextLayout"`
}
```

子插件的配置键自动加上 Tag 中指定的前缀，形成层级结构：

```properties
# 父插件 ConsoleLogger 的配置
logger.console.type = ConsoleLogger

# 子插件 Layout 的配置，自动加上 .layout 前缀
logger.console.layout.type = JSONLayout
logger.console.layout.fileLineMaxLength = 60
```

这种嵌套机制支持任意深度的插件组合，例如：
- Logger 内嵌 Appender 数组
- Appender 内嵌 Layout
- Layout 内嵌 Encoder

---

#### 插件注册 

插件在使用前都需要通过 `log.RegisterPlugin` 函数注册，这样配置系统才能根据类型名称创建实例。

```go
// 自定义插件类型
type SamplingAppender struct {
	log.FileAppender

	SampleRate float64 `PluginAttribute:"sampleRate,default=0.01"`
	rand       *rand.Rand
}

// 注册插件
func init() {
	log.RegisterPlugin[SamplingAppender]("SamplingAppender")
}
```

#### 生命周期管理

插件的生命周期由日志系统统一管理，贯穿配置加载到运行的全过程。
插件可以实现 `Start()` 和 `Stop()` 方法，
前者在配置注入完成后调用，用于初始化资源（如创建连接、初始化随机种子等），
后者在系统关闭时调用，用于优雅地释放资源（如关闭连接、刷写缓冲区等）。

```go
// Start 在配置注入完成后调用，用于初始化资源
func (l *SamplingLogger) Start() error {
	l.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return l.AsyncLogger.Start()
}

// Stop 在系统关闭时调用，用于清理资源
func (l *SamplingLogger) Stop() {
	l.AsyncLogger.Stop()
}
```

---

#### 类型转换器

如果插件需要自定义配置类型，可以使用类型转换器。它可以将配置字符串转换为目标类型。
Go-Spring 日志系统提供了 `log.RegisterConverter` 函数用于注册自定义转换器。

类型转换器要求必须是纯函数，没有副作用，相同的输入总是产生相同的输出。它的函数签名是：

```go
type Converter[T any] func(string) (T, error)
```

日志的配置级别 `LevelRange` 就是自定义类型。

---

## 错误处理

通常在遇到日志写入错误时，我们不应再次向文件或控制台写入日志，而是将错误报告给其他系统，触发告警。
日志系统的错误处理有其特殊性：当日志写入失败时，不能再次通过日志系统记录这个错误，
否则可能引发无限递归，加重系统负担，有时候甚至也不能写入标准错误流。

Go-Spring 采用**全局错误回调**的设计，通过 `log.ReportError` 函数统一上报写入错误。
该回调只会在 Event 写入失败时触发，不会在其他场景调用。

实现回调时，应避免在错误处理器中做耗时操作，也绝对不要在回调函数中产生 panic。

```go
log.ReportError = func(err error) {
	// 上报错误到其他系统，如告警系统
	metric.Incr("log_write_error_total")
}
```

---

## 配置刷新

Go-Spring 提供了 `log.Refresh` 和 `log.RefreshConfig` 两个函数用于刷新日志配置。
前者从 `flatten.Storage` 中读取配置，后者从扁平化 map 加载配置。

Go-Spring 应用框架使用 `log.Refresh` 刷新日志配置。
如果单独使用日志组件，建议使用 `log.RefreshConfig`。

```go
err := log.RefreshConfig(map[string]string{
	"appender.console.type":        "ConsoleAppender",
	"appender.console.layout.type": "TextLayout",

	"appender.file.type":        "FileAppender",
	"appender.file.dir":         "./logs",
	"appender.file.file":        "app.log",
	"appender.file.layout.type": "JSONLayout",

	"logger.sync.type":               "SyncLogger",
	"logger.sync.tag":                "_app_*",
	"logger.sync.level":              "INFO",
	"logger.sync.appenderRef[0].ref": "console",
	"logger.sync.appenderRef[1].ref": "file",
})
```

---

## 框架适配

除了直接使用 Go-Spring 的原生日志 API，项目中已有的日志入口也可以逐步接入这套日志系统。

### GetLogger

传统日志框架通常基于 name 区分 logger，例如以程序名命名的 logger 用于输出应用和业务日志，
以 rpc 命名的 logger 用于输出 RPC 调用日志。

Go-Spring 日志系统提供了基于 name 获取 logger 的 `log.GetLogger` 函数，适合第三方库适配或项目迁移。

```go
rootLogger := log.GetLogger("root")
rootLogger.Write(log.InfoLevel, []byte("hello world\n"))
```

我们需要在配置里定义同名 Logger，否则会报错。比如配合上述代码，我们可以提供下面的配置：

```properties
logger.root.type = FileLogger
logger.root.level = INFO
logger.root.dir = ./logs
logger.root.file = app.log
logger.root.layout.type = JSONLayout
logger.root.layout.fileLineMaxLength = 60
```

### 适配标准库 log

Go 标准库的 `log` 包是最基础也是使用最广泛的日志库，许多第三方依赖都使用它来输出日志。
将标准库 log 统一接入 Go-Spring 日志系统，可以实现全应用日志的集中管理，避免出现分散的日志输出。

标准库 log 提供了 `io.Writer` 作为输出扩展点，我们只需实现这个接口并通过 `SetOutput` 替换默认输出，
即可完成适配。

下面是完整的适配实现和使用示例：

```go
package main

import (
	"context"
	stdlog "log"

	"go-spring.org/log"
)

// StdLogWriter 实现 io.Writer 接口，将标准库 log 的输出转发到 Go-Spring 日志系统。
type StdLogWriter struct {
	logger *log.LoggerWrapper
}

// Write 实现 io.Writer 接口，将日志内容以 INFO 级别写入 Go-Spring Logger。
func (w *StdLogWriter) Write(p []byte) (int, error) {
	w.logger.Write(log.InfoLevel, p)
	return len(p), nil
}

func main() {
	// 替换标准库 log 的默认输出目标，所有后续通过 stdlog 输出的日志都会转发到 Go-Spring
	stdlog.SetOutput(&StdLogWriter{
		logger: log.GetLogger("root"),
	})

	// 初始化 Go-Spring 日志配置。
	err := log.RefreshConfig(map[string]string{
		"logger.root.type":                     "FileLogger",
		"logger.root.level":                    "INFO",
		"logger.root.dir":                      "./logs",
		"logger.root.file":                     "app.log",
		"logger.root.layout.type":              "JSONLayout",
		"logger.root.layout.fileLineMaxLength": "20",
	})
	if err != nil {
		stdlog.Printf("log refresh config error: %v", err)
		return
	}

	// 通过标准库 log 输出日志。
	stdlog.Println("hello from standard log")

	// 使用 Go-Spring 原生日志接口输出日志，验证两者可以共存。
	log.Info(context.Background(), log.TagAppDef, log.String("user", "alice"))
}
```

执行示例后，标准库 log 的输出会通过适配器转发到 Go-Spring 日志系统，最终写入 `./logs/app.log` 文件。
输出内容包含两种格式的日志：

```text
2026/05/03 22:27:11 hello from standard log
{"level":"info","time":"2026-05-03T22:27:11.584","fileLine":".../myapp/main.go:46","tag":"_app_def","user":"alice"}
```

可以看到，第一条是标准库 log 产生的纯文本日志，第二条是 Go-Spring 原生产生的结构化日志。
两种日志最终都汇聚到同一个输出目标，实现了日志的统一管理。

### 适配 Zap

与标准库 log 的适配思路相同，我们可以通过实现 Zap 的核心接口来完成适配。
Zap 框架提供了 `zapcore.Core` 接口作为日志写入的核心抽象。通过实现这个接口，
将 Zap 产生的日志事件转发到 Go-Spring 日志系统。

下面是完整的适配实现和使用示例：

```go
package main

import (
	"context"
	stdlog "log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go-spring.org/log"
)

// ZapGoSpringWriter 是一个 zapcore.Core 适配器，
// 用于将 Zap 日志转发到 Go-Spring 日志系统。
type ZapGoSpringWriter struct {
	logger  *log.LoggerWrapper
	fields  []zapcore.Field
	Encoder zapcore.Encoder
}

// NewZapGoSpringWriter 创建一个适配 Go-Spring Logger 的 Zap Core。
func NewZapGoSpringWriter(logger *log.LoggerWrapper) zapcore.Core {
	return &ZapGoSpringWriter{
		logger:  logger,
		Encoder: zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
	}
}

// Enabled 判断当前日志级别是否允许输出。
func (c *ZapGoSpringWriter) Enabled(level zapcore.Level) bool {
	// 委托给 Go-Spring 的级别判断
	return c.logger.Enable(toGoSpringLevel(level))
}

// With 追加结构化字段，并返回新的 Core 实例。
func (c *ZapGoSpringWriter) With(fields []zapcore.Field) zapcore.Core {
	clone := &ZapGoSpringWriter{
		logger:  c.logger,
		fields:  append(c.fields, fields...),
		Encoder: c.Encoder,
	}
	return clone
}

// Check 判断日志条目是否需要写入。
func (c *ZapGoSpringWriter) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// Write 编码 Zap 日志条目，并转发给 Go-Spring Logger。
func (c *ZapGoSpringWriter) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// 将 Zap 字段编码为文本格式
	buf, err := c.Encoder.EncodeEntry(entry, append(c.fields, fields...))
	if err != nil {
		log.ReportError(err) // 报告文件写入错误
		return err
	}

	// 转发到 Go-Spring Logger
	level := toGoSpringLevel(entry.Level)
	c.logger.Write(level, buf.Bytes())
	return nil
}

// Sync 刷新日志缓冲区。
func (c *ZapGoSpringWriter) Sync() error {
	// Go-Spring Logger 由自身系统管理，这里无需额外处理。
	return nil
}

// toGoSpringLevel 将 Zap 日志级别映射为 Go-Spring 日志级别。
func toGoSpringLevel(level zapcore.Level) log.Level {
	switch level {
	case zapcore.DebugLevel:
		return log.DebugLevel
	case zapcore.InfoLevel:
		return log.InfoLevel
	case zapcore.WarnLevel:
		return log.WarnLevel
	case zapcore.ErrorLevel:
		return log.ErrorLevel
	case zapcore.DPanicLevel:
		return log.PanicLevel
	case zapcore.PanicLevel:
		return log.PanicLevel
	case zapcore.FatalLevel:
		return log.FatalLevel
	default:
		return log.InfoLevel
	}
}

func main() {
	// 创建转发到 Go-Spring 的 Zap Core。
	core := NewZapGoSpringWriter(log.GetLogger("root"))

	// 基于自定义 Core 创建 Zap Logger。
	zapLogger := zap.New(core, zap.AddCaller())
	defer zapLogger.Sync()

	// 初始化 Go-Spring 日志配置。
	err := log.RefreshConfig(map[string]string{
		"logger.root.type":                     "FileLogger",
		"logger.root.level":                    "INFO",
		"logger.root.dir":                      "./logs",
		"logger.root.file":                     "app.log",
		"logger.root.layout.type":              "JSONLayout",
		"logger.root.layout.fileLineMaxLength": "20",
	})
	if err != nil {
		stdlog.Printf("log refresh config error: %v", err)
		return
	}

	// 使用 Zap 输出日志，最终写入 Go-Spring 日志系统。
	zapLogger.Info("zap info message",
		zap.String("user", "alice"),
		zap.Int64("order_id", 10001),
	)

	zapLogger.Warn("zap warn message",
		zap.String("action", "retry"),
		zap.Int("attempt", 3),
	)

	zapLogger.Error("zap error message",
		zap.Error(os.ErrNotExist),
	)

	// 使用 Go-Spring 原生日志接口输出日志，验证两者可以共存。
	log.Info(context.Background(), log.TagAppDef, log.String("user", "alice"))
}
```

执行上述示例后，Zap 产生的日志会通过适配器转发到 Go-Spring 日志系统，最终写入 `./logs/app.log` 文件。
输出内容包含两种格式的日志：

```text
{"level":"info","ts":1777816204.077995,"caller":"myapp/main.go:116","msg":"zap info message","user":"alice","order_id":10001}
{"level":"warn","ts":1777816204.078231,"caller":"myapp/main.go:121","msg":"zap warn message","action":"retry","attempt":3}
{"level":"error","ts":1777816204.078253,"caller":"myapp/main.go:126","msg":"zap error message","error":"file does not exist"}
{"level":"info","time":"2026-05-03T21:50:04.078","fileLine":"...myapp/main.go:131","tag":"_app_def","user":"alice"}
```

可以看到，前三条是 Zap 产生的日志（使用 Zap 自身的 JSON 格式），最后一条是 Go-Spring 原生日志。
两种日志最终都汇聚到同一个输出目标，实现了日志系统的统一管理。
