# Go-Spring 实战第 20 课 —— 日志架构：标签路由与输出管线

应用进入运行期以后，问题会从“能不能启动”变成“发生了什么、发生在哪里、能不能检索出来”。这时候日志不再只是几行提示文本，而是线上排查、容量评估和业务审计都会依赖的观测入口。

真实项目里的日志很快会变复杂。应用生命周期日志、业务事件日志、外部依赖日志和框架内部日志需要不同级别、不同格式、不同输出目标；同一条业务事件还可能同时写控制台、文件和远端采集系统。

如果这些判断都塞进业务调用点，日志代码会变成一组难以治理的全局开关。Go-Spring 日志系统的处理方式是把“事件语义”和“输出策略”拆开：业务代码声明标签、级别和字段，日志系统根据标签找到 Logger，再由 Logger、Appender、Layout 和 Encoder 完成过滤、调度和落地。

## 调用点只声明标签和字段

从业务代码看，Go-Spring 日志调用最先需要表达的是标签和字段。标签说明日志事件属于哪类语义，字段说明这条事件携带哪些结构化信息。至于写到哪里、用什么格式、是否异步，这些都应该留给日志配置和输出管线处理。

下面这个例子里，启动日志和订单日志走了不同标签，但调用点都只声明事件语义和事件字段。

```go
var (
	TagAppStartup = log.RegisterAppTag("startup", "")
	TagBizOrder   = log.RegisterBizTag("order", "create")
)

func main() {
	_ = log.RefreshConfig(map[string]string{
		"logger.root.type":  "ConsoleLogger",
		"logger.root.level": "INFO",
	})

	ctx := context.Background()

	log.Infof(ctx, TagAppStartup, "应用启动成功，版本: %s", "v1.0.0")

	log.Info(ctx, TagBizOrder,
		log.Int("order_id", 10001),
		log.String("user", "alice"),
		log.Float("amount", 99.99),
		log.Msg("订单创建成功"),
	)
}
```

这段代码里，业务代码没有指定写控制台还是文件，也没有关心 JSON 还是文本格式。`TagAppStartup` 和 `TagBizOrder` 只是语义入口，Go-Spring 会在配置刷新后把标签绑定到具体 Logger。格式化日志适合简单提示，结构化字段适合业务事件和线上检索，但它们都会进入同一条路由和输出链路。

## 六层职责

有了调用点上的标签和字段，Go-Spring 接下来要做的是把一条日志事件稳定送到输出目标。这个过程被拆成六层职责，每一层只回答一个问题。

```text
应用层 API
  -> 标签层 Tag
  -> Logger 层
  -> Appender 层
  -> Layout 层
  -> Encoder 层
```

应用层 API 面向业务代码，提供 `Trace`、`Debug`、`Info`、`Warn`、`Error`、`Record` 等入口。

标签层负责日志语义分类和路由匹配。业务代码持有 `*log.Tag`，不直接关心底层 Logger。

Logger 层负责级别过滤和事件分发。同步、异步、控制台、文件、滚动文件等写入策略都在这一层表达。

Appender 层负责落地到具体目标，例如控制台、本地文件、滚动文件或自定义远端系统。

Layout 层决定输出格式，例如面向人类的文本格式和面向机器的 JSON 格式。

Encoder 层负责字段编码，尽量减少反射和中间对象。

这些层次分开以后，路由、过滤、落地目标和输出格式都可以独立调整。后续扩展某一层时，影响面也更容易控制。

## 标签路由

标签回答的是“这条日志是什么性质”，而不是“这条日志来自哪个包”。这让日志配置更贴近工程治理场景。例如订单创建、Redis 调用、应用启动分别属于不同语义，即使它们出现在同一个包里，也可能需要不同输出策略。

Go-Spring 推荐使用下面这些前缀来区分常见日志类别。

| 前缀 | 场景 | 示例 |
|------|------|------|
| `_app_` | 应用生命周期与基础设施 | `_app_startup` |
| `_biz_` | 业务流程与领域事件 | `_biz_order_create` |
| `_rpc_` | 外部依赖调用 | `_rpc_redis_get` |
| `_infra_` | 框架与中间件内部 | `_infra_pool_exhausted` |

