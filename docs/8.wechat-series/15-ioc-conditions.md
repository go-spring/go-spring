# Go-Spring 实战第 15 课 —— Condition：根据配置和上下文激活 Bean

在自动装配出现之前，装配逻辑大多直接写在应用代码里。也就是说，一个 Bean 要不要注册，通常在项目初始化代码里就已经写死了。这样做很直观，但代价也明显：同一套 Bean 很难在不同项目之间复用。

在自动装配出现之后，我们把体系化的 Bean 注册逻辑提取到了 Starter 里，复用性确实更强了。但问题也随之出现：应用引入 Starter 后，并不一定需要里面的每个 Bean；有些 Bean 可能缺少必要配置；还有些场景下，用户希望用自己注册的 Bean 替换 Starter 提供的默认实现。

这时候就需要条件来参与决策：在容器解析阶段，根据配置、已有 Bean 或其他上下文，决定某个 Bean 是否启用。

## Condition

下面是一个简单的条件注册：

```go
func init() {
	gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
}
```

在上面的代码中，我们注册了 `NewMyService`，同时指定它只有在 `my.condition` 存在时才启用。

具体来说，Go-Spring 会在容器解析阶段，根据 `my.condition` 是否存在，判断 `NewMyService` 是否应该参与本次装配。如果 `my.condition` 存在，`NewMyService` 会被创建、注入和初始化；如果不存在，`NewMyService` 就会被移出本次装配流程。

条件的本质其实就是一次 if 判断，只不过这个判断发生在 Bean 定义层面。把判断条件写在注册语句里，而不是塞进构造函数或者业务代码里，对代码的侵入性就会小很多。

从实现上看，`Condition` 的核心就是一个匹配函数：

```go
type Condition interface {
	Matches(ctx ConditionContext) (bool, error)
}
```

每个具体的条件都实现 `Condition` 接口，并通过 `ConditionContext` 读取配置和查找 Bean。

## 基于配置的条件

配置通常是最直接的开关，所以最常见的条件，就是围绕配置项来判断：它是否存在、值是否等于某个值，或者是否满足某个表达式。对应到 Go-Spring 里，主要就是使用 `OnProperty`。

`OnProperty` 既可以判断配置项是否存在，也可以判断值是否等于某个值，还可以通过表达式完成更复杂的逻辑判断。

```go
gs.OnProperty("redis.enabled")
gs.OnProperty("redis.enabled").HavingValue("true")
gs.OnProperty("redis.enabled").HavingValue("true").MatchIfMissing()
```

这三个写法的含义稍有不同。只写 `OnProperty("redis.enabled")` 时，表示只要配置项存在，条件就成立；加上 `HavingValue("true")` 后，表示配置项存在并且最终值等于 `true` 时，条件才成立；再加上 `MatchIfMissing()`，则表示配置不存在也视为条件成立。

这里的配置项不限定某个来源，可以来自配置文件、环境变量、Profile 配置、基础配置和代码默认值等。因此，`OnProperty` 看的是合并后的配置体系，而不是某一个单独来源。

不过，`HavingValue` 有一个边界：它只能判断叶子值，不能直接判断结构值，否则会返回错误并中断启动。如果传入的是普通字面量，比如 `true`、`123` 等，表示等值判断。如果判断规则已经不是简单等值，比如要判断范围、前缀或包含关系，就可以改用 `expr:` 前缀表达式。

表达式里用 `$` 表示当前配置值。Go-Spring 基于 expr 库计算表达式，所以可以写出更丰富的判断规则，比如：

```go
// 端口必须大于 8080
gs.OnProperty("server.port").HavingValue("expr:int($) > 8080")
// 端口必须在 1024 到 65535 之间
gs.OnProperty("server.port").HavingValue("expr:int($) > 1024 && int($) < 65535")
// 环境必须不是生产环境
gs.OnProperty("app.env").HavingValue(`expr:$ != "prod"`)
```

`expr` 表达式必须返回布尔值。只要表达式解析失败、类型不匹配，或者返回值不是布尔值，Go-Spring 都会报错并终止启动。

如果判断逻辑继续变复杂，硬往 `expr` 里堆条件会影响可读性。这时可以把规则下沉到自定义校验函数里。

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

在上面的代码中，我们注册了一个 `isValidPort` 函数，用于判断端口是否在 1024 到 65535 之间。然后在注册 `NewServer` 时，使用 `expr:isValidPort($)` 表达式来调用它。这样一来，表达式里只保留一次函数调用，真正的判断逻辑放在 Go 函数里，读起来会更清楚。

