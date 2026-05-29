# Go-Spring 实战第 15 课 —— Condition：根据配置或者环境激活 Bean

前面讲 Bean 注册时，我们看到 `gs.Provide()`、`gs.Module()` 和配置类方法都会先把 Bean 放进候选集合。但真实项目里，候选不等于本次启动一定要启用。

Redis 客户端通常只在配置了连接信息时注册，监控和调试组件通常由开关控制，Starter 会提供默认实现但又要允许应用覆盖，某些增强组件也只有在基础组件存在时才有意义。

如果这些判断散落在构造函数或者业务代码里，运行期就会出现一堆半启用组件：对象已经创建出来了，但内部还要反复判断配置、检查依赖、处理空值。Go-Spring 的条件注册，就是把这些启动前就能确定的判断放回容器解析阶段。

## 启动期裁剪

先看条件注册的最小语义。`NewMyService` 已经进入候选集合，但它是否参与本次装配，还要看 `my.condition` 是否存在。

```go
func init() {
	gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
}
```

Go-Spring 的条件注册把装配判断放在容器解析阶段。Bean 先进入候选集合，随后条件决定它是否参与本次装配。

条件成立，Bean 继续参与创建、注入和初始化；条件不成立，Bean 会被裁剪掉，后续依赖查找和重复 Bean 检查都不会再把它算进去；条件返回错误，本次启动直接失败，并带上对应条件信息。

也就是说，条件注册解决的是启动期 Bean 裁剪，不是业务运行期的 `if` 分支。容器解析完成以后，业务代码应该面对一组明确可用的组件，而不是反复处理半启用状态。

从实现上看，`Condition` 的核心就是一个匹配函数：

```go
type Condition interface {
	Matches(ctx ConditionContext) (bool, error)
}
```

`ConditionContext` 可以读取配置，也可以查找当前容器里仍然有效的 Bean 候选。`OnProperty`、`OnBean`、`OnMissingBean`、`OnSingleBean` 这些条件，都是围绕这两个入口展开。

`gs.Module()` 也可以带条件，而且判断发生得更早。模块条件不满足时，模块函数不会执行，这一组 Bean 甚至不会展开注册。普通 Bean 的条件则在解析阶段统一计算，条件不满足时从候选集合里退出。

## 基于配置的条件

最常见的条件来自配置。基础设施组件、可选插件、调试能力和灰度功能，通常都会有一个配置开关或者关键配置项。

`OnProperty` 可以表达三类常见规则：

```go
gs.OnProperty("spring.gorm.dsn")

gs.OnProperty("redis.enabled").HavingValue("true")

gs.OnProperty("feature.audit").MatchIfMissing()
```

`OnProperty("spring.gorm.dsn")` 表示配置项存在时条件成立。`HavingValue("true")` 表示配置项存在，并且最终值等于 `true` 时条件成立。`MatchIfMissing()` 适合默认启用的能力：配置不存在时也成立，应用可以通过显式配置切换到另一种行为。

这里判断的是 Go-Spring 合并后的最终配置，而不是某一个单独来源。命令行参数、环境变量、Profile 配置、基础配置和代码默认值完成合并以后，条件才会基于最终结果做判断。因此，同一个 Bean 在不同启动参数下可以得到不同装配结果，但规则仍然集中在注册语句上。

如果配置项不是叶子值，`HavingValue` 无法拿到简单字符串值，条件会返回错误并中断启动。这比把一个结构节点误当成字符串继续运行更直接，也更容易排查。

当规则不是简单等值判断时，`HavingValue` 支持 `expr:` 前缀表达式。表达式里的 `$` 是当前配置值，类型是字符串，所以数值比较要显式转换。

```go
gs.OnProperty("server.port").HavingValue("expr:int($) > 8080")

gs.OnProperty("server.port").HavingValue("expr:int($) > 1024 && int($) < 65535")

gs.OnProperty("app.env").HavingValue(`expr:$ != "prod"`)
```

表达式必须返回布尔值。表达式解析失败、类型不匹配或者返回值不是布尔值，都会作为条件错误向上传递，最终让启动失败。

如果同一段判断会在多处复用，可以注册自定义表达式函数。下面的例子把端口判断收敛成 `isValidPort`：

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

自定义函数应该靠近配置语义，例如端口范围、枚举值、字符串格式这类装配规则。如果函数开始访问业务数据库、调用远程接口，或者承载一段业务流程，就已经超出了条件注册的边界。

## 基于 Bean 存在性的条件

配置只能回答“开关是否打开”“值是否匹配”。Starter 和自动装配还经常需要回答另一个问题：应用是否已经提供了某类 Bean。

最典型的场景是默认实现。组件包希望开箱即用，但又不能挡住应用自己的实现。

```go
type UserService interface {
	FindUser(id int64) (*User, error)
}

func NewDefaultUserService() UserService {
	return &DefaultUserService{}
}

func init() {
	gs.Provide(NewDefaultUserService).
		Condition(gs.OnMissingBean[UserService]())
}
```

如果应用已经注册了 `UserService`，默认实现的条件就不成立，`NewDefaultUserService` 不会参与本次装配。这个过程发生在解析阶段，不是先创建默认实现，再在运行期替换掉。

Go-Spring 提供了几种围绕 Bean 存在性的条件：

```go
gs.OnBean[*HttpServeMux]()
gs.OnMissingBean[UserService]()
gs.OnSingleBean[*DataSource]()
gs.OnBean[*DataSource]("master")
```

`OnBean` 表示至少存在一个匹配 Bean。`OnMissingBean` 表示不存在匹配 Bean。`OnSingleBean` 表示恰好存在一个匹配 Bean。最后一种写法同时按类型和名称匹配，适合多实例资源，例如多个数据源、多个 Redis 客户端或者多个 HTTP 客户端。

这里的“存在”不是简单看注册表里有没有候选。条件查找会跳过已经被删除的 Bean，也会先解析匹配候选自己的条件。也就是说，`OnBean` 系列判断的是本次解析后仍然有效的候选，而不是所有曾经注册过的声明。

这个语义对 Starter 很重要。默认实现、增强组件、适配器组件都可以通过 Bean 存在性条件跟应用代码协作：应用提供了更具体的实现，Starter 默认实现就退出；基础组件不存在，依赖它的增强组件也不必启用。

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
