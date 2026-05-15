# Go-Spring 实战第 22 课：结构化日志字段：如何把业务事件写成可检索字段

上一篇先搭起了 Go-Spring 日志系统的骨架。标签负责语义路由，Logger 负责调度，Appender、Layout 和 Encoder 负责输出。回到业务代码，最容易被写坏的其实是字段。

线上排查时，纯文本日志只能靠全文检索。一旦要按订单号过滤、按状态码聚合、按耗时分桶，字符串就会变成负担。业务信息如果一开始只塞进 `msg`，后面再想补结构化索引，通常要改大量调用点。

Go-Spring 的 Field 系统参考了 zerolog、zap 等库，用强类型字段表达日志内容。字段名、字段类型和字段值在写日志时就确定下来，后续 Layout 和 Encoder 才能稳定输出给日志平台。

## 基础字段要在调用点声明类型

稳定字段应该优先使用强类型字段函数。这样字段名和值类型在调用点就已经确定，编码时不需要再从 `any` 里推断。

下面这条登录日志把可检索信息拆成独立字段，最后用 `Msg` 留一句人类可读摘要。

```go
log.Info(ctx, tag,
	log.Int("user_id", userID),
	log.String("ip", ip),
	log.Bool("success", success),
	log.Msg("用户登录完成"),
)
```

常见基础类型可以直接选择对应字段函数。

```go
log.Bool("success", true)
log.Int("user_id", 10001)
log.Int("duration_us", duration.Microseconds())
log.Uint("bytes_transferred", uint64(1024*1024))
log.Float("amount", 99.99)
log.String("order_no", "ORD202401010001")
```

强类型字段比 `map[string]any` 更适合高频日志路径。原因是类型信息已经在写日志时明确下来，Encoder 不需要为每条日志重新做运行时判断。

## nil 和空值需要稳定表达

有些字段不是没有，而是当前值为空。指针字段和 `Nil` 字段用于表达这类语义。

```go
var enabled *bool
log.BoolPtr("enabled", enabled)

var userID *int64
log.IntPtr("user_id", userID)

var remark *string
log.StringPtr("remark", remark)

log.Nil("deleted_at")
```

如果指针为 nil，字段输出为 `null`。这比直接省略字段更适合表达“字段存在但值为空”，因为下游系统可以区分“没有这个字段”和“字段值为空”。

## msg 只保留给人读的摘要

`Msg` 和 `Msgf` 是特殊字段，key 都是 `msg`。它们适合保存一句人类可读摘要，而不是承载所有业务信息。

```go
log.Msg("订单创建成功")
log.Msgf("处理了 %d 条记录，成功 %d，失败 %d", total, success, failed)
```

订单号、用户 ID、状态码、耗时这类可检索信息，仍然应该拆成独立字段。`msg` 的定位是帮助人快速扫读，结构化字段的定位是帮助机器检索、聚合和分析。

## 数组和对象要保留原始结构

当一个字段天然是列表时，直接使用数组字段函数。这样日志平台拿到的是数组，而不是被拼接后的字符串。

```go
log.Ints("item_ids", []int{1, 2, 3})
log.Strings("tags", []string{"vip", "new_user"})
log.Bools("flags", []bool{true, false})
log.Floats("prices", []float64{9.99, 19.99})
```

当多个字段共同描述一个业务对象时，用 `Object` 把它们收进同一个结构下。下面的订单对象还包含一个嵌套的商品对象。

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

嵌套对象尤其适合 JSON 输出。它保留业务对象的层次关系，避免把一组相关字段摊平成难以维护的命名约定。

## Map 展开只留给动态字段集合

有些日志字段来自外部系统或动态属性集合，调用点并不知道完整字段列表。这时可以使用 `FieldsFromMap` 把 `map[string]any` 展开成多个字段。

```go
data := map[string]any{
	"order_id": "ORD001",
	"amount":   99.99,
	"user_id":  int64(10001),
	"success":  true,
}

log.Info(ctx, tag, log.FieldsFromMap(data))
```

如果字段本身是稳定的，直接使用强类型字段函数会更清楚。`FieldsFromMap` 的价值在于承接动态字段集合，而不是替代所有结构化字段写法。

## Any 只做兜底，不做默认选择

`Any` 会根据值自动选择编码方式。它适合字段类型确实无法提前确定，或者临时接入已有对象的场景。

```go
log.Any("order_id", "ORD001")
log.Any("amount", 99.99)
log.Any("user_id", int64(10001))
log.Any("tags", []string{"a", "b"})
```

`Any` 使用方便，但类型不如强类型字段明确，无法识别时可能回退到反射编码。因此高频路径更适合使用强类型字段，`Any` 更适合作为兜底入口。

## 字段稳定性比字段数量更影响检索质量

结构化日志不是字段越多越好，而是字段越稳定、越清晰、越可检索，后续排查和聚合越省力。字段名保持稳定，多个信息拆成多个字段，本应结构化的业务信息不要塞进 `msg`。

字段表达清楚以后，日志事件还需要被调度到正确输出路径。接下来要看的就是 Logger 体系，也就是同步、异步、控制台、文件、滚动文件和自定义 Logger 分别解决什么问题。
