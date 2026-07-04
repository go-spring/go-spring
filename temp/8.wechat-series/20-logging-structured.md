# Go-Spring 实战第 20 课 —— 结构化日志：API、Field 与 Event

上一篇咱们介绍了 Go-Spring 日志系统的分层架构，梳理了日志从业务代码出发，进入应用层 API，然后根据 Tag 找到 Logger，最后经过 Appender、Layout 和 Encoder 完成输出的全过程。在这个过程中，API 承担着入口和转换的作用。

我们知道，记录日志并不是输出一段文本就完了。除了日志内容，我们还需要说明事件的严重程度、业务分类以及其他结构化属性。另外，还需要补充时间、调用位置等公共信息。Go-Spring 把这些信息整理成统一的 Event，然后交给后续组件进行处理。

本篇咱们就来看看，业务代码是怎样通过 API 和 Field 描述一条日志，以及这些信息最终又是怎样汇集成 Event 的。

## 日志 API

Go-Spring 提供了从 `Trace`、`Debug`、`Info`、`Warn`、`Error` 到 `Panic`、`Fatal` 的完整日志 API，分别对应了常见的日志级别：

- `TRACE`、`DEBUG` 用于开发和排障信息。
- `INFO` 用于正常业务过程和状态变化。
- `WARN` 用于需要关注但仍可继续处理的异常情况。
- `ERROR` 用于当前操作已经失败的情况。
- `PANIC`、`FATAL` 用于表达更严重的错误语义。

在调用日志 API 时，业务代码主要提供 Context、Tag 和日志内容三类信息。其中 Context 用于链路追踪，Tag 用于分类，日志内容用于描述事件。

大家最熟悉的写法应该是使用以 `f` 结尾的格式化接口，因为它们和 `fmt.Sprintf` 的语法非常相似。

```go
log.Infof(ctx, TagAppStartup, "应用启动完成，版本: %s", version)
log.Warnf(ctx, TagBizOrder, "订单 %s 状态异常", orderNo)
log.Errorf(ctx, TagRPCPayment, "支付请求失败: %v", err)
```

然而，这些格式化接口会将结果放入固定的 `msg` 字段，其中的业务属性无法作为独立字段进行检索。因此，这种写法虽然直接，但是只适合用于启动提示、开发排障和信息量较少的日志。

对于需要记录丰富信息的日志，比如包含订单号、用户 ID、状态码和耗时等信息的日志，如果将所有内容没有章法地拼进一段字符串里，那么在日志平台进行检索的时候就只能进行全文检索了。如果我们想要根据订单号进行过滤，或者按照状态码进行聚合，就需要重新解析文本，那样不仅麻烦，而且成本很高。

所以，如果日志中包含需要检索和分析的业务属性，我们强烈建议使用结构化接口。

```go
log.Info(ctx, TagBizOrder,
	log.String("order_no", orderNo),
	log.Int("user_id", userID),
	log.String("status", "created"),
	log.Msg("订单创建完成"),
)
```

上面的代码记录了 `order_no`、`user_id`、`status` 和 `msg` 四个独立的字段。它们既可以被 TextLayout 输出成便于阅读的文本，也可以被 JSONLayout 输出为结构化数据，甚至可以自定义成任何想要的格式。

> **需要注意的是**，级别只能描述事件的严重程度。也就是说，调用 `log.Panic` 时不会自动触发 panic，调用 `log.Fatal` 时也不会自动退出进程。错误返回、panic 和进程退出仍然需要由业务代码进行明确处理。

## Field

Field 是 Go 主流日志库普遍采用的一种设计。它将日志内容拆分为多个可以独立编码的字段，每个 Field 都包含字段名、字段类型和字段值。这样，写日志就不再是拼接一段文本，而是记录一组结构化字段。

在输出日志时，Encoder 会按照 Field 的类型进行编码。这种强类型设计可以尽量减少反射的使用，极大地提高日志序列化时的效率。所以，我们应该优先使用强类型函数。

