# Go-Spring 实战第 21 课 —— 日志路由与调度：Tag 与 Logger

上一篇咱们介绍了结构化日志的构造过程。业务代码通过 API 提供 Level、Tag 和 Field，Go-Spring 再补充时间、调用位置和上下文信息，最后把一次日志调用整理成统一的 Event。

不过，一次日志调用并不是一上来就完整构造 Event。Go-Spring 要先通过 Tag 找到 Logger，再由 Logger 判断当前级别是否启用。只有判断通过以后，Event 才会被完整构造，并交给 Logger 继续处理。所以本篇咱们沿着这条流程，重点看清楚两个问题：Logger 从哪里来？拿到 Logger 以后，它又会怎样处理 Event？

## Tag

业务代码调用日志 API 时，并没有直接传入 Logger，而是传入了一个 Tag。Go-Spring 正是通过这个 Tag，找到当前日志对应的 Logger。所以，要回答 Logger 从哪里来，咱们得先从 Tag 说起。

Tag 是业务代码和日志配置之间的稳定标识。它描述的是日志事件的业务语义，而不是调用代码所在的 Go 包，也不对应某个具体的输出目标。这样一来，即使代码位置发生调整，或者日志从控制台改到了文件，业务代码使用的 Tag 也不需要跟着变化。

比如，订单创建和订单查询即使位于同一个包，也可以使用不同的 Tag。同一个订单 Tag，也可以被不同层的代码共同使用。业务代码只负责选择合适的 Tag，至于日志最终写到哪里、怎样写入，则交给配置决定。

### Tag 类型

为了覆盖常见场景，Go-Spring 预定义了三类 Tag：

| 注册函数 | Tag 前缀 | 适用场景 |
|----------|----------|----------|
| `RegisterAppTag` | `_app_` | 应用启动、关闭、健康检查等应用行为 |
| `RegisterBizTag` | `_biz_` | 订单、支付、用户等业务行为 |
| `RegisterRPCTag` | `_rpc_` | HTTP、gRPC、Redis 等外部或内部依赖调用 |

```go
var (
	TagAppStartup = log.RegisterAppTag("startup", "")
	TagBizCreate  = log.RegisterBizTag("order", "create")
	TagRPCGet     = log.RegisterRPCTag("redis", "get")
)
```

上面的注册分别会得到 `_app_startup`、`_biz_order_create` 和 `_rpc_redis_get`。如果这三种预定义语义都不合适，我们也可以通过 `BuildTag` 和 `RegisterTag` 注册自定义分类。

### Tag 注册

注册函数不只是构造一个 Tag 名称，还会把 Tag 加入日志系统的注册表。等配置刷新时，Go-Spring 会遍历这些已经注册的 Tag，并为它们绑定 Logger。

因此，Tag 通常要声明为包级变量，在应用初始化阶段完成注册。第一次配置刷新以后，就不能再注册新的 Tag 了，也不能在请求处理过程中动态创建。

### Logger 绑定

Tag 注册完成以后，Logger 就可以通过配置声明自己要处理哪些 Tag。日志配置刷新时，Go-Spring 会根据 Logger 的 `tag` 配置，为每个已注册的 Tag 选择对应的 Logger。

Logger 的 `tag` 既可以精确匹配，也可以使用末尾通配符匹配一组标签。比如 `_biz_order_create`，会按照下面的顺序查找：

```text
_biz_order_create
_biz_order_*
_biz_*
Root Logger
```

规则越具体，优先级就越高。如果没有配置精确标签，Go-Spring 会逐级尝试 `_*` 通配规则。如果所有规则都没有匹配，最终就会使用 Root Logger。

看个配置示例：

```properties
logging.logger.root.type = ConsoleLogger
logging.logger.root.level = INFO

logging.logger.order.type = RollingFileLogger
logging.logger.order.tag = _biz_order_*
logging.logger.order.level = INFO
logging.logger.order.dir = .
logging.logger.order.file = order.log
logging.logger.order.layout.type = JSONLayout
```

