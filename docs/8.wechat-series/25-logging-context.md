# Go-Spring 实战第 25 课 —— 日志上下文：trace_id 与请求信息自动进入日志

前面几篇已经把 Go-Spring 的日志事件从业务调用一路讲到了输出管线。现在我们补上一个线上排查时非常关键、但也很容易漏掉的细节，即上下文字段。

单条日志本身往往不够。一个请求会经过多个服务和组件，产生大量日志。如果每条日志都需要业务代码手动传 `trace_id`、`user_id`、`request_id`，不仅重复，还很容易漏。所以这类字段更适合从上下文统一提取。

Go-Spring 提供了全局上下文提取钩子，可以从 `context.Context` 中自动提取字段，并注入到每条日志事件中。这样，调用点继续只写业务字段，链路字段由日志系统统一补齐。

## 上下文里适合自动带出哪些字段

上下文字段应该是跨调用点稳定出现、并且对串联请求链路有价值的信息。常见字段如下。

| 字段 | 含义 |
|------|------|
| `trace_id` | 全局链路追踪 ID |
| `span_id` | 当前调用跨度 |
| `request_id` | HTTP 请求 ID |
| `user_id` | 当前用户 |
| `client_ip` | 客户端 IP |
| `tenant_id` | 租户标识 |

这些字段通常在请求入口层写入 context，Go-Spring 日志系统在输出时统一读取。这个分工很重要：请求入口负责建立上下文，日志系统负责提取上下文，业务代码不必在每个日志调用点重复拼字段。

## FieldsFromContext 输出结构化上下文字段

`FieldsFromContext` 返回结构化字段，是更常用的方式。它可以把 trace、用户、请求 ID 这类稳定字段接进结构化日志模型，并在提取时确定字段类型。

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
		fields = append(fields, log.Int("user_id", userID))
	}
	if requestID, ok := ctx.Value("request_id").(string); ok {
		fields = append(fields, log.String("request_id", requestID))
	}

	return fields
}
```

设置后，业务代码只需要正常传入 `ctx`。下面的日志调用只写业务字段，链路字段会由全局钩子补齐。

```go
log.Info(ctx, TagBizOrder,
	log.String("order_no", "ORD001"),
	log.Msg("订单创建成功"),
)
```

这段代码的语义是：`FieldsFromContext` 在每次日志输出时执行，返回的字段会和调用点传入的业务字段一起进入 Layout。这样一来，最终输出就会自动包含上下文字段。

## OpenTelemetry 提供 trace 和 span 信息

生产环境里，常见做法是从 OpenTelemetry Context 提取链路信息。Go-Spring 日志系统不需要自己发明 trace 协议，只要从已有上下文里读出标准链路 ID。

```go
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

		if span.SpanContext().IsSampled() {
			fields = append(fields, log.Bool("sampled", true))
		}
	}

	return fields
}
```

这段代码的语义是从当前 Span 中读取 `trace_id`、`span_id` 和采样状态，再把它们作为结构化字段输出。这样日志和分布式追踪就可以通过同一组 ID 串联。查一条请求链路时，不需要在日志和 trace 之间手工猜关联关系。

## StringFromContext 兼容旧的文本格式

`StringFromContext` 提取一个格式化字符串。它适合日志格式暂时不能结构化改造的历史系统，把上下文信息拼进文本前缀即可。

```go
type traceCtxType struct{}

log.StringFromContext = func(ctx context.Context) string {
	trace, _ := ctx.Value(traceCtxType{}).(string)
	return trace
}
```

它的语义是把返回字符串放进事件的上下文字符串位置。它更适合历史系统或文本格式兼容场景。不过，新代码通常优先使用 `FieldsFromContext`，因为结构化字段保留类型信息，也更适合索引和聚合。

## 上下文提取一定要保持轻量

上下文提取会在每一次日志输出时执行，所以这条路径越轻越好。复杂操作放进来以后，日志路径本身就可能变成性能负担。

为了让这条全局路径保持稳定，常见做法是把工作前移到请求入口。

- 在请求入口处一次性把需要的值写入 context。
- 提取时只做简单类型断言和读取。
- 高频字段放在前面。
- 避免创建复杂对象。

反过来说，下面这些操作不适合放进日志上下文钩子。

- 在钩子中做复杂计算。
- 在钩子中访问网络或磁盘。
- 在钩子中加锁。
- 使用反射遍历 context。

上下文提取是观测链路的基础设施，越稳定、越轻量，越适合放到全局路径。

## 日志上下文

上下文提取在每次日志输出时执行，它的价值是减少业务代码重复传字段，而不是把复杂计算塞进日志钩子。全局钩子越简单，对所有日志调用的影响就越可控。

Go-Spring 把日志上下文放在全局钩子里，是为了让 trace、request、user、tenant 这类跨调用点字段进入统一模型。请求入口负责写入 context，日志钩子负责轻量提取，业务调用点只保留当前事件自己的字段。这个边界越清楚，日志链路越稳定。