```go
log.Bool("success", true)
log.Int("user_id", 10001)
log.Uint("bytes", uint64(4096))
log.Float("amount", 99.99)
log.String("order_no", "ORD001")
```

`Msg` 是一个特殊的 Field，常用来对日志进行摘要或者总结，它的 Key 固定为 `msg`。我们推荐每条日志都包含这个 Field。

```go
log.Msg("订单创建完成")
log.Msgf("处理了 %d 条记录", total)
```

为了兼顾检索和阅读性，我们可以把需要检索的属性拆成独立的 Field，然后用 `Msg` 留下一句方便人阅读的摘要。这样，日志就既保留了方便机器处理的能力，也保留了方便阅读的体验。

Field 支持基础类型、指针、数组和嵌套对象等常见数据类型，Go-Spring 为它们分别提供了相应的 Field 函数。

```go
var remark *string
var retryCount *int

log.StringPtr("remark", remark)
log.IntPtr("retry_count", retryCount)
log.Nil("deleted_at")

log.Ints("item_ids", []int64{1001, 1002, 1003})
log.Strings("tags", []string{"new_user", "coupon"})

log.Object("order",
	log.String("order_no", "ORD001"),
	log.Float("amount", 99.99),
	log.Object("item",
		log.String("sku", "SKU001"),
		log.Int("quantity", 2),
	),
)
```

当调用点不能确定具体类型时，可以使用 `Any`。

```go
log.Any("result", result)
```

`Any` 会先识别常见的基础类型、指针和数组，无法识别时再回退到反射编码。要注意的是，`Any` 只能作为兼容入口，不应该代替所有强类型字段函数。

对于动态字段集合，Go-Spring 提供了 `FieldsFromMap`，方便展开 `map` 中的每一项。

```go
attrs := map[string]any{
	"region":  "cn",
	"attempt": 2,
	"success": true,
}

log.Info(ctx, tag,
	log.FieldsFromMap(attrs),
	log.Msg("任务执行完成"),
)
```

上面的代码会记录 `region`、`attempt` 和 `success` 三个独立的 Field。因为这种写法的值类型不能确定，所以效率比强类型写法低。

## Event

在业务代码通过 API 和 Field 描述完当前日志以后，Go-Spring 会把相关信息整理成统一的 Event。

```go
type Event struct {
	Level     Level     // 日志级别
	Time      time.Time // 日志时间戳
	File      string    // 日志调用文件路径
	Line      int       // 日志调用行号
	Tag       string    // 日志标签名
	Fields    []Field   // 日志字段列表
	CtxString string    // 上下文字符串表示
	CtxFields []Field   // 上下文字段列表
}
```

Event 的作用是把一次日志调用转换成后续组件都能理解的统一对象。这样，Logger 不需要区分业务代码调用的是 `Infof` 还是 `Info`，Layout 也不需要重新读取调用栈或者 Context。这些信息都被组装在 Event 中了。

## 惰性构造

从实践经验来看，`TRACE` 和 `DEBUG` 日志在生产环境中经常被关闭，而这些日志的字段又常常需要复杂的计算。如果采用通常的 API 设计，即使这些日志最终没有输出，我们也要付出构造字段的额外开销。

因此，Go-Spring 的 `Trace` 和 `Debug` 使用函数**惰性**构造并返回 Field。

```go
log.Debug(ctx, TagBizOrder, func() []log.Field {
	return []log.Field{
		log.Any("snapshot", buildOrderSnapshot(order)),
		log.Msg("订单调试信息"),
	}
})
```

在上面的代码示例中，Go-Spring 首先检查对应 Logger 是否启用了 `DEBUG`。只有级别判断通过以后，字段函数才会执行。这样，`buildOrderSnapshot` 就不会在日志关闭时产生开销。

## 结构化日志

结构化日志的价值不仅在于输出 JSON，更在于让业务字段能够被日志平台索引、过滤和聚合。下一篇，咱们沿着日志处理链路，看看 Tag 和 Logger 如何对日志进行路由、过滤和调度。