在上面的配置中，`_biz_order_create` 和 `_biz_order_query` 都会绑定到 `order` Logger，其他 Tag 则使用 Root Logger。

需要注意的是，这个匹配过程只在配置刷新时执行。等绑定完成以后，业务代码每次记录日志时就不需要再临时扫描配置了。

## Logger

通过 Tag 找到 Logger 以后，接下来就要看 Logger 怎样处理一次日志调用了。这个过程可以分成两步：先判断当前级别是否启用，判断通过以后，再根据 Logger 的具体类型调度和分发 Event。

### 日志过滤

第一次级别判断发生在日志 API 的入口处。比如 Logger 只启用了 `INFO` 及以上级别，那么 `TRACE` 和 `DEBUG` 调用会直接结束，不再创建 Event。只有判断通过以后，API 才会继续补充时间、调用位置和上下文等信息。

这样设计的原因很直接：既然一条日志最终不会被处理，就没有必要再为它构造完整的 Event。

Go-Spring 的级别范围采用左闭右开区间。只配置一个级别时，表示从该级别一直到 `MAX`。使用 `~` 时，则可以明确指定不包含在范围内的上边界。

```properties
# INFO 及以上级别
logging.logger.root.level = INFO

# DEBUG、INFO，不包含 WARN
logging.logger.debug.level = DEBUG~WARN

# WARN、ERROR、PANIC，不包含 FATAL
logging.logger.warning.level = WARN~FATAL
```

`NONE` 和 `MAX` 主要用于表达范围边界。如果内置级别无法表达特殊语义，我们也可以注册自定义 Level，它仍然使用同一套过滤规则。

### 日志类型

通过级别过滤以后，Event 就真正进入 Logger 的处理流程了。不同 Logger 的主要区别，在于怎样把 Event 送入后续输出阶段。按照 Logger 与 Appender 的组织方式，我们可以把它们分为组合式和集成式两类。

组合式 Logger 包括 `SyncLogger` 和 `AsyncLogger`。它们负责同步或异步调度，再通过 `AppenderRef` 引用独立配置的 Appender。这种方式适合组合多个输出目标，或者分别控制每个目标接收的日志级别。

集成式 Logger 包括 `ConsoleLogger`、`FileLogger` 和 `RollingFileLogger`。它们把 Logger 与常用 Appender 组合在一起，配置起来会更简单。除此之外，还有一个 `DiscardLogger`，它会直接结束处理，不产生任何输出。

### SyncLogger

咱们先看组合式 Logger 的两种调度方式。`SyncLogger` 会在当前记录日志的 goroutine 中，依次调用各个 Appender。

```properties
logging.logger.root.type = SyncLogger
logging.logger.root.level = INFO
logging.logger.root.appenderRef[0].ref = console
logging.logger.root.appenderRef[1].ref = file
```

在上面的配置中，Event 会依次交给 `console` 和 `file`。因为使用的是同步模式，所以格式编码、写入和锁等待都发生在业务 goroutine 中。如果某个输出目标比较慢，记录日志的调用耗时也会随之增加。

### AsyncLogger

如果我们不希望后续输出过程一直占用业务 goroutine，就可以使用 `AsyncLogger`。

```properties
logging.logger.biz.type = AsyncLogger
logging.logger.biz.tag = _biz_*
logging.logger.biz.level = INFO
logging.logger.biz.bufferSize = 10000
logging.logger.biz.onBufferFull = discard
logging.logger.biz.appenderRef[0].ref = file
```

`AsyncLogger` 会先把 Event 放入缓冲区，再由后台 goroutine 顺序调用 Appender。这样一来，业务 goroutine 通常只需要完成入队，就可以继续执行后面的逻辑了。

不过，异步调度也带来了一个新问题：如果日志产生得太快，缓冲区满了该怎么办？Go-Spring 提供了三种处理策略：

