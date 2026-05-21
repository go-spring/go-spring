# Go-Spring 实战第 15 课 —— 条件注册：配置开关、默认实现和环境差异如何裁剪对象图

Bean 定义进入 Go-Spring 容器以后，还不能直接等同于本次启动会创建这个对象。真实项目里的组件经常带有启用条件。

例如 Redis 客户端只在配置打开时注册，Starter 默认实现只在应用没有自定义实现时生效，某些日志或监控组件只在特定环境中启用。如果这些分支都写进业务代码，模块边界会很快混在一起。

Go-Spring 把“是否装配”留在启动解析阶段处理。注册信息先进入容器，条件再决定哪些 Bean 留在本次对象图里。这样，业务运行期看到的是已经裁剪后的对象图，而不是一堆需要反复判断的半启用组件。

## 条件注册

先看一个证明条件注册基本语义的例子。`NewMyService` 已经注册，但只有 `my.condition` 存在时才会参与后续装配。

```go
func init() {
	gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
}
```

条件在 Go-Spring 容器解析阶段执行。满足条件的 Bean 会保留，不满足条件的 Bean 会被裁剪，不再参与创建和注入。

这个边界很重要。条件注册解决的是启动期对象图裁剪，不是业务运行期的 if 分支。对象图确定以后，业务代码应该面对明确的依赖关系。

## OnProperty

最常见的条件来自配置。下面这组例子证明 `OnProperty` 可以判断配置项是否存在，也可以判断配置值是否匹配。

```go
gs.OnProperty("enable.redis")

gs.OnProperty("env").HavingValue("prod")

gs.OnProperty("optional.feature").MatchIfMissing()
```

`OnProperty("enable.redis")` 表示配置项存在时条件成立。`HavingValue("prod")` 表示配置值匹配时成立。`MatchIfMissing()` 则适合默认启用的能力：配置不存在时也成立，应用可以通过显式配置关闭或切换。

当条件不是简单等值判断时，`HavingValue` 支持使用 `expr:` 前缀表达式。

```go
gs.OnProperty("server.port").HavingValue("expr:$ > 8080")
```

常见表达式可以直接描述端口范围、环境排除、字符串前缀和列表包含关系。

```go
gs.OnProperty("server.port").HavingValue("expr:$ > 1024 && $ < 65535")
gs.OnProperty("app.env").HavingValue("expr:$ != 'prod'")
gs.OnProperty("app.base-url").HavingValue("expr:startsWith($, 'http://')")
gs.OnProperty("app.features").HavingValue("expr:contains($, 'debug')")
```

这些表达式仍然属于装配规则。它们决定 Bean 是否参与本次启动，而不是替代业务运行期的分支逻辑。

如果装配规则需要一个可复用判断，也可以注册自定义表达式函数。

```go
func init() {
	gs.RegisterExpressFunc("isValidPort", func(port int) bool {
		return port > 1024 && port < 65535
	})

	gs.Provide(NewServer).Condition(
		gs.OnProperty("server.port").HavingValue("expr:isValidPort($)"),
	)
}
```

自定义函数适合复用少量稳定判断。函数越接近配置语义，条件越容易排查；如果函数开始承载业务流程，就应该回到业务代码或模块初始化逻辑里。

## OnBean

Starter 常见的需求是提供默认实现，同时允许应用自己覆盖。这个判断不能只看配置，还要看容器里是否已经有某类 Bean。

下面的例子证明 `OnMissingBean` 可以把默认实现留给应用覆盖。

```go
func init() {
	gs.Provide(NewDefaultUserService).
		Condition(gs.OnMissingBean[UserService]())
}
```

如果应用已经提供了 `UserService`，默认实现就不会进入本次对象图。这样组件包能提供开箱即用的能力，同时把覆盖权留给应用。

Go-Spring 还提供了几种围绕 Bean 存在性的条件。

```go
gs.OnBean[*UserService]()
gs.OnMissingBean[*UserService]()
gs.OnSingleBean[*UserService]()
gs.OnBean[*DataSource]("master")
```

它们分别表示至少存在一个匹配 Bean、不存在匹配 Bean、恰好存在一个匹配 Bean，以及按类型和名称同时匹配。

`OnBean` 系列适合装配层面的依赖判断。它回答的是“本次对象图里有没有这个候选”，不是运行期某个对象当前是否可用。

## OnFunc

如果配置条件和 Bean 条件都不够，可以用 `OnFunc` 承载自定义判断。下面的例子证明自定义条件可以访问 Go-Spring 提供的条件上下文。

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	return myCustomCheck(ctx)
})
```

`OnFunc` 适合少量需要访问配置、Bean 定义或外部上下文的装配判断。判断逻辑越接近“是否装配”，后续排查时越容易解释。

如果一个条件函数里开始访问业务数据库、调用远程接口，或者执行一段业务流程，它就已经超出了条件注册的边界。启动期条件应该尽量确定、轻量，并且错误来源可解释。

## 组合条件

真实模块经常需要多个条件同时成立。Go-Spring 提供了 `And`、`Or`、`Not`、`None` 来组合条件。下面的例子证明组合条件应该仍然读得出装配规则。

```go
gs.Provide(NewService).Condition(gs.And(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))

gs.Provide(NewService).Condition(gs.Or(
	gs.OnProperty("profile.dev"),
	gs.OnProperty("profile.test"),
))

gs.Provide(NewFallbackService).Condition(gs.Not(
	gs.OnBean[RealService](),
))

gs.Provide(NewService).Condition(gs.None(
	gs.OnProperty("profile.dev"),
	gs.OnProperty("profile.test"),
))
```

组合条件可以继续嵌套。

```go
gs.And(
	gs.OnProperty("env").HavingValue("prod"),
	gs.Or(
		gs.OnProperty("enable.a"),
		gs.OnProperty("enable.b"),
	),
)
```

表达力足够强以后，条件也容易变成难以推理的布尔表达式。更稳的写法是让条件读起来像装配规则，而不是让读者在条件里还原一段业务流程。

## OnOnce

如果条件计算复杂，并且会被多处复用，可以使用 `OnOnce` 缓存结果。下面的例子证明多个条件可以被包装成一次性计算的复合条件。

```go
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

简单条件通常不需要缓存。条件成本较高、且同一判断会被多处复用时，再加上 `OnOnce`，语义会更清楚。

## 条件注册

条件注册主要服务模块边界。适合条件化的对象通常是基础设施组件、Starter 默认实现、可选插件和环境相关实现。

配置条件负责回答“这个开关或环境是否匹配”，Bean 条件负责回答“应用是否已经提供了候选”，自定义条件只补充少量框架层判断。条件越靠近装配语义，Go-Spring 裁剪对象图的结果就越容易预测。

Profile 条件是条件注册里最常见的环境场景。它需要和配置 Profile 放在一起看，因为配置切换和 Bean 切换最好沿着同一套环境语义前进。
