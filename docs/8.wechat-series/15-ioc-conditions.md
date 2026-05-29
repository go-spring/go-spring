# Go-Spring 实战第 15 课 —— Condition：根据配置和上下文激活 Bean

在自动装配出现之前，装配逻辑大多都是直接写在应用代码里的。每个 Bean 是否注册，在本项目的初始化阶段实际上就已经确定。但是也导致 bean 的复用性不高。

在自动装配出现之后，我们把体系化的 bean 注册逻辑提取到了 Starter 里，可复用性更强。但是也面临了新的问题。应用引入的 bean 可能是不需要，甚至可能缺乏配置的。而且用户可能希望使用自己的 bean，替换 starter 里面的默认 bean。

这时候我们就需要条件来决策 bean 是否启用。它需要在容器解析阶段，可以根据配置、已有 Bean 或其他上下文来决定哪个 bean 是启用的。

## Condition

下面是一个简单的条件注册：

```go
func init() {
	gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
}
```

在上面的代码中，我们注册了 `NewMyService`，同时指定它只能在 `my.condition` 存在时启用。

具体来说，Go-Spring 会在容器解析阶段，根据 `my.condition` 是否存在，判断 `NewMyService` 是否应该参与本次装配。如果 `my.condition` 存在，`NewMyService` 会被创建、注入和初始化；如果不存在，`NewMyService` 就会被删除。

条件的本质实际上就是 if 判断，但是它作用在 Bean 定义层面。条件注册把判断条件写在注册语句里，而不是塞进构造函数或者业务代码里，这样对代码的侵入性会更小。

从实现上看，`Condition` 的核心就是一个匹配函数：

```go
type Condition interface {
	Matches(ctx ConditionContext) (bool, error)
}
```

每个具体的条件都实现 `Condition` 接口，并通过 `ConditionContext` 读取配置和查找 Bean。

## 基于配置的条件

最常见的条件是根据配置项是否存在、值是否等于某个值，或者是否满足某个表达式来进行判断。（todo 这里缺少一个更好的过渡）

`OnProperty` 可以判断配置项是否存在、值是否等于某个值，也可以通过表达式完成更复杂的逻辑判断。

```go
gs.OnProperty("redis.enabled")
gs.OnProperty("redis.enabled").HavingValue("true")
gs.OnProperty("redis.enabled").HavingValue("true").MatchIfMissing()
```

在上面的代码中，只使用 `OnProperty("redis.enabled")` 时，表示配置项存在时条件才成立。使用 `HavingValue("true")` 时，表示配置项存在并且最终值等于 `true` 时条件成立。使用 `MatchIfMissing()` 时，表示配置不存在也视为条件成立。

配置项可以来自配置文件、环境变量、Profile 配置、基础配置和代码默认值等。`OnProperty` 面向的是合并后的配置体系，而不是某一个单独来源。

需要注意的是，`HavingValue` 只针对叶子值，不能直接判断结构值，否则会返回错误并中断启动。如果我们给 `HavingValue` 传入的是普通字面量，比如 `true`、`123` 等，表示等值判断。如果判断规则不是简单等值，比如范围、前缀或包含关系，可以使用 `expr:` 前缀表达式。

表达式里用 `$` 表示当前配置值。Go-Spring 基于 expr 库计算表达式，因此可以写出更丰富的判断规则。示例如下：

```go
// 端口必须大于 8080
gs.OnProperty("server.port").HavingValue("expr:int($) > 8080")
// 端口必须在 1024 到 65535 之间
gs.OnProperty("server.port").HavingValue("expr:int($) > 1024 && int($) < 65535")
// 环境必须不是生产环境
gs.OnProperty("app.env").HavingValue(`expr:$ != "prod"`)
```

expr 表达式必须返回布尔值。如果表达式解析失败，或者类型不匹配，或者返回值不是布尔值，会报错并终止启动。

如果判断逻辑比较复杂，继续堆 expr 表达式会影响可读性，可以把规则沉到自定义校验函数里。

举个例子：

```go
func init() {
	gs.RegisterExpressFunc("isValidPort", func(s string) bool {
		port, err := strconv.Atoi(s)
		return err == nil && port > 1024 && port < 65535
	})

	gs.Provide(NewServer).Condition(
		gs.OnProperty("server.port").HavingValue("expr:isValidPort($)"),
	)
}
```

在上面的代码中，我们注册了一个 `isValidPort` 函数，用于判断端口是否在 1024 到 65535 之间。然后，我们在注册 `NewServer` 时，使用 `expr:isValidPort($)` 表达式，判断端口是否在有效范围内。

## 基于 Bean 的条件

