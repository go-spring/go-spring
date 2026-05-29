# Go-Spring 实战第 15 课 —— Condition：根据配置和上下文激活 Bean

条件注册用来把 Bean 是否启用的判断放到容器解析阶段。配置是否存在、开关是否打开、应用是否已经提供实现、基础组件是否可用，这些都会影响候选 Bean 是否应该参与本次启动；写成 `Condition` 以后，Go-Spring 就能在创建对象之前完成裁剪。

Redis 客户端通常只在配置了连接信息时注册，监控和调试组件通常由开关控制，Starter 会提供默认实现但又要允许应用覆盖，某些增强组件也只有在基础组件存在时才有意义。

如果这些判断散落在构造函数或者业务代码里，运行期就会出现一堆半启用组件：对象已经创建出来了，但内部还要反复判断配置、检查依赖、处理空值。条件注册把边界提前以后，容器完成解析时只会保留本次启动真正需要的 Bean。

## Condition

下面是一个简单的条件注册：

```go
func init() {
	gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
}
```

在上面的代码中，我们注册了 `NewMyService`，但是指定了它只能在 `my.condition` 存在时才启用。

具体来说，Go-Spring 会在容器解析阶段，根据 `my.condition` 是否存在，判断 `NewMyService` 是否应该参与本次装配。如果 `my.condition` 存在，`NewMyService` 会被创建、注入和初始化；如果不存在，`NewMyService` 会被裁剪掉，后续依赖查找和重复 Bean 检查都不会再把它算进去。

条件本质上就是 if 判断，但是它的写法更简洁。条件注册把判断条件写在注册语句里，而不是在构造函数里、业务代码里，对代码的侵入性更小。

从实现上看，`Condition` 的核心就是一个匹配函数：

```go
type Condition interface {
	Matches(ctx ConditionContext) (bool, error)
}
```

每个具体的条件都实现 `Condition` 接口，并通过 `ConditionContext` 读取配置和查找 bean。

## 基于配置的条件

最常见的条件是根据配置项是否存在、值是否等于某个值、是否等于某个范围等。像基础设施组件、可选插件、调试能力和灰度功能，通常都会有一个配置开关或者关键配置项。

`OnProperty` 可以判断配置项是否存在、值是否等于某个值、是否等于某个范围等。

```go
gs.OnProperty("redis.enabled")
gs.OnProperty("redis.enabled").HavingValue("true")
gs.OnProperty("redis.enabled").HavingValue("true").MatchIfMissing()
```

在上面的代码中，仅使用 `OnProperty("redis.enabled")` 时表示配置项存在时条件才成立。如果使用了 `HavingValue("true")` 则表示配置项存在，并且最终值等于 `true` 时条件成立。如果使用了 `MatchIfMissing()` 则表示配置不存在时也成立。

配置项可以来自于配置文件、环境变量、Profile 配置、基础配置和代码默认值等。它从视为整体的配置体系中取值。

需要注意的是，`HavingValue` 只针对叶子值，而不能判断结构值，否则会返回错误并中断启动。如果我们给 `HavingValue` 传入的是一个简单值，比如 `true`, `123` 等，表示等值判断。如果判断规则比较复杂，不是等值判断时，我们可以使用 `expr:` 前缀表达式。

我们在表达式里用 `$` 表示当前配置值，基于 expr 库可以实现丰富的判断规则。示例如下：

```go
// 端口必须大于 8080
gs.OnProperty("server.port").HavingValue("expr:int($) > 8080")
// 端口必须在 1024 到 65535 之间
gs.OnProperty("server.port").HavingValue("expr:int($) > 1024 && int($) < 65535")
// 环境必须不是生产环境
gs.OnProperty("app.env").HavingValue(`expr:$ != "prod"`)
```

expr 表达式必须返回布尔值。如果表达式解析失败，或者类型不匹配，或者返回值不是布尔值，会报错并终止启动。

如果判断逻辑比较复杂，使用 expr 表达式很难写，或者非常复杂，我们可以使用自定义校验函数。

举个例子。

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

在上面的代码中，我们注册了一个 `isValidPort` 函数，用于判断端口是否在 1024 到 65535 之间。然后，我们在注册 `NewServer` 时，使用 `expr:isValidPort($` 表达式，判断端口是否在有效范围内。

