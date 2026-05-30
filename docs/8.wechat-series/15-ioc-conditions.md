# Go-Spring 实战第 15 课 —— Condition：根据配置和上下文激活 Bean

在自动装配出现之前，装配逻辑大多直接写在应用代码里。也就是说，一个 Bean 要不要注册，通常在项目初始化阶段就已经被写死了。这样做很直观，但代价也很明显：同一套 Bean 很难在不同的项目之间复用。

有了自动装配之后，我们把体系化的 Bean 注册逻辑可以提取到 Starter 里，复用性确实更强。但新问题也随之出现：应用引入 Starter 后，并不一定需要里面的每一个 Bean；有些场景下，用户还希望用自己注册的 Bean 替换掉 Starter 提供的默认实现。

Go-Spring IoC 容器在 Bean 解析阶段，会根据配置、已有 Bean 或其他上下文，决定某个 Bean 是否启用。这种机制就是条件（Condition）。它可以很好地解决上面这些问题。

## Condition

先看一个简单的条件注册示例：

```go
func init() {
	gs.Provide(NewMyService).
		Condition(gs.OnProperty("my.condition"))
}
```

在上面的代码中，我们使用 `gs.Provide()` 注册 `NewMyService`，然后通过 `Condition()` 和 `gs.OnProperty()` 指定它只有在 `my.condition` 存在时才会启用。如果 `my.condition` 存在，`NewMyService` 会被创建、注入和初始化；如果不存在，`NewMyService` 会从本次装配流程中移除。

条件本质上就是一次 if 判断，只是这个判断发生在 Bean 定义层面。我们把判断条件写在注册语句里，而不是放进构造函数或业务代码里，这样可以明显降低装配逻辑对业务代码的侵入，应该说是“零侵入”。

从实现上看，`Condition` 的核心就是一个匹配函数：

```go
type Condition interface {
	Matches(ctx ConditionContext) (bool, error)
}
```

每个具体的条件都要实现 `Condition` 接口，并通过 `ConditionContext` 读取配置和查找 Bean。

## 基于配置的条件

配置通常是最直接的开关，所以最常见的条件就是围绕配置项实现的。配置项是否存在、配置项的值是否等于某个值、是否满足某个表达式，都可以作为激活 Bean 的条件。

在 Go-Spring 里，配置条件主要使用 `OnProperty`。它既可以判断配置项是否存在，也可以判断配置项的值是否等于某个值，还可以通过表达式完成更复杂的逻辑判断。

常见写法如下：

```go
gs.OnProperty("redis.enabled")
gs.OnProperty("redis.enabled").HavingValue("true")
gs.OnProperty("redis.enabled").HavingValue("true").MatchIfMissing()
```

上面三种写法的含义略有不同。只写 `OnProperty("redis.enabled")` 时，表示只要配置项存在，条件就能成立；加上 `HavingValue("true")` 后，表示配置项存在并且最终值等于 `true` 时，条件才能成立；再加上 `MatchIfMissing()` 后，则表示即使配置不存在，也可以视为条件成立。

实际使用时，我们可以根据场景选择其中一种，或者组合使用。

> 需要说明的是，`OnProperty` 使用合并之后的配置体系，而不是某一个单独的配置来源。读者如果对 Go-Spring 的配置体系还不熟悉，可以查看本系列开头的几篇文章。

不过，`HavingValue` 有一个细节需要注意，即它只能判断叶子值，不能直接判断结构值，否则会返回错误并中断启动。如果我们传入的是普通字面量，比如 `true`、`123` 等，那么表示等值判断。如果判断规则不只是简单的等值判断，比如需要判断范围、前缀或者包含关系，我们就可以使用 `expr:` 表达式。

`expr:` 表达式基于 expr-lang 库实现，支持各种复杂的逻辑判断。我们可以在表达式里使用 `$` 表示当前配置值，从而写出更丰富的判断规则。

例如：

```go
// 端口必须大于 8080
gs.OnProperty("server.port").
	HavingValue("expr:int($) > 8080")

// 端口必须在 1024 到 65535 之间
gs.OnProperty("server.port").
	HavingValue("expr:int($) > 1024 && int($) < 65535")

// 环境必须不是生产环境
gs.OnProperty("app.env").
	HavingValue(`expr:$ != "prod"`)
```

`expr:` 表达式要求结果必须是布尔值。如果表达式解析失败、类型不匹配，或者返回值不是布尔值，Go-Spring 会报错并终止启动。

如果判断逻辑进一步变得复杂，那么把规则写在 `expr:` 里可能会影响可读性。这时我们可以把规则提取到自定义校验函数里。

举个例子：

```go
func init() {
	gs.RegisterExpressFunc("isValidPort", func(s string) bool {
		port, err := strconv.Atoi(s)
		return err == nil && port > 1024 && port < 65535
	})

	gs.Provide(NewServer).Condition(
		gs.OnProperty("server.port").
			HavingValue("expr:isValidPort($)"),
	)
}
```

在上面的代码中，我们注册了一个 `isValidPort` 函数，它可以判断端口是否在 1024 到 65535 之间。然后我们在注册 `NewServer` 时，使用 `expr:isValidPort($)` 表达式来调用它。这样，表达式里只会保留一次函数调用，真正的判断逻辑在 Go 函数里，读起来就会更加清楚。

## 基于 Bean 的条件