## 基于 Bean 的条件

除了配置，容器里已经有哪些 Bean 也是很常见的判断依据。比如一个价格服务 Starter 可以提供 HTTP 默认客户端，但如果应用已经注册了自己的 `domain.PriceClient`，Starter 的默认实现就应该退出；再比如某个增强组件依赖基础客户端，基础客户端不存在时，增强组件也没有必要启用。

还是看一个例子：

```go
func init() {
	gs.Provide(NewHTTPPriceClient, gs.TagArg("${bookman.price}")).
		Export(gs.As[domain.PriceClient]())

	gs.Provide(NewPriceReporter).
		Condition(gs.OnBean[domain.PriceClient]())
}
```

在上面的代码中，`NewPriceReporter` 依赖 `domain.PriceClient`。只有 `domain.PriceClient` 存在时，`NewPriceReporter` 才会启用。换句话说，Reporter 不自己决定客户端从哪里来，它只关心容器里是否已经有可用的 `PriceClient`。

Go-Spring 提供了几种围绕 Bean 存在性的条件：

```go
gs.OnBean[*HttpServeMux]()
gs.OnMissingBean[UserService]()
gs.OnSingleBean[*DataSource]()
gs.OnBean[*DataSource]("master")
```

其中，`OnBean` 表示至少存在一个匹配 Bean。`OnMissingBean` 表示不存在匹配 Bean。`OnSingleBean` 表示恰好存在一个匹配 Bean。这几个条件既可以只按类型匹配，也可以传入名称，按类型和名称一起匹配。

还有一个细节需要注意：Go-Spring 在判断 Bean 是否存在时，会跳过已经因为条件不成立而退出装配的 Bean。换句话说，一个已经退出本次装配的 Bean，不会再影响后续条件判断。

## 自定义条件

Go-Spring 内置的条件已经覆盖了绝大多数场景。不过，如果规则已经超出了配置值和 Bean 存在性，比如需要读取多个上下文信息，或者要做更细的业务判断，就可以使用 `OnFunc`。

写法也很直接：

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

在上面的代码中，我们通过 `OnFunc` 创建了一个条件：只有 `audit.mode` 存在并且值不是 `off` 时，`NewAuditSink` 才会启用。相比把逻辑塞到字符串表达式里，`OnFunc` 更适合承载这种稍微复杂一点的判断。

## 条件组合

有些场景下，单个条件不够，需要把多个条件合在一起判断。为了把组合关系说清楚，Go-Spring 提供了 `And`、`Or`、`Not` 和 `None` 四个组合条件。

例如，某个指标组件既要求配置开关打开，又要求 HTTP 入口已经存在，可以写成：

```go
func init() {
	gs.Provide(NewMetricsMiddleware).Condition(gs.And(
		gs.OnProperty("metrics.enabled").HavingValue("true"),
		gs.OnBean[*HttpServeMux](),
	))
}
```

如果两个配置开关任意一个打开就启用，可以使用 `Or`：

```go
func init() {
	gs.Provide(NewDebugExporter).Condition(gs.Or(
		gs.OnProperty("debug.enabled").HavingValue("true"),
		gs.OnProperty("trace.enabled").HavingValue("true"),
	))
}
```

如果只有真实实现不存在时才启用兜底实现，可以使用 `Not`：

```go
func init() {
	gs.Provide(NewFallbackSender).Condition(gs.Not(
		gs.OnBean[*RealSender](),
	))
}
```

如果一组互斥开关都没有打开时才启用默认能力，可以使用 `None`：

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

组合条件可以表达复杂关系，但条件越复杂，注册语句也越难读。因此，一方面可以将组合条件提取成变量，让主流程更清楚；另一方面也要克制使用条件组合。如果一段注册逻辑已经出现很复杂的条件组合，就要反过来想想，配置边界或者模块设计是不是可以再调整一下。

## 条件结果缓存

有时候，多个 Bean 的装配条件会共享同一段判断，而且这段判断写起来比较长，或者执行成本比较高。这时可以用 `OnOnce` 共享同一个条件实例。`OnOnce` 会缓存第一次匹配结果，后续再次判断同一个条件实例时，直接返回缓存结果。

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

在这个例子里，`NewMetricsExporter` 和 `NewMetricsMiddleware` 共享了 `metricsCondition` 条件。

不过要提醒的是，`OnOnce` 不应该变成默认选择。简单条件直接重复写通常更清楚；只有在它确实能减少重复、提升可读性，或者条件计算本身有明显成本时，才值得使用。
