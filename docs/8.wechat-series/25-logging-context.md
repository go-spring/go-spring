# Go-Spring 实战第 25 课：日志上下文提取：自动补齐 trace_id、request_id 和用户信息

前面几篇已经把 Go-Spring 的日志事件从业务调用一路讲到了输出管线。现在我们补上一个线上排查时非常关键、但也很容易漏掉的细节：上下文字段。

单条日志本身往往不够。一个请求会经过多个服务和组件，产生大量日志。如果每条日志都需要业务代码手动传 `trace_id`、`user_id`、`request_id`，不仅重复，还很容易漏。所以这类字段最好从上下文统一提取。

Go-Spring 提供了全局上下文提取钩子，可以从 `context.Context` 中自动提取字段，并注入到每条日志事件中。

## 哪些字段应该从上下文自动带出

| 字段 | 含义 |
|------|------|
| `trace_id` | 全局链路追踪 ID |
| `span_id` | 当前调用跨度 |
| `request_id` | HTTP 请求 ID |
| `user_id` | 当前用户 |
| `client_ip` | 客户端 IP |
| `tenant_id` | 租户标识 |

这些字段通常在请求入口层写入 context，Go-Spring 日志系统在输出时统一读取。这样把写入和提取分开，业务代码就不必在每个日志调用点重复拼字段。

## FieldsFromContext 输出结构化字段

`FieldsFromContext` 返回结构化字段，是推荐方式：

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

设置后，业务代码只需要正常传入 `ctx`：

```go
log.Info(ctx, TagBizOrder,
	log.String("order_no", "ORD001"),
	log.Msg("订单创建成功"),
)
```

这样一来，最终输出就会自动包含上下文字段。

## OpenTelemetry 会提供 trace 和 span 信息

生产环境里，常见做法是从 OpenTelemetry Context 提取链路信息：

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

这样日志和分布式追踪就可以通过同一组 ID 串联。

## StringFromContext 兼容文本格式

`StringFromContext` 提取一个格式化字符串：

```go
type traceCtxType struct{}

log.StringFromContext = func(ctx context.Context) string {
	trace, _ := ctx.Value(traceCtxType{}).(string)
	return trace
}
```

它更适合历史系统或文本格式兼容场景。如果是新代码的话，优先使用 `FieldsFromContext`，因为结构化字段保留类型信息。

## 上下文提取必须轻量

上下文提取会在每一次日志输出时执行，所以一定要很轻。否则日志路径本身就会变成性能负担。

建议：

- 在请求入口处一次性把需要的值写入 context。
- 提取时只做简单类型断言和读取。
- 高频字段放在前面。
- 避免创建复杂对象。

禁止：

- 在钩子中做复杂计算。
- 在钩子中访问网络或磁盘。
- 在钩子中加锁。
- 使用反射遍历 context。

上下文提取是观测链路的基础设施，越稳定、越轻量，越适合放到全局路径。

## 全局上下文钩子不能承担复杂逻辑

上下文提取在每次日志输出时执行，它的价值是减少业务代码重复传字段，而不是把复杂计算塞进日志钩子。这个边界要守住，否则全局钩子会影响所有日志调用。

上下文字段补齐以后，日志系统还要回到工程治理：如何用配置驱动 Logger、Appender、Layout，如何接入标准库 `log` 和 Zap。
