# Go-Spring 实战第 21 课：日志系统架构：从业务调用到 Logger、Appender 和 Encoder

Go-Spring 应用能启动、能关闭，只是生命周期的基础。真正跑起来以后，我们还需要持续理解它正在发生什么。这个时候，日志就从“打印几行信息”变成了工程治理的一部分。

Go-Spring 的日志系统很容易被低估为几个 `Infof` 加一个文件输出。但真实应用里的日志要解决更多问题：日志分类、级别过滤、结构化字段、上下文提取、异步写入、多目标输出、格式化和配置刷新。

如果这些能力没有架构约束，日志代码会很快变成一组难以治理的全局开关。所以 Go-Spring 日志系统用了一套基于标签路由的分层架构，把这些职责拆成可组合组件。

## 先看一次完整日志调用

日志调用以标签和字段为核心：

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

格式化日志适合简单提示；结构化字段适合业务事件和线上检索。后面会单独展开结构化字段，这里我们先把整体链路看清楚。

## 日志链路拆成六层

Go-Spring 日志链路分为六层：

```text
应用层 API
  -> 标签层 Tag
  -> Logger 层
  -> Appender 层
  -> Layout 层
  -> Encoder 层
```

应用层 API 面向业务代码，提供了 `Trace`、`Debug`、`Info`、`Warn`、`Error`、`Record` 等入口。

标签层负责日志语义分类和路由匹配。业务代码持有 `*log.Tag`，不直接关心底层 Logger。

Logger 层负责级别过滤和事件分发。同步、异步、控制台、文件、滚动文件等写入策略都在这里表达。

Appender 层负责落地到具体目标，例如控制台、本地文件、滚动文件或自定义远端系统。

Layout 层决定输出格式，例如面向人类的文本格式和面向机器的 JSON 格式。

Encoder 层负责字段编码，尽量减少反射和中间对象。

这些层次分开以后，我们就可以独立调整路由、过滤、落地目标和输出格式，而不用把所有逻辑塞进一个 Logger 实现里。

## 标签按业务语义路由

标签回答的其实是“这条日志是什么性质”，而不是“这条日志来自哪个包”。这样比传统按包名路由更贴近业务语义。

官方推荐前缀：

| 前缀 | 场景 | 示例 |
|------|------|------|
| `_app_` | 应用生命周期与基础设施 | `_app_startup` |
| `_biz_` | 业务流程与领域事件 | `_biz_order_create` |
| `_rpc_` | 外部依赖调用 | `_rpc_redis_get` |
| `_infra_` | 框架与中间件内部 | `_infra_pool_exhausted` |

标签通常在包初始化阶段注册。下面几个辅助函数会生成带规范前缀的标签，最后一个 `RegisterTag` 留给自定义标签：

```go
log.RegisterAppTag("startup", "")      // _app_startup
log.RegisterBizTag("order", "create") // _biz_order_create
log.RegisterRPCTag("redis", "get")    // _rpc_redis_get
log.RegisterTag("_cache_hit")         // 自定义标签
```

标签路由用的是精确优先、最长优先。例如 `_biz_order_create` 会依次匹配：

1. `_biz_order_create`
2. `_biz_order_*`
3. `_biz_*`
4. `logger.root`

这样就可以做到大类使用通用配置，小类覆盖特殊策略。

## 级别决定日志是否进入管线

Go-Spring 内置级别包括 `TRACE`、`DEBUG`、`INFO`、`WARN`、`ERROR`、`PANIC`、`FATAL`，并提供了 `NONE` 和 `MAX` 作为范围边界。

如果内置级别不够表达业务语义，也可以注册自定义级别。下面的 `AUDIT` 用来审计事件，并通过 `Record` 显式传入级别：

```go
var AuditLevel = log.RegisterLevel(350, "AUDIT")
var TagBizAudit = log.RegisterBizTag("audit", "record")

log.Record(ctx, AuditLevel, TagBizAudit, 2,
	log.String("user_id", "10086"),
	log.String("action", "modify_password"),
)
```

级别配置既支持单个级别，也支持范围，后续日志配置文章会展开。

## 业务代码有格式化和结构化两类入口

格式化日志适合简短提示，主要价值是给人快速阅读：

```go
log.Infof(ctx, TagAppStartup, "应用启动成功，版本: %s", "v1.0.0")
log.Warnf(ctx, TagBizUser, "用户 %s 密码错误", "bob")
```

结构化日志适合线上业务事件，字段会被日志平台索引和聚合：

```go
log.Info(ctx, TagBizOrder,
	log.Int("order_id", orderID),
	log.String("status", "created"),
	log.Msg("订单创建完成"),
)
```

如果是线上业务日志，建议优先结构化，便于日志平台索引、聚合和检索。

## 低级别日志要避免无效计算

`Trace` 和 `Debug` 强制使用惰性求值，避免级别关闭时仍然执行昂贵计算：

```go
log.Debug(ctx, TagBizOrder, func() []log.Field {
	return []log.Field{
		log.Any("stats", calculateExpensiveStats()),
		log.Msg("调试信息"),
	}
})
```

如果要封装日志工具函数，可以用 `Record` 的 `skip` 参数调整调用栈深度：

```go
func Audit(ctx context.Context, tag *log.Tag, fields ...log.Field) {
	log.Record(ctx, AuditLevel, tag, 3, fields...)
}
```

## 日志骨架先立住

标签路由、Logger、Appender、Layout、Encoder 这些层次先分清楚，后面的结构化字段、上下文提取、异步输出和配置刷新才不会挤在同一个抽象里。

骨架立住以后，业务代码最先接触到的就是字段模型：基础类型、指针、消息字段、数组、对象、Map 展开和 `Any` 分别适合不同日志内容。
