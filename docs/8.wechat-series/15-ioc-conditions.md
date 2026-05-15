# Go-Spring 实战第 15 课：条件注册：配置开关、默认实现和环境差异如何裁剪对象图

Bean 定义进入 Go-Spring 容器以后，还不能直接等同于本次启动会创建这个对象。真实项目里的组件经常带有启用条件。

例如 Redis 客户端只在配置打开时注册，Starter 默认实现只在应用没有自定义实现时生效，某些日志或监控组件只在特定环境中启用。如果这些分支都写进业务代码，模块边界会很快混在一起。

Go-Spring 把“是否装配”留在启动解析阶段处理。注册信息先进入容器，条件再决定哪些 Bean 留在本次对象图里。

## 条件注册把是否装配留在解析阶段

注册 Bean 时可以追加条件。下面这个 Bean 只有在 `my.condition` 满足时才会参与后续装配。

```go
gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
```

条件在 Go-Spring 容器解析阶段执行。满足条件的 Bean 会保留，不满足条件的 Bean 会被裁剪，不再参与创建和注入。

这样一来，业务运行期看到的是已经裁剪后的对象图。业务代码不需要反复判断某个基础设施组件是否存在，装配结果在启动阶段就已经确定。

## OnProperty 适合配置开关和值匹配

最常见的条件来自配置。`OnProperty` 可以判断配置项是否存在，也可以判断配置值是否匹配。

```go
gs.OnProperty("enable.redis")

gs.OnProperty("env").HavingValue("prod")

gs.OnProperty("optional.feature").MatchIfMissing()
```

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

这些条件仍然属于装配规则。它们决定 Bean 是否参与本次启动，而不是替代业务运行期的分支逻辑。

## OnBean 让默认实现给应用覆盖空间

Starter 常见的需求是提供默认实现，同时允许应用自己覆盖。这个判断不能只看配置，还要看容器里是否已经有某类 Bean。

```go
gs.OnBean[*UserService]()
gs.OnMissingBean[*UserService]()
gs.OnSingleBean[*UserService]()
gs.OnBean[*DataSource]("master")
```

这些条件分别表示至少存在一个匹配 Bean、不存在匹配 Bean、恰好存在一个匹配 Bean，以及按类型和名称同时匹配。

典型用法是默认实现配合 `OnMissingBean`。如果应用已经提供了自定义实现，Starter 的默认 Bean 就不会进入本次对象图。这样组件包能提供开箱即用的能力，同时把覆盖权留给应用。

## OnFunc 只承载少量自定义装配判断

如果配置条件和 Bean 条件都不够，可以用 `OnFunc` 承载自定义判断。

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	return myCustomCheck(ctx)
})
```

`OnFunc` 适合少量需要访问配置、Bean 定义或外部上下文的装配判断。判断逻辑越接近“是否装配”，后续排查时越容易解释；如果函数里开始承载业务流程，就应该回到业务代码或模块初始化逻辑里处理。

## 组合条件要读得出装配规则

Go-Spring 提供了 `And`、`Or`、`Not`、`None` 来组合条件。组合后的表达式应该仍然能读出装配规则。

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

## OnOnce 只缓存高成本的复用判断

如果条件计算复杂，并且会被多处复用，可以使用 `OnOnce` 缓存结果。

```go
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

简单条件通常不需要缓存。条件成本较高、且同一判断会被多处复用时，再加上 `OnOnce`，语义会更清楚。

## 条件注册服务模块边界

Go-Spring 的条件注册主要服务模块边界。适合条件化的对象通常是基础设施组件、Starter 默认实现、可选插件和环境相关实现。

Profile 条件是条件注册里最常见的环境场景。它需要和配置 Profile 放在一起看，因为配置切换和 Bean 切换最好沿着同一套环境语义前进。
