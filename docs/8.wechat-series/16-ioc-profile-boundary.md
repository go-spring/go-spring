# Go-Spring 实战第 16 课：Profile 装配边界：配置切换和 Bean 切换如何保持一致

我们在 Go-Spring 配置篇已经看过 Profile，它决定本次启动应该加载哪些环境配置。到了 IoC 这里呢，Profile 还有另一层含义，即它也可以决定哪些 Bean 参与装配。

可以先这样理解——Go-Spring 配置 Profile 决定“读哪些配置”，IoC Profile 条件决定“启用哪些 Bean”。两者沿着同一条环境语义设计时，配置切换和实现切换会更容易对齐。

## 先对齐配置 Profile 的语义

Go-Spring 使用 `spring.profiles.active` 激活 Profile。

```bash
./app -Dspring.profiles.active=prod
```

也可以这样写。

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

配置文件遵循命名约定。

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

基础配置先加载，Profile 配置后加载并覆盖差异项。多个 Profile 同时激活时，后面的优先级更高。

## OnProfiles 表达 Bean 的环境条件

对于按环境启用 Bean 的场景，Go-Spring 提供了 `.OnProfiles()`。

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

它本质上基于 `spring.profiles.active` 判断。当前激活的 Profiles 中任意一个匹配时，条件成立。

这样比手写 `OnProperty("spring.profiles.active")` 会更直接，因为它更清楚地表达了“这是 Profile 维度的装配条件”。

## 配置和实现要沿同一维度切换

一个常见模式是，Profile 文件提供环境配置，Profile 条件选择环境实现。

例如开发环境使用本地日志实现。

```go
func init() {
	gs.Provide(NewConsoleDebugLogger).OnProfiles("dev")
}
```

生产环境使用文件或远程日志实现。

```go
func init() {
	gs.Provide(NewProdLogger).OnProfiles("prod")
}
```

同时，`app-dev.yaml` 和 `app-prod.yaml` 只保存对应实现需要的差异配置。

例如生产配置只描述生产日志需要的参数。

```yaml
log:
  dir: /var/log/app
  level: info
```

对应的生产 Bean 只在 `prod` 下装配。

```go
func init() {
	gs.Provide(NewProdLogger).OnProfiles("prod")
}
```

这样配置选择和 Bean 选择是对齐的。看到 `prod` 时，我们就能同时知道它影响了配置输入和对象装配。反过来，如果配置切换和 Bean 切换各用一套名字，排查环境问题时就很容易对不上。

## 多个 Profile 正交后才好组合

多个 Profile 可以表达多个维度。

```properties
spring.profiles.active=prod,metrics
```

这里 `prod` 表示环境，`metrics` 表示功能。对应的 Bean 条件也沿着这两个维度拆开。

```go
gs.Provide(NewProdDataSource).OnProfiles("prod")
gs.Provide(NewMetricsExporter).OnProfiles("metrics")
```

如果把 `prod-metrics` 这类组合都写成新的 Profile，组合数量会快速膨胀。只有它确实代表独立部署形态时，单独建 Profile 才更清楚。

## Profile 是条件注册里的环境特例

`.OnProfiles()` 是 Profile 场景下的便捷条件。更复杂的装配仍然可以使用 `.Condition()`。

```go
gs.Provide(NewService).Condition(gs.And(
	gs.OnProperty("feature.enabled").HavingValue("true"),
	gs.OnBean[*Dependency](),
))
```

Profile 更适合环境和功能维度；普通条件更适合配置开关、依赖存在性和默认实现选择。

## Profile 主要用来描述部署语义

Profile 适合描述部署环境、能力开关和基础设施组合。不过，订单状态、用户类型、租户策略这类运行期业务分支，通常会留在业务代码里表达。

当注册、条件和 Profile 都确定以后，这些信息还要在容器运行阶段被合并、裁剪、注入、运行和销毁。