| 策略 | 行为 |
|------|------|
| `block` | 等待缓冲区出现空位 |
| `discard` | 丢弃当前 Event |
| `drop-oldest` | 丢弃缓冲区中较早的 Event |

`block` 更关注日志完整性，但可能增加业务延迟。`discard` 优先保证请求不被阻塞。`drop-oldest` 则更倾向于保留最近的现场。实际使用时，我们需要根据日志价值和业务流量来选择。

应用正常关闭时，`AsyncLogger` 会继续处理已经入队的 Event。不过，如果进程被直接终止，缓冲区中还没来得及处理的日志仍然可能丢失。

### AppenderRef

`SyncLogger` 和 `AsyncLogger` 只决定怎样调度 Event，至于 Event 具体要交给哪些输出目标，则由 `AppenderRef` 决定。一个组合式 Logger 可以引用多个 Appender，还可以为每个引用设置局部级别范围。

```properties
logging.logger.biz.appenderRef[0].ref = console
logging.logger.biz.appenderRef[0].level = INFO~WARN

logging.logger.biz.appenderRef[1].ref = file
logging.logger.biz.appenderRef[1].level = WARN~MAX
```

上面的配置会把 `INFO` 交给控制台，把 `WARN` 及以上级别交给文件。也就是说，同一个 Logger 可以根据级别，把 Event 分发到不同的输出目标。

不过，`AppenderRef` 只负责描述引用关系和局部过滤。具体的输出位置与 Layout，仍然由 `appender.*` 配置决定。

### 集成式 Logger

组合式 Logger 把调度和输出目标拆开，换来了更灵活的组合能力。不过，它也要求我们分别声明 Logger、Appender 和 `AppenderRef`。对于控制台、单文件和滚动文件这些常见的固定组合，这样的配置就显得有些重了。

因此，Go-Spring 又提供了集成式 Logger。它把常见组合收进同一个配置对象，让 Logger 直接拥有相应的输出能力。

| Logger | 内部能力 |
|--------|----------|
| `ConsoleLogger` | 内部使用 ConsoleAppender |
| `FileLogger` | 内部使用 FileAppender |
| `RollingFileLogger` | 内部组合滚动文件、级别分离以及同步或异步调度 |

`ConsoleLogger` 和 `FileLogger` 分别覆盖控制台与单文件输出，适合目标明确、不需要复用 Appender 的场景。`RollingFileLogger` 则进一步把文件滚动、历史保留、级别分离和调度方式组合起来，更适合长期运行的服务。

```properties
logging.logger.biz.type = RollingFileLogger
logging.logger.biz.tag = _biz_*
logging.logger.biz.level = INFO
logging.logger.biz.dir = ./logs
logging.logger.biz.file = biz.log
logging.logger.biz.interval = 1h
logging.logger.biz.maxAge = 168h
logging.logger.biz.separate = true
logging.logger.biz.async = true
logging.logger.biz.bufferSize = 10000
logging.logger.biz.onBufferFull = discard
logging.logger.biz.layout.type = JSONLayout
```

这段配置不需要单独声明 `RollingFileAppender` 和 `AppenderRef`，但仍然完成了级别过滤、异步调度、文件滚动和 JSON 输出。开启 `separate` 以后，`WARN` 及以上日志还会被分离到单独的文件里，方便我们优先查看异常信息。

集成式 Logger 并没有改变底层处理模型，它只是封装了常见组合，让配置更简单。当一个 Logger 需要连接多个 Appender，或者每个 Appender 需要不同的级别范围时，我们仍然应该使用组合式 Logger。如果完全不需要某一类日志，则可以使用 `DiscardLogger`，让它直接结束处理。

## 日志路由与调度

到这里，Tag 和 Logger 的职责就比较清楚了。Tag 通过预先绑定，决定一条日志应该交给谁处理。Logger 则负责级别过滤，并按照同步或异步方式调度 Event。

换句话说，Tag 负责路由，Logger 负责调度。等 Event 离开 Logger、进入 Appender 以后，日志处理流程也就来到了真正的输出阶段。
