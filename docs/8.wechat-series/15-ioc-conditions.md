# Go-Spring 实战第 15 课：条件注册：配置开关、默认实现和环境差异如何裁剪 Bean

Bean 注册进 Go-Spring 容器之后，并不代表它就一定会在本次启动中生效。真实项目里，组件往往不是永远启用的。

某些 Bean 只在配置存在时启用，某些实现只在缺少默认实现时启用，某些模块只在特定环境中启用。如果这些分支全部写进业务代码，模块边界会很快变得混乱。Go-Spring 把“是否装配”放到了启动期判断。

Go-Spring 通过 Condition 机制表达这些装配规则。我们可以把条件注册理解成启动期的“装配裁剪”：注册信息先进入容器，解析阶段再根据条件决定哪些 Bean 留下来。

## 先看条件如何裁剪 Bean

注册 Bean 时可以追加条件：

```go
gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
```

条件在 Go-Spring 容器解析阶段执行。满足条件的 Bean 保留，不满足条件的 Bean 会被裁剪，不参与后续创建和注入。

这样一来，业务代码不需要在运行期反复判断组件是否存在。装配结果在启动时就已经确定。

## OnProperty 处理配置开关

先看最常用的 `OnProperty`。它可以判断配置项是否存在，也可以判断配置值是否匹配。

```go
gs.OnProperty("enable.redis")

gs.OnProperty("env").HavingValue("prod")

gs.OnProperty("optional.feature").MatchIfMissing()
```

`HavingValue` 支持表达式，使用 `expr:` 前缀：

```go
gs.OnProperty("server.port").HavingValue("expr:$ > 8080")
```

常见表达式：

```go
gs.OnProperty("server.port").HavingValue("expr:$ > 1024 && $ < 65535")
gs.OnProperty("app.env").HavingValue("expr:$ != 'prod'")
gs.OnProperty("app.base-url").HavingValue("expr:startsWith($, 'http://')")
gs.OnProperty("app.features").HavingValue("expr:contains($, 'debug')")
```

也可以注册自定义表达式函数：

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

## OnBean 处理默认实现和覆盖

有些自动配置需要根据 Go-Spring 容器中是否已有某个 Bean 决定是否启用：

```go
gs.OnBean[*UserService]()
gs.OnMissingBean[*UserService]()
gs.OnSingleBean[*UserService]()
gs.OnBean[*DataSource]("master")
```

语义分别是：

- `OnBean[T]()`：至少存在一个匹配 Bean。
- `OnMissingBean[T]()`：不存在匹配 Bean。
- `OnSingleBean[T]()`：恰好存在一个匹配 Bean。
- 可选名称参数用来同时按类型和名称匹配。

这类条件常见于 Starter 场景：如果应用已经提供自定义实现，Starter 就不再注册默认实现。这样组件包可以提供默认能力，同时给应用保留覆盖空间。

## OnFunc 接入少量自定义判断

简单自定义逻辑可以用 `OnFunc`：

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	return myCustomCheck(ctx)
})
```

如果条件需要访问配置、Bean 定义或外部上下文，可以封装成自定义函数条件。条件越接近装配规则，后续排查时也越容易解释。

## 组合条件读起来像装配规则

Go-Spring 提供了 `And`、`Or`、`Not`、`None`：

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

组合条件可以嵌套：

```go
gs.And(
	gs.OnProperty("env").HavingValue("prod"),
	gs.Or(
		gs.OnProperty("enable.a"),
		gs.OnProperty("enable.b"),
	),
)
```

表达力足够强以后，条件也容易写成难以推理的布尔表达式。更理想的状态是：读条件时能看出装配规则，而不是在里面还原一段业务流程。

## OnOnce 用来复用高成本条件

如果条件计算复杂且需要复用，可以使用 `OnOnce` 缓存结果：

```go
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

简单条件通常不需要缓存。条件成本较高、且被多处复用时，再加上 `OnOnce`。

## 条件注册要服务模块边界

Go-Spring 的条件注册主要服务于模块边界。适合条件化的对象通常是基础设施组件、Starter 默认实现、可选插件和环境相关实现。

Profile 条件是条件注册里最常见的环境场景，需要和配置 Profile 放在一起看，避免配置切换和实现切换各走各的。
