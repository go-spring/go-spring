# Go-Spring 实战第 16 课：Profile 装配边界：配置切换和 Bean 切换如何保持一致

我们在 Go-Spring 配置篇已经看过 Profile：它决定本次启动应该加载哪些环境配置。到了 IoC 这里呢，Profile 还有另一层含义：它也可以决定哪些 Bean 参与装配。

可以先这样理解：Go-Spring 配置 Profile 决定“读哪些配置”，IoC Profile 条件决定“启用哪些 Bean”。两者应该沿着同一条环境语义设计，否则很容易出现配置已经切换、实现却没有切换，或者实现切换了但配置仍然来自旧环境的情况。

## 先回到配置 Profile 的语义

Go-Spring 使用 `spring.profiles.active` 激活 Profile：

```bash
./app -Dspring.profiles.active=prod
```

或：

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

配置文件遵循命名约定：

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

基础配置先加载，Profile 配置后加载并覆盖差异项。多个 Profile 同时激活时，后面的优先级更高。

## 用 OnProfiles 表达 Bean 的环境条件

对于按环境启用 Bean 的场景，Go-Spring 提供 `.OnProfiles()`：

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

它本质上基于 `spring.profiles.active` 判断。当前激活的 Profiles 中任意一个匹配时，条件成立。

这样比手写 `OnProperty("spring.profiles.active")` 会更直接，也更清楚地表达“这是 Profile 维度的装配条件”。

## 配置和实现要沿同一维度切换

一个常见模式是：Profile 文件提供环境配置，Profile 条件选择环境实现。

例如开发环境使用本地日志实现：

```go
func init() {
	gs.Provide(NewConsoleDebugLogger).OnProfiles("dev")
}
```

生产环境使用文件或远程日志实现：

```go
func init() {
	gs.Provide(NewProdLogger).OnProfiles("prod")
}
```

同时，`app-dev.yaml` 和 `app-prod.yaml` 只保存对应实现需要的差异配置。

这样配置选择和 Bean 选择是对齐的。也就是说，我们看到 `prod`，就能同时理解它影响了配置输入和对象装配。

## 多个 Profile 也要保持正交

多个 Profile 可以表达多个维度：

```properties
spring.profiles.active=prod,metrics
```

这里 `prod` 表示环境，`metrics` 表示功能。对应的 Bean 条件也应保持正交：

```go
gs.Provide(NewProdDataSource).OnProfiles("prod")
gs.Provide(NewMetricsExporter).OnProfiles("metrics")
```

不要把 `prod-metrics` 这类组合写成新的 Profile，除非它确实代表独立部署形态。否则组合数量会快速膨胀，Profile 也会重新变成复制粘贴。

## Profile 是条件注册的特化场景

`.OnProfiles()` 是 Profile 场景下的便捷条件。更复杂的装配仍然可以使用 `.Condition()`：

```go
gs.Provide(NewService).Condition(gs.And(
	gs.OnProperty("feature.enabled").HavingValue("true"),
	gs.OnBean[*Dependency](),
))
```

所以建议把 Profile 用于环境和功能维度，把普通条件用于配置开关、依赖存在性和默认实现选择。

## Profile 不该承载业务规则

这里要特别注意：不要把 Profile 当成业务规则系统。Profile 适合描述部署环境、能力开关和基础设施组合，不适合表达订单状态、用户类型、租户策略这类运行期业务分支。

## 下一篇预告

下一篇会从容器运行阶段看这些注册信息如何被合并、裁剪、注入、运行和销毁。
