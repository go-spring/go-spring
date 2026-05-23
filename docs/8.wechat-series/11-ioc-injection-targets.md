# Go-Spring 实战第 11 课 —— 依赖注入的目标：单 Bean 注入和集合注入

上一课解决的是依赖写在哪里。字段注入也好，构造函数注入也好，依赖点一旦出现，Go-Spring 容器还要继续判断：这个位置应该拿到哪个 Bean。

在简单服务里，一个类型通常只有一个实现，按类型注入就够了。服务继续演进以后，同一个接口可能有主从两个数据源，某个能力可能只是可选插件，一条过滤器链也可能来自多个 Bean。此时，问题就不再是“能不能注入”，而是“怎么明确选择注入目标”。

Go-Spring 的规则很直接：能按类型唯一匹配时，就让类型承担选择语义；同类型出现多个候选时，再用名称表达差异；这个依赖本来就可以不存在时，才用 `?` 声明可选；需要多个实现时，则用 slice 或 Map 承接集合注入。

## 类型匹配

先看最基础的注入语义：没有指定名称时，Go-Spring 会按字段类型查找唯一候选。

```go
type Service struct {
	Repo UserRepository `autowire:""`
}
```

构造函数参数也使用同样规则。只要 `UserService` 能唯一匹配，注册时就不需要额外声明参数标签。

```go
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

这里类型就是选择条件。注册方提供一个 `UserService`，依赖方声明一个 `UserService`，Go-Spring 只需要确认候选唯一，然后完成注入。这个方式最简单，也最适合核心业务链路。

## 名称匹配

当同类型 Bean 出现多个时，类型已经不足以表达选择意图。下面的例子证明名称会成为注入声明的一部分。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Repository struct {
	DS DataSource `autowire:"slave"`
}
```

`autowire:"slave"` 表示依赖方不仅要求类型匹配，还要求 Bean 名称是 `slave`。构造函数参数中也通过 `TagArg` 表达同样选择。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

名称一旦进入注入声明，就不再只是展示信息。它会成为注册方和依赖方之间的约定，因此名称要稳定、可读，并且能表达实例差异。像 `master`、`slave`、`primary`、`readonly` 这类名称，比 `db1`、`db2` 更容易长期维护。

## 可选依赖

默认情况下，找不到匹配 Bean 会让应用启动失败。这个默认值很重要，因为核心依赖缺失应该停在启动阶段。

如果某个依赖确实只是增强能力，缺失语义必须写进声明里。下面的例子证明 `?` 会把“没有候选”从启动错误变成可接受结果。

```go
type Service struct {
	OptionalDep Dep `autowire:"?"`
	NamedDep    Dep `autowire:"my-name?"`
}
```

构造函数参数也可以使用相同语义。

```go
func NewService(dep Dep, named Dep) *Service {
	return &Service{OptionalDep: dep, NamedDep: named}
}

func init() {
	gs.Provide(NewService, gs.TagArg("?"), gs.TagArg("my-name?"))
}
```

可选依赖适合插件式能力、可选增强或兼容旧系统的过渡场景。反过来，如果核心依赖也标成可选，错误就会从启动阶段推迟到业务运行时，所以这里表达的是“能力可选”，不是“找不到也无所谓”。

## 集合注入

有些依赖点需要的不是一个 Bean，而是一组实现。下面的例子证明 `[]T` 可以收集所有匹配 Bean。

```go
type Application struct {
	Plugins []Plugin `autowire:""`
}

func init() {
	gs.Provide(NewPluginA).Export(gs.As[Plugin]())
	gs.Provide(NewPluginB).Export(gs.As[Plugin]())
	gs.Provide(NewPluginC).Export(gs.As[Plugin]())
}
```

构造函数参数也可以声明 `[]Plugin`，Go-Spring 会按同样规则收集候选。

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{Plugins: plugins}
}

func init() {
	gs.Provide(NewApplication)
}
```

未指定标签内容时，Go-Spring 会收集所有匹配 Bean，并按 Bean 名称的字典序放入 slice。这样做的重点不是表达业务顺序，而是让每次启动得到一致结果。

如果执行顺序本身有业务含义，就不要依赖默认收集规则，而应该在注入声明中显式列出名称。

```go
type Chain struct {
	Filters []Filter `autowire:"auth?,tracing,recovery"`
}
```

列表中可以使用 `name?` 表示不存在时跳过，也可以使用一次 `*` 表示收集剩余未显式列出的 Bean。

```go
type Chain struct {
	Filters []Filter `autowire:"auth,*,recovery"`
}
```

这里的语义是：`auth` 固定在最前，`recovery` 固定在最后，`*` 补齐其他同类型 Bean。`*` 匹配到的部分仍然按 Bean 名称排序。只要顺序会影响行为，就应该让顺序出现在标签或配置里，而不是让读者从注册位置里推断。

`map[string]T` 也能收集多个 Bean。下面的例子证明 Bean 名称会成为 Map 的 key。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

Map 更适合按名称查找或导出注册表，不适合表达执行顺序。需要限制 Map 中的条目时，也可以在标签里列出名称。

```go
type Router struct {
	Handlers map[string]Handler `autowire:"auth,user?,order"`
}
```

这里 `user?` 表示 `user` 这个 Handler 不存在时跳过。虽然 Map 注入也能使用 `*`，但 Map 本身没有顺序，只有在“收集剩余 Bean”这个语义确实有用时才值得使用。

## 配置驱动

有些选择不能固定在代码里。例如不同部署环境要切换存储后端，或者不同站点要启用不同过滤器链。这时，注入标签可以写成 `${...}`，把 Bean 名称从配置中读出来。

```go
type Service struct {
	Storage Storage `autowire:"${storage.provider}"`
}
```

构造函数参数同样可以由配置驱动。

```go
func init() {
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

集合注入也支持配置决定列表。下面的例子证明过滤器列表可以来自配置。

```go
type Chain struct {
	Filters []Filter `autowire:"${http.filters}"`
}
```

构造函数参数也可以用同样方式读取配置中的列表。

```go
func init() {
	gs.Provide(NewChain, gs.TagArg("${http.filters}"))
}
```

配置值最终仍然会回到名称选择或列表选择上。比如 `storage.provider=oss` 表示选择名为 `oss` 的 Bean，`http.filters=auth,tracing,recovery` 表示按这个顺序注入过滤器。配置驱动适合真实的环境差异，不适合替代普通的按类型或按名称注入。

## 注入目标

注入目标回答的是“这个依赖点拿哪个 Bean”。

单 Bean 注入优先按类型匹配，出现多个候选时用名称收窄范围，确实允许缺失时再加 `?`。集合注入用 `[]T` 或 `map[string]T` 承接多个 Bean：slice 可以表达顺序，Map 适合按名称查找。配置驱动则把名称或列表交给部署环境决定。

把这些选择写清楚以后，Go-Spring 才能在启动阶段给出明确结果：要么找到符合声明的 Bean，要么直接暴露缺失、重复或配置错误。对业务代码来说，最重要的是注入声明本身能说清意图，而不是把选择过程藏在代码顺序或运行时分支里。
