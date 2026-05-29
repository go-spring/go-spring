# Go-Spring 实战第 15 课 —— Condition：根据配置或者环境激活 Bean

前面的内容中，我们多次提到过可以使用条件（Condition）来激活 Bean。那么本篇咱们就来详细聊聊条件，它有哪些类型，以及如何使用。

真实项目里的组件经常带有启用条件。Redis 客户端只在配置打开时注册，Starter 默认实现只在应用没有自定义实现时生效，某些日志或监控组件只在特定环境中启用。

Go-Spring 注册信息先进入容器，条件再决定哪些 Bean 参与本次启动。这样，业务运行期面对的是已经确定的组件集合，而不是一堆需要反复判断的半启用组件。

## 启动期裁剪

先看条件注册的基本语义。`NewMyService` 已经进入候选集合，但它是否参与本次启动，还要看 `my.condition` 是否存在。

```go
func init() {
	gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
}
```

条件在 Go-Spring 容器解析阶段执行。满足条件的 Bean 会保留，不满足条件的 Bean 会被裁剪，不再参与创建、注入和初始化。

这个边界很重要。条件注册解决的是启动期 Bean 裁剪，不是业务运行期的 if 分支。容器解析完成以后，业务代码应该面对明确的可用组件。

## 配置条件

最常见的条件来自配置。基础设施组件、可选插件和调试能力通常都由配置开关控制。`OnProperty` 可以表达三类常见规则：配置存在、配置值匹配，以及缺失时仍然匹配。

```go
gs.OnProperty("enable.redis")

gs.OnProperty("env").HavingValue("prod")

gs.OnProperty("optional.feature").MatchIfMissing()
```

`OnProperty("enable.redis")` 表示配置项存在时条件成立。`HavingValue("prod")` 表示配置值匹配时成立。`MatchIfMissing()` 则适合默认启用的能力：配置不存在时也成立，应用可以通过显式配置关闭或切换。

也就是说，`OnProperty` 判断的是配置树里的最终值，而不是某个单一配置来源。命令行参数、环境变量、Profile 配置和基础配置已经完成合并以后，条件才会基于合并结果做装配判断。

当条件不是简单等值判断时，`HavingValue` 支持使用 `expr:` 前缀表达式。下面这个条件不是检查端口是否存在，而是检查最终端口值是否落在指定范围之外。

```go
gs.OnProperty("server.port").HavingValue("expr:$ > 8080")
```

表达式适合描述少量和装配直接相关的判断。常见规则可以直接写成端口范围、环境排除、字符串前缀和列表包含关系。

```go
gs.OnProperty("server.port").HavingValue("expr:$ > 1024 && $ < 65535")
gs.OnProperty("app.env").HavingValue("expr:$ != 'prod'")
gs.OnProperty("app.base-url").HavingValue("expr:startsWith($, 'http://')")
gs.OnProperty("app.features").HavingValue("expr:contains($, 'debug')")
```

这些表达式仍然属于装配规则。它们决定 Bean 是否参与本次启动，而不是替代业务运行期的分支逻辑。表达式越接近配置含义，后续排查条件为什么成立或不成立时就越清楚。

如果装配规则需要一个可复用判断，也可以注册自定义表达式函数。下面的例子把端口判断注册成 `isValidPort`，避免多个 Bean 重复书写同一段表达式。

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

## Bean 存在性

Starter 常见的需求是提供默认实现，同时允许应用自己覆盖。这个判断不能只看配置，还要看本次启动中是否已经存在某类 Bean。

`OnMissingBean` 可以把默认实现留给应用覆盖。

```go
func init() {
	gs.Provide(NewDefaultUserService).
		Condition(gs.OnMissingBean[UserService]())
}
```

如果应用已经提供了 `UserService`，默认实现就不会参与本次启动。这样组件包能提供开箱即用的能力，同时把覆盖权留给应用。条件判断发生在解析阶段，因此默认实现不会先创建出来再被替换。

Go-Spring 还提供了几种围绕 Bean 存在性的条件。

```go
gs.OnBean[*UserService]()
gs.OnMissingBean[*UserService]()
gs.OnSingleBean[*UserService]()
gs.OnBean[*DataSource]("master")
```

它们分别表示至少存在一个匹配 Bean、不存在匹配 Bean、恰好存在一个匹配 Bean，以及按类型和名称同时匹配。

`OnBean` 系列适合装配层面的 Bean 判断。它回答的是“本次容器里有没有这个候选”，不是运行期某个对象当前是否可用。

## 自定义条件

如果配置条件和 Bean 条件都不够，可以用 `OnFunc` 承载自定义判断。它适合少量需要访问 Go-Spring 条件上下文的装配规则。

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	return myCustomCheck(ctx)
})
```

`OnFunc` 可以访问配置、Bean 定义或外部上下文。它应该补充 Go-Spring 内置条件表达不了的装配规则，而不是把一段业务流程搬进条件函数。

如果一个条件函数里开始访问业务数据库、调用远程接口，或者执行一段业务流程，它就已经超出了条件注册的边界。启动期条件应该尽量确定、轻量，并且错误来源可解释。

## 组合规则

真实模块经常需要多个条件同时成立。例如某个服务既要打开配置开关，又要求另一个 Bean 已经存在。Go-Spring 提供了 `And`、`Or`、`Not`、`None` 来组合条件。

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

表达力足够强以后，条件也容易变成难以推理的布尔表达式。更稳的写法是让条件读起来像装配规则，而不是让读者在条件里还原一段业务流程。组合条件一旦变得复杂，通常应该回到模块边界上重新拆分 Bean 或配置开关。

## 条件缓存

如果条件计算复杂，并且会被多处复用，可以使用 `OnOnce` 缓存结果。下面的写法把两个条件包装成一次性计算的复合条件，适合多个 Bean 共享同一个装配判断。

```go
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

简单条件通常不需要缓存。条件成本较高、且同一判断会被多处复用时，再加上 `OnOnce`，语义会更清楚。`OnOnce` 优化的是条件计算本身，不会改变条件成立与否的规则。

## 条件注册

条件注册主要服务模块边界。适合条件化的对象通常是基础设施组件、Starter 默认实现、可选插件和环境相关实现。

配置条件负责回答“这个开关或环境是否匹配”，Bean 条件负责回答“应用是否已经提供了候选”，自定义条件只补充少量框架层判断。条件越靠近装配语义，Go-Spring 的装配结果就越容易预测。

条件注册的价值，不是让业务代码少写几个 `if`，而是把可选能力、默认实现和环境差异都放回启动期 Bean 裁剪里。Go-Spring 在解析阶段得到确定的 Bean 集合，运行期代码才能保持直接调用。