除了配置，容器里面某些 Bean 是否存在，也是很常见的判断依据。比如 Starter 里的某个增强组件依赖用户提供的基础客户端 Bean，如果这个客户端 Bean 不存在，那么增强组件也就没有必要启用。

看个例子：

```go
func init() {
	gs.Provide(NewHTTPPriceClient, gs.TagArg("${bookman.price}")).
		Export(gs.As[domain.PriceClient]())

	gs.Provide(NewPriceReporter).
		Condition(gs.OnBean[domain.PriceClient]())
}
```

在上面的代码中，`NewPriceReporter` 依赖于 `domain.PriceClient`。只有 `domain.PriceClient` 存在时，`NewPriceReporter` 才会启用。换句话说，Reporter 自己不决定客户端从哪里来，而只关心容器里是否已经有可用的 `PriceClient`。这种场景在 Starter 里非常常见。

Go-Spring 提供了三种围绕 Bean 存在性的条件：

```go
gs.OnBean[*HttpServeMux]()
gs.OnMissingBean[UserService]()
gs.OnSingleBean[*DataSource]()
gs.OnBean[*DataSource]("master")
```

其中，`OnBean` 表示至少需要存在一个匹配 Bean，否则条件就不成立。`OnMissingBean` 表示必须不存在匹配 Bean，否则条件就不成立。`OnSingleBean` 表示恰好只存在一个匹配 Bean，否则条件就不成立。这几个条件都是既可以只按类型匹配，也可以传入名称，按类型和名称一起匹配。

> 有个细节可能需要说明：Go-Spring 在判断 Bean 是否存在时，会跳过已经因为条件不成立而退出本次装配的 Bean。

## 自定义条件

Go-Spring 内置的条件已经覆盖了绝大多数场景。不过，如果判断规则超出了配置值和 Bean 存在性，比如需要读取多个上下文信息，或者需要更细的业务判断，那么可以使用 `OnFunc`。

示例如下：

```go
func init() {
	gs.Provide(NewAuditSink).Condition(gs.OnFunc(
		func(ctx gs.ConditionContext) (bool, error) {
			mode, ok := ctx.Prop("audit.mode")
			return ok && mode != "off", nil
		},
	))
}
```

在上面的代码中，我们通过 `OnFunc` 创建了一个条件：只有 `audit.mode` 存在并且值不是 `off` 时，`NewAuditSink` 才会启用。相比于把复杂的判断逻辑塞进字符串表达式里，`OnFunc` 显然更适合承载这类稍微复杂的判断。

## 条件组合

有些场景下，单个条件可能不够，需要把多个条件合在一起组合判断。比如我们希望某个 Bean 只有在配置开关打开时才启用，同时还要求必须存在另一个 Bean。这种逻辑在 Starter 里很常见。

Go-Spring 提供了 `And`、`Or`、`Not` 和 `None` 四种组合条件，并且支持嵌套组合，可以满足各种复杂的判断逻辑。

如果某个中间件既要求配置开关打开，又要求 `*HttpServeMux` 同时存在，我们可以使用 `And`，代码如下：

```go
func init() {
	gs.Provide(NewMetricsMiddleware).Condition(gs.And(
		gs.OnProperty("metrics.enabled").HavingValue("true"),
		gs.OnBean[*HttpServeMux](),
	))
}
```

> 对于上面的代码，我们可以去掉外层的 `And`，因为 `Condition()` 默认就是 `And` 语义。

如果两个配置开关中任意一个打开就能启用，我们可以使用 `Or`，代码如下：

```go
func init() {
	gs.Provide(NewDebugExporter).Condition(gs.Or(
		gs.OnProperty("debug.enabled").HavingValue("true"),
		gs.OnProperty("trace.enabled").HavingValue("true"),
	))
}
```

如果只有 `*RealSender` 不存在时才能启用兜底实现，我们可以使用 `Not`，代码如下：

```go
func init() {
	gs.Provide(NewFallbackSender).Condition(gs.Not(
		gs.OnBean[*RealSender](),
	))
}
```

> 对于上面的代码，我们当然也可以使用 `OnMissingBean` 实现。这里仅用于展示 `Not` 的写法。

如果需要一组互斥开关都没有打开时才能启用默认能力，我们可以使用 `None`，代码如下：

```go
func init() {
	gs.Provide(NewDefaultReporter).Condition(gs.None(
		gs.OnProperty("reporter.file"),
		gs.OnProperty("reporter.remote"),
	))
}
```

我们还可以对组合条件进行嵌套，代码如下：

```go
gs.And(
	gs.OnProperty("service.enabled").HavingValue("true"),
	gs.Or(
		gs.OnBean[*PrimaryClient](),
		gs.OnBean[*BackupClient](),
	),
)
```

> 上面的代码仅用于展示组合条件的嵌套用法。实际场景中，仍然应该根据具体需求设计条件组合。

## 条件注册

条件很强大，但条件越复杂，注册语句就越难读，对 Bean 启用规则的理解也越困难。因此，我们在遇到复杂的组合条件时，需要特别注意以下两点：

- 首先，思考如此复杂的条件是否真的有必要，是否是由于配置边界或者模块设计的不够清晰，才导致条件组合变得复杂。
- 其次，可以将组合条件提取成变量，然后在多个 Bean 注册语句中复用。这样既能减少重复，也能让注册逻辑更清楚。

Condition 的价值不在于写出复杂的判断，而在于把 Bean 是否启用在装配层面进行表达。只有条件清晰、边界清楚，Starter 才更容易被复用，也更容易被应用按需覆盖。
