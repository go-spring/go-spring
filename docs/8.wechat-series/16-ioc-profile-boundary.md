# Go-Spring 实战第 16 课 —— Profile 装配边界：配置文件和 Bean 实现如何对齐切换

上一篇讲条件注册时，我们已经看到 Bean 定义进入容器以后，还会在解析阶段被条件裁剪。真实项目里最常见的一类条件，往往不是单个功能开关，而是环境。

同一个服务部署到开发、测试和生产环境时，变化通常不只发生在配置值上。开发环境可能使用控制台日志，生产环境可能接入文件或远程日志；测试环境可能替换外部客户端，生产环境则使用真实连接。如果配置文件按一套环境名切换，Bean 条件又按另一套名字切换，排查问题时就很难判断本次启动到底使用了哪组输入和哪组实现。

Go-Spring 的 Profile 同时连接配置加载和 Bean 装配。配置 Profile 决定叠加哪些 `app-{profile}.*` 文件，IoC Profile 条件决定哪些 Bean 参与本次对象图。两者沿同一套环境语义前进时，配置切换和实现切换才不会错位。

## spring.profiles.active

Go-Spring 使用 `spring.profiles.active` 表达当前激活的 Profile。这个值通常来自命令行参数或环境变量，因为它们最接近一次具体启动。

```bash
./app -Dspring.profiles.active=prod
```

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

这两个写法最终都会进入同一个配置 key。配置文件则按照 Profile 名称组织。

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

这里要证明的是：Profile 不是只服务配置文件命名。`prod` 一旦成为本次启动的环境语义，它后续也应该被 Bean 装配条件复用。基础配置先加载，Profile 配置随后叠加差异；如果同时激活多个 Profile，Go-Spring 按声明顺序加载，后面的 Profile 可以覆盖前面的同名 key。

也就是说，`spring.profiles.active=prod` 先决定了本次启动的配置输入，随后同一个 `prod` 还可以继续决定哪些环境实现进入容器。

## OnProfiles

按环境启用 Bean 时，可以使用 `.OnProfiles()`。下面这个例子证明的是：Bean 是否属于某个环境，不需要手写成普通配置开关。

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

`.OnProfiles("dev")` 基于当前激活的 `spring.profiles.active` 判断。只要注册语句声明的 Profile 与当前激活 Profiles 中任意一个匹配，这个 Bean 条件就成立。

它和 `OnProperty("spring.profiles.active")` 能表达相近结果，但语义不同。`.OnProfiles()` 直接说明这个 Bean 属于 Profile 维度；读注册代码时，可以知道它不是普通业务开关，也不是依赖存在性判断，而是环境装配规则。

## 配置和实现

更完整的用法，是让 Profile 文件提供环境差异配置，让 Profile 条件选择对应实现。下面这个例子要证明的是：同一个 Profile 名称可以同时约束配置输入和 Bean 实现。

开发环境注册一个控制台日志实现。

```go
func init() {
	gs.Provide(NewConsoleDebugLogger).OnProfiles("dev")
}
```

生产环境注册生产日志实现。

```go
func init() {
	gs.Provide(NewProdLogger).OnProfiles("prod")
}
```

生产环境需要的差异配置放在 `app-prod.yaml`。

```yaml
log:
  dir: /var/log/app
  level: info
```

激活 `prod` 后，Go-Spring 会叠加 `app-prod.yaml`，同时让 `NewProdLogger` 对应的 Bean 参与装配。这样看到 `prod` 时，就能同时理解配置输入和对象实现的变化。

如果配置文件叫 `prod`，而 Bean 条件写成 `online`、`release` 或其他临时名字，系统仍然可以运行，但排查时会出现额外映射关系。更麻烦的是，配置已经切到生产差异，Bean 实现却可能因为条件名称不一致没有切过去。

## Profile 组合

Go-Spring 允许同时激活多个 Profile。下面这个例子要证明的是：多个 Profile 适合表达正交维度，而不是把所有组合都变成新名字。

```properties
spring.profiles.active=prod,metrics
```

对应的 Bean 条件可以沿两个维度拆开。

```go
func init() {
	gs.Provide(NewProdDataSource).OnProfiles("prod")
	gs.Provide(NewMetricsExporter).OnProfiles("metrics")
}
```

这里 `prod` 表示部署环境，`metrics` 表示观测能力。它们可以组合，是因为两个 Profile 回答的问题不同。前者决定生产环境下的基础设施差异，后者决定是否启用指标能力。

如果把 `prod-metrics`、`prod-debug`、`test-metrics` 都写成独立 Profile，组合数量会很快膨胀。只有某个组合确实代表稳定部署形态时，单独建 Profile 才更清楚。否则，拆成正交 Profile，再让配置文件和 Bean 条件分别沿这些维度表达差异，会更容易维护。

## 普通条件

Profile 适合表达环境和能力维度，但它不应该替代所有条件。下面这个例子要证明的是：环境语义可以和普通装配规则组合，但两者承担的职责不同。

```go
func init() {
	gs.Provide(NewService).Condition(gs.And(
		gs.OnProperty("feature.enabled").HavingValue("true"),
		gs.OnBean[*Dependency](),
	))
}
```

`OnProperty` 更适合配置开关和值匹配，`OnBean` 更适合依赖存在性和默认实现选择，`.OnProfiles()` 更适合环境或能力维度。它们都发生在 Bean 解析阶段，但读代码时应该能分辨出条件背后的原因。

Profile 也不适合承载运行期业务分支。订单状态、用户类型、租户策略这类问题，通常应该留在业务代码里表达。Profile 描述的是本次启动的部署语义，而不是每一次请求里的业务选择。

## Profile 装配边界

Profile 装配边界的核心，是让同一个环境语义同时约束配置输入和 Bean 对象图。`spring.profiles.active` 决定加载哪些 Profile 配置，`.OnProfiles()` 决定哪些环境实现参与装配。

这条边界清楚以后，环境差异不会散落在配置文件名、普通开关和业务分支里。Go-Spring 会在启动解析阶段把配置和 Bean 候选都裁剪到当前 Profile 对应的形态，后续容器运行流程就可以基于这个确定结果继续推进。