## 基于 Bean 的条件

除了根据配置项决定 bean 是否创建，我们还可能根据 bean 是否存在进行判断。这在 starter 和自动装配的场景中经常需要。比如。。。（提供一个更有说服性的实例，尤其是来自真实 starter 的案例）（应用提供了更具体的实现，Starter 默认实现就退出；基础组件不存在，依赖它的增强组件也不必启用。）

看下示例。

```go
todo 换成上面更真实的 starter 示例
```

Go-Spring 提供了几种围绕 Bean 存在性的条件：

```go
gs.OnBean[*HttpServeMux]()
gs.OnMissingBean[UserService]()
gs.OnSingleBean[*DataSource]()
gs.OnBean[*DataSource]("master")
```

`OnBean` 表示至少存在一个匹配 Bean。`OnMissingBean` 表示不存在匹配 Bean。`OnSingleBean` 表示恰好存在一个匹配 Bean。`OnBean` 可以仅根据类型匹配，也可以根据类型和名称同时匹配。

需要说明的是，go-spring 在判断 bean 是否存在的时候，会跳过已经被删除的 Bean。。。

## 自定义条件

内置条件覆盖了大多数装配场景。如果规则确实无法用配置值或者 Bean 存在性表达，可以使用 `OnFunc`。

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

`OnFunc` 可以访问 `ConditionContext`，因此可以读取配置，也可以查找 Bean。它适合承载少量框架层装配规则，例如兼容旧配置项、组合多个配置来源，或者处理项目内部稳定的判断。

但 `OnFunc` 不应该变成业务初始化入口。条件函数应该轻量、确定、错误来源清楚。访问外部服务、查询数据库、执行迁移任务这类逻辑，应该放回应用启动流程或者业务初始化流程里，而不是放在条件判断里。

## 条件组合

一个 Bean 可以挂多个条件，多个条件默认都要成立。对于需要明确表达组合关系的场景，Go-Spring 提供了 `And`、`Or`、`Not` 和 `None`。

某个指标组件既要求配置开关打开，又要求 HTTP 入口已经存在，可以写成：

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

组合条件可以继续嵌套：

```go
gs.And(
	gs.OnProperty("service.enabled").HavingValue("true"),
	gs.Or(
		gs.OnBean[*PrimaryClient](),
		gs.OnBean[*BackupClient](),
	),
)
```

表达能力足够强以后，条件也容易写成难以推理的布尔表达式。更稳的方式，是让条件读起来像装配规则：这个 Bean 依赖哪个配置，依赖哪个基础 Bean，为什么兜底实现应该退出。组合条件一旦开始承载业务流程，就应该重新审视 Bean 边界或者配置建模。

## 条件结果缓存

有些条件会被多个 Bean 复用，而且计算成本比较高。这个时候可以使用 `OnOnce` 缓存条件结果。

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

`OnOnce` 会把传入条件按 `And` 语义组合起来，并缓存第一次匹配结果。后续再次判断同一个条件实例时，会直接返回缓存结果。第一次判断得到的错误也会被缓存，因此它适合确定、稳定、可复用的装配判断。

简单的 `OnProperty` 和 `OnBean` 通常不需要缓存。只有条件确实昂贵，或者同一个判断会被多处共享时，`OnOnce` 才能让语义更清楚。它优化的是条件计算本身，不会改变条件成立与否的规则。

## 条件装配边界

条件注册的价值，不是让业务代码少写几个 `if`，而是把可选能力、默认实现、依赖增强和环境差异放回启动期裁剪里。

配置条件回答“这个开关或最终配置值是否匹配”，Bean 存在性条件回答“本次容器里是否已经有可用候选”，自定义条件补充少量框架层判断，组合条件负责把这些判断组织成清晰的装配规则。

反过来，订单状态、用户类型、租户策略、请求参数这类运行期业务选择，不应该写进条件注册。它们每次请求都可能不同，而 Go-Spring 条件只在启动解析阶段决定 Bean 是否参与本次装配。

这个边界清楚以后，容器解析完成时，应用面对的就是一组已经确定的 Bean。运行期代码可以直接依赖这些组件，而不是反复处理半启用状态。
