# Go-Spring 实战第 22 课：结构化日志字段：让日志可检索、可聚合、可分析

上一篇我们先搭起了 Go-Spring 日志系统的骨架——标签负责语义路由，Logger 负责调度，Appender、Layout 和 Encoder 负责输出。现在我们回到业务代码真正会写出来的部分，也就是字段。

如果日志只是一段字符串，排查时只能靠全文检索；一旦要统计、聚合、按字段过滤，纯文本就很吃力。所以这一篇先把字段模型讲清楚，后面才不容易把日志又写回字符串。

结构化日志把日志内容拆成带类型的字段，让日志平台可以直接索引、聚合和过滤。字段一旦稳定下来以后，日志就不只是“给人看的文本”，也是系统观测的一部分。反过来说，如果关键业务信息只留在字符串里，后面再想聚合就会很被动。

Go-Spring 的 Field 系统参考了 zerolog、zap 等库，用的是强类型字段，尽量避免运行时反射。

## 基础字段要把类型带清楚

基础字段的写法很直接，即每个字段函数同时声明字段名和值类型，最后用 `Msg` 放一句人类可读摘要。

```go
log.Info(ctx, tag,
	log.Int("user_id", userID),
	log.String("ip", ip),
	log.Bool("success", success),
	log.Msg("用户登录完成"),
)
```

常见基础类型如下。

```go
log.Bool("success", true)
log.Int("user_id", 10001)
log.Int("duration_us", duration.Microseconds())
log.Uint("bytes_transferred", uint64(1024*1024))
log.Float("amount", 99.99)
log.String("order_no", "ORD202401010001")
```

字段函数直接携带类型信息，编码时不需要重新推断。这也是它比 `map[string]any` 更适合高频日志路径的原因，即类型信息在写日志时就已经确定了。

## nil 和空值需要稳定表达

指针字段会自动处理 nil。

```go
var enabled *bool
log.BoolPtr("enabled", enabled)

var userID *int64
log.IntPtr("user_id", userID)

var remark *string
log.StringPtr("remark", remark)

log.Nil("deleted_at")
```

如果指针为 nil，字段输出为 `null`。这样比省略字段更适合表达“字段存在但值为空”的语义，因为下游系统能区分“没有这个字段”和“字段值为空”。

## msg 只保留给人读的摘要

`Msg` 和 `Msgf` 是特殊字段，key 都是 `msg`。

```go
log.Msg("订单创建成功")
log.Msgf("处理了 %d 条记录，成功 %d，失败 %d", total, success, failed)
```

`msg` 适合保存人类可读摘要。结构化信息仍然拆成独立字段，例如订单号、用户 ID、状态码、耗时等。

可以把 `msg` 当成一句话摘要，可检索的业务信息则放到独立字段里。这样后续检索和聚合才能继续走结构化路径。

## 数组和对象尽量保留原始结构

当一个字段天然是列表时，直接使用数组字段函数，日志平台会保留数组结构。

```go
log.Ints("item_ids", []int{1, 2, 3})
log.Strings("tags", []string{"vip", "new_user"})
log.Bools("flags", []bool{true, false})
log.Floats("prices", []float64{9.99, 19.99})
```

当多个字段共同描述一个业务对象时，用 `Object` 把它们收进同一个结构下。

```go
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

嵌套对象适合表达一个字段下的结构化子对象，尤其是 JSON 输出。这样日志平台拿到的也是天然的对象结构。

## Map 展开留给动态字段集合

`FieldsFromMap` 会把 `map[string]any` 展开成多个字段。

```go
data := map[string]any{
	"order_id": "ORD001",
	"amount":   99.99,
	"user_id":  int64(10001),
	"success":  true,
}

log.Info(ctx, tag, log.FieldsFromMap(data))
```

这种方式适合已有动态字段集合的场景。如果字段本身是稳定的，直接使用强类型字段函数会更清楚。

## Any 只做兜底，不做默认选择

`Any` 会根据值自动选择编码方式。

```go
log.Any("order_id", "ORD001")
log.Any("amount", 99.99)
log.Any("user_id", int64(10001))
log.Any("tags", []string{"a", "b"})
```

`Any` 使用方便，但类型不如强类型字段明确，无法识别时可能回退到反射编码。所以高频路径更适合使用强类型字段。

## 字段稳定性比字段数量更重要

结构化日志字段越稳定、清晰、可检索，后续排查和聚合越省力。字段名保持稳定，多个信息拆成多个字段，本应结构化的业务字段也尽量不要塞进 `msg`。

字段表达清楚以后，日志事件还需要被调度到正确输出路径，这就进入了同步、异步、控制台、文件、滚动文件和自定义 Logger 的选择问题。
