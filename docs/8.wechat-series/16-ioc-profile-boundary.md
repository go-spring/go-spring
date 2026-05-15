# Go-Spring 实战第 16 课：Profile 装配边界：配置文件和 Bean 实现如何沿同一环境语义切换

同一个服务部署到开发、测试和生产环境时，变化通常不只发生在配置值上。开发环境可能使用本地日志实现，生产环境可能接入文件或远程日志；测试环境可能替换外部客户端，生产环境则使用真实连接。

如果配置文件按一套环境名切换，Bean 条件又按另一套名字切换，排查问题时就很难判断本次启动到底使用了哪组输入和哪组实现。

Go-Spring 的 Profile 同时影响配置加载和 Bean 装配。配置 Profile 决定读哪些配置，IoC Profile 条件决定启用哪些 Bean。两者沿同一条环境语义设计时，配置切换和实现切换才会对齐。

## spring.profiles.active 先决定加载哪些配置

Go-Spring 使用 `spring.profiles.active` 激活 Profile。命令行参数可以直接指定本次启动的环境。

```bash
./app -Dspring.profiles.active=prod
```

环境变量也可以表达同样语义。

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

基础配置会先加载，Profile 配置随后加载并覆盖差异项。多个 Profile 同时激活时，后面的优先级更高。

这个顺序决定了本次启动看到的配置输入。Bean 装配如果也使用 Profile，就应该沿着同一组 Profile 名称表达环境差异。

## OnProfiles 表达 Bean 是否属于某个环境

对于按环境启用 Bean 的场景，Go-Spring 提供了 `.OnProfiles()`。

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

它本质上基于 `spring.profiles.active` 判断。当前激活的 Profiles 中任意一个匹配时，条件成立。

相比手写 `OnProperty("spring.profiles.active")`，`.OnProfiles()` 更明确地表达了这个 Bean 属于 Profile 维度。读注册代码时，可以直接看出它不是普通配置开关，而是环境装配条件。

## 配置和实现要沿同一 Profile 维度变化

一个常见模式是 Profile 文件提供环境配置，Profile 条件选择环境实现。开发环境使用本地日志实现时，注册语句可以直接挂在 `dev` 上。

```go
func init() {
	gs.Provide(NewConsoleDebugLogger).OnProfiles("dev")
}
```

生产环境使用文件或远程日志实现时，对应 Bean 挂在 `prod` 上。

```go
func init() {
	gs.Provide(NewProdLogger).OnProfiles("prod")
}
```

同时，`app-prod.yaml` 只保存生产日志实现需要的差异配置。

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

这样一来，看到 `prod` 就能同时知道它影响了配置输入和对象装配。反过来说，如果配置切换和 Bean 切换各用一套名字，环境问题就很容易出现“配置已经切过去，实现还没切过去”的错位。

## 多个 Profile 只有正交后才适合组合

多个 Profile 可以同时激活，但每个 Profile 最好代表一个相对独立的维度。

```properties
spring.profiles.active=prod,metrics
```

这里 `prod` 表示部署环境，`metrics` 表示功能能力。对应的 Bean 条件也沿着这两个维度拆开。

```go
gs.Provide(NewProdDataSource).OnProfiles("prod")
gs.Provide(NewMetricsExporter).OnProfiles("metrics")
```

如果把 `prod-metrics`、`prod-debug`、`test-metrics` 都写成新的 Profile，组合数量会快速膨胀。只有某个组合确实代表独立部署形态时，单独建 Profile 才更清楚。

## Profile 解决环境语义，普通条件解决装配规则

`.OnProfiles()` 是 Profile 场景下的便捷条件。更复杂的装配仍然可以使用 `.Condition()`。

```go
gs.Provide(NewService).Condition(gs.And(
	gs.OnProperty("feature.enabled").HavingValue("true"),
	gs.OnBean[*Dependency](),
))
```

Profile 更适合描述环境和功能维度；普通条件更适合配置开关、依赖存在性和默认实现选择。两类条件可以组合，但语义要分清楚。

## Profile 不应该承载运行期业务分支

Profile 描述的是部署语义，例如部署环境、能力开关和基础设施组合。订单状态、用户类型、租户策略这类运行期业务分支，通常应该留在业务代码里表达。

当注册、条件和 Profile 都确定以后，这些信息还要在 Go-Spring 容器运行阶段被合并、裁剪、注入、运行和销毁。下一步就要把这些阶段连成完整的启动流程来看。