标签通常在包初始化阶段注册。下面几个辅助函数会生成带规范前缀的标签，最后一个 `RegisterTag` 留给自定义标签。

```go
log.RegisterAppTag("startup", "")      // _app_startup
log.RegisterBizTag("order", "create") // _biz_order_create
log.RegisterRPCTag("redis", "get")    // _rpc_redis_get
log.RegisterTag("_cache_hit")         // 自定义标签
```

路由匹配使用精确优先、最长优先。例如 `_biz_order_create` 会依次尝试下面这些规则。

1. `_biz_order_create`
2. `_biz_order_*`
3. `_biz_*`
4. `logger.root`

这样，大类可以使用通用配置，小类也可以覆盖特殊策略。业务代码只持有标签，具体走哪个 Logger 由配置决定。

## 级别先决定日志能不能进入管线

标签决定走哪条路由，级别先决定这条日志能不能进入管线。Go-Spring 内置级别包括 `TRACE`、`DEBUG`、`INFO`、`WARN`、`ERROR`、`PANIC`、`FATAL`，并提供 `NONE` 和 `MAX` 作为范围边界。

如果内置级别不够表达业务语义，也可以注册自定义级别。下面的 `AUDIT` 用来表达审计事件，并通过 `Record` 显式传入级别。

```go
var AuditLevel = log.RegisterLevel(350, "AUDIT")
var TagBizAudit = log.RegisterBizTag("audit", "record")

log.Record(ctx, AuditLevel, TagBizAudit, 2,
	log.String("user_id", "10086"),
	log.String("action", "modify_password"),
)
```

自定义级别进入同一套级别过滤和路由流程。级别配置既支持单个级别，也支持范围，后续日志配置文章会展开。

## 业务代码按事件价值选择入口

Go-Spring 提供格式化和结构化两类日志入口。格式化日志适合简短提示，主要价值是让人快速阅读。

```go
log.Infof(ctx, TagAppStartup, "应用启动成功，版本: %s", "v1.0.0")
log.Warnf(ctx, TagBizOrder, "订单 %s 状态异常", orderNo)
```

结构化日志适合线上业务事件，字段会被日志平台索引和聚合。

```go
log.Info(ctx, TagBizOrder,
	log.Int("order_id", orderID),
	log.String("status", "created"),
	log.Msg("订单创建完成"),
)
```

这两种入口的语义不同。格式化入口会把内容收进 `msg`，便于人直接阅读；结构化入口会把订单号、状态、耗时、用户 ID 等信息保留成字段，便于日志平台按字段检索和聚合。因此线上业务事件通常优先结构化，启动提示和临时排障信息可以继续使用格式化文本。

## 低级别惰性求值

`Trace` 和 `Debug` 经常在生产环境关闭。如果参数在调用前就完成计算，即使日志最终被级别过滤掉，业务路径也已经付出了成本。因此 Go-Spring 要求低级别日志使用惰性求值。

```go
log.Debug(ctx, TagBizOrder, func() []log.Field {
	return []log.Field{
		log.Any("stats", calculateExpensiveStats()),
		log.Msg("调试信息"),
	}
})
```

这段写法把低级别日志的计算边界放到了 Go-Spring 的级别判断之后。只有当这条 Debug 日志通过级别判断后，函数才会执行。这样低级别日志可以保留在代码里，但不会在关闭状态下持续消耗业务路径。

如果要封装日志工具函数，可以用 `Record` 的 `skip` 参数调整调用栈深度。

```go
func Audit(ctx context.Context, tag *log.Tag, fields ...log.Field) {
	log.Record(ctx, AuditLevel, tag, 3, fields...)
}
```

`skip` 的意义是让最终输出的调用位置指向业务调用点，而不是封装函数本身。

## 标签路由与输出管线

标签路由、Logger、Appender、Layout、Encoder 这些层次先分清楚，结构化字段、上下文提取、异步输出和配置刷新才有各自的位置。

这套架构的价值不是让概念变多，而是让每个概念只承担一类责任。业务代码稳定地面对标签、级别和字段；Go-Spring 把路由、过滤、调度、格式和编码放到独立层次中处理。后续无论调整输出格式、接入滚动文件，还是补充上下文字段，都不需要把这些差异重新挤回业务调用点。
