# Go-Spring 实战第 23 课 —— 日志上下文：通过 Context 关联链路追踪

上一篇我们介绍了 Appender、Layout 和 Encoder。一条 Event 进入输出阶段以后，可以被编码成文本或 JSON，再写到控制台或者文件。

不过，格式正确并不意味着日志容易排查。一次 HTTP 请求可能经过多个中间件、业务方法和下游服务，每个环节都会产生自己的日志。只看其中一条，我们可能知道订单创建失败了，却不知道它来自哪个请求，也不知道应该到哪条 Trace 中继续定位。

要把这些分散的日志关联起来，每条日志都需要携带 `trace_id`、`span_id` 或 `request_id` 等链路信息。问题在于，这些信息属于整条请求链路，而不是某一个业务事件。如果每个调用点都手动传递，不仅代码重复，也很容易在某一层遗漏。

Go-Spring 的日志上下文提取，就是为了解决这个问题。

## Context 与链路追踪

在 Go 服务中，`context.Context` 会沿着一次请求的调用过程逐层传递。HTTP 中间件可以把请求标识写入 Context，链路追踪 SDK 也会把当前 Span 放入 Context；进入下一个进程时，Trace 信息通过 HTTP Header 或 RPC 元数据传播，再由对端恢复到新的 Context 中。

因此，Context 是请求级信息在应用内部传播的载体，而 Trace 则描述这次请求经过了哪些服务和调用阶段。日志本身只是一个个离散事件，只有从 Context 中取得相同的链路标识，才能与其他日志以及 Trace 关联起来。

常见的上下文信息包括：

| 字段 | 作用 |
|------|------|
| `trace_id` | 关联同一条分布式调用链 |
| `span_id` | 定位当前调用阶段 |
| `request_id` | 关联同一次请求 |
| `user_id` | 关联当前用户 |
| `tenant_id` | 区分当前租户 |

这些值通常在请求入口、RPC 拦截器或者 Trace SDK 中产生。日志系统不负责重新计算它们，只需要在记录日志时从 Context 中读取，并自动补充到 Event。

这样一来，业务调用点只描述当前发生的事件，链路公共字段则随着 Context 统一传播。

## 上下文提取 API

Go-Spring 提供了两个全局扩展 API：`FieldsFromContext` 和 `StringFromContext`。它们都接收 `context.Context`，但返回的数据形态不同。

`FieldsFromContext` 返回一组结构化 Field，适合 JSON 日志、字段检索和聚合；`StringFromContext` 返回一段已经格式化好的字符串，适合文本日志或者兼容已有的上下文格式。两个扩展点可以单独使用，也可以同时配置。

### FieldsFromContext

`FieldsFromContext` 用于提取多个结构化字段。

```go
type (
	requestIDKey struct{}
	userIDKey    struct{}
)

func init() {
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		var fields []log.Field

		if requestID, ok := ctx.Value(requestIDKey{}).(string); ok {
			fields = append(fields, log.String("request_id", requestID))
		}
		if userID, ok := ctx.Value(userIDKey{}).(int64); ok {
			fields = append(fields, log.Int("user_id", userID))
		}
		return fields
	}
}
```

配置完成以后，业务代码仍然按照普通方式记录日志，只传入当前事件自己的字段。

```go
log.Info(ctx, TagBizOrder,
	log.String("order_no", orderNo),
	log.Msg("订单创建完成"),
)
```

日志通过级别判断以后，Go-Spring 会调用 `FieldsFromContext`。返回值会进入 Event 的 `CtxFields`，再由 Layout 和调用点传入的业务 Field 一起编码。

最终的 JSON 日志可能如下：

```json
{"level":"info","time":"2026-06-05T10:30:00.000","fileLine":"order.go:42","tag":"_biz_order_create","request_id":"req-1001","user_id":10001,"order_no":"ORD001","msg":"订单创建完成"}
```

`request_id` 和 `user_id` 来自 Context，`order_no` 和 `msg` 来自当前调用点。两类字段最终出现在同一条日志中，但各自的来源和职责保持清楚。

### StringFromContext

`StringFromContext` 用于提取一段已经格式化好的上下文字符串。

```go
type traceKey struct{}

func init() {
	log.StringFromContext = func(ctx context.Context) string {
		traceID, _ := ctx.Value(traceKey{}).(string)
		if traceID == "" {
			return ""
		}
		return "trace_id=" + traceID
	}
}
```

`TextLayout` 会把这段字符串放在 Tag 和结构化 Field 之间：

```text
[INFO][...][order.go:42] _biz_order_create||trace_id=abc123||order_no=ORD001||msg=订单创建完成
```

`JSONLayout` 不会解析这段字符串，而是把它保存在 `ctxString` 字段中：

```json
{"level":"info","time":"2026-06-05T10:30:00.000","fileLine":"order.go:42","tag":"_biz_order_create","ctxString":"trace_id=abc123","order_no":"ORD001","msg":"订单创建完成"}
```

因此，新代码通常更适合使用 `FieldsFromContext`，让链路信息保留为独立的结构化字段。已经存在固定文本格式，或者需要整体传递一段上下文内容时，可以使用 `StringFromContext`。

如果同时配置两个扩展点，`StringFromContext` 的结果进入 Event 的 `CtxString`，`FieldsFromContext` 的结果进入 `CtxFields`，Layout 会同时处理它们。

## OpenTelemetry 适配

如果项目已经接入 OpenTelemetry，就不需要再维护一套独立的 Trace 标识。日志系统可以直接从当前 Context 中取得 `SpanContext`，复用 OpenTelemetry 的 `trace_id` 和 `span_id`。

```go
import "go.opentelemetry.io/otel/trace"

func init() {
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		spanContext := trace.SpanContextFromContext(ctx)
		if !spanContext.IsValid() {
			return nil
		}

		return []log.Field{
			log.String("trace_id", spanContext.TraceID().String()),
			log.String("span_id", spanContext.SpanID().String()),
			log.Bool("sampled", spanContext.IsSampled()),
		}
	}
}
```

当应用记录日志时，Go-Spring 会自动把当前 Span 的标识补充到 Event：

```go
log.Error(ctx, TagBizOrder,
	log.String("order_no", orderNo),
	log.String("error", err.Error()),
	log.Msg("订单创建失败"),
)
```

最终输出可能如下：

```json
{"level":"error","time":"2026-06-05T10:30:00.000","fileLine":"order.go:56","tag":"_biz_order_create","trace_id":"0af7651916cd43dd8448eb211c80319c","span_id":"b7ad6b7169203331","sampled":true,"order_no":"ORD001","error":"库存服务超时","msg":"订单创建失败"}
```

此时，日志和 Trace 使用同一个 `trace_id`。我们可以从错误日志跳转到完整调用链，也可以根据 Trace 中的 `span_id` 定位产生日志的具体调用阶段。上下文提取没有创建新的追踪体系，只是把日志接入已有的 OpenTelemetry 链路。