除了根据配置项决定 Bean 是否创建，我们还可能根据 Bean 是否存在进行判断。比如一个价格服务 Starter 可以提供 HTTP 默认客户端，但如果应用已经注册了自己的 `domain.PriceClient`，Starter 的默认实现就应该退出；再比如某个增强组件依赖基础客户端，基础客户端不存在时，增强组件也没有必要启用。

看个例子。

```go
func init() {
	gs.Provide(NewHTTPPriceClient, gs.TagArg("${bookman.price}")).
		Export(gs.As[domain.PriceClient]())

	gs.Provide(NewPriceReporter).
		Condition(gs.OnBean[domain.PriceClient]())
}
```

在上面的代码中，`NewPriceReporter` 依赖 `domain.PriceClient`。只有 `domain.PriceClient` 存在时，`NewPriceReporter` 才会启用。

Go-Spring 提供了几种围绕 Bean 存在性的条件：

```go
gs.OnBean[*HttpServeMux]()
gs.OnMissingBean[UserService]()
gs.OnSingleBean[*DataSource]()
gs.OnBean[*DataSource]("master")
```

其中，`OnBean` 表示至少存在一个匹配 Bean。`OnMissingBean` 表示不存在匹配 Bean。`OnSingleBean` 表示恰好存在一个匹配 Bean。这几个条件都可以只按类型匹配，也可以传入名称，按类型和名称共同匹配。

另外，Go-Spring 在判断 Bean 是否存在时，会跳过已经条件判断失败被删除的 bean。也就是说，一个条件不满足的 bean 不会继续影响后续的 `OnBean`、`OnMissingBean` 和 `OnSingleBean` 判断。

## 自定义条件

Go-Spring 内置的条件已经覆盖了绝大多数场景。如果规则已经超出配置值和 Bean 存在性，可以使用 `OnFunc`。

代码如下：

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

在上面的代码中，我们通过 `OnFunc` 创建了一个条件：只有 `audit.mode` 存在并且值不是 `off` 时，`NewAuditSink` 才会启用。

## 条件组合

有些场景下，我们需要通过多种条件来进行判断。对于需要明确表达组合关系的场景，Go-Spring 提供了 `And`、`Or`、`Not` 和 `None` 四个组合条件。

比如某个指标组件既要求配置开关打开，又要求 HTTP 入口已经存在，我们可以写成：

```go
func init() {
	gs.Provide(NewMetricsMiddleware).Condition(gs.And(
		gs.OnProperty("metrics.enabled").HavingValue("true"),
		gs.OnBean[*HttpServeMux](),
	))
}
```

比如两个配置开关任意一个打开就启用，可以使用 `Or`：

```go
func init() {
	gs.Provide(NewDebugExporter).Condition(gs.Or(
		gs.OnProperty("debug.enabled").HavingValue("true"),
		gs.OnProperty("trace.enabled").HavingValue("true"),
	))
}
```

比如只有真实实现不存在时才启用兜底实现，可以使用 `Not`：

```go
func init() {
	gs.Provide(NewFallbackSender).Condition(gs.Not(
		gs.OnBean[*RealSender](),
	))
}
```

比如一组互斥开关都没有打开时才启用默认能力，可以使用 `None`：

```go
func init() {
	gs.Provide(NewDefaultReporter).Condition(gs.None(
		gs.OnProperty("reporter.file"),
		gs.OnProperty("reporter.remote"),
	))
}
```

组合条件还可以继续嵌套：

```go
gs.And(
	gs.OnProperty("service.enabled").HavingValue("true"),
	gs.Or(
		gs.OnBean[*PrimaryClient](),
		gs.OnBean[*BackupClient](),
	),
)
```

组合条件能表达复杂关系，但条件越复杂，注册语句越难读。通常应该把可复用或业务含义明确的部分提取成变量；如果组合已经像业务流程，就要考虑是否应该调整配置边界或模块设计。

## 条件结果缓存

有时候，多个 Bean 的装配条件有相同部分，而且这部分写法较长或执行成本较高，可以使用 `OnOnce` 共享同一个条件实例。`OnOnce` 会缓存第一次匹配结果，后续再次判断同一个条件实例时，直接返回缓存结果。

代码如下：

```go
func init() {
	metricsCondition := gs.OnOnce(
		gs.OnProperty("metrics.enabled").HavingValue("true"),
		gs.OnBean[*HttpServeMux](),
	)

	gs.Provide(NewMetricsExporter).Condition(metricsCondition)
	gs.Provide(NewMetricsMiddleware).Condition(metricsCondition)
}
```

在上面的代码中，`NewMetricsExporter` 和 `NewMetricsMiddleware` 共享了 `metricsCondition` 条件。

不过，需要提醒的是，`OnOnce` 不是默认选择。简单条件直接重复写通常更清楚；只有它确实减少重复、提升可读性，或者条件计算本身有明显成本时，才值得使用。
