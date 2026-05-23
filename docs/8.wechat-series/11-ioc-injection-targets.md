# Go-Spring 实战第 11 课 —— 依赖注入的目标：单 Bean 注入和集合注入

上一课我们讲的是依赖应该如何注入，也就是字段注入和构造函数注入。位置确定以后，Go-Spring 还要继续判断另一个问题：这个依赖点最终应该拿到什么。

这个问题不是由项目大小决定的，而是由依赖点自己的需求决定的。有些依赖点只需要一个对象，例如 `UserRepository`；有些依赖点需要一组对象，例如 `[]Filter` 或 `map[string]Handler`。目标类型不同，Go-Spring 的选择规则也不同。

所以，从结果看，Go-Spring 的注入目标只有两类：一种是单 Bean 注入，即一个依赖点拿到一个 Bean；另一种是集合注入，即一个依赖点拿到一组 Bean。不会再有第三种目标形态。无论哪一种，目标都可以直接写在代码里；如果确实需要随环境变化，也可以交给配置。

## 单 Bean 注入

先看单 Bean 注入。当字段或构造函数参数声明的是一个普通 Bean 类型时，Go-Spring 要做的是从候选 Bean 中选出一个。它的基本规则是：能按类型唯一匹配时，就不要增加额外约束；同类型出现多个候选时，再用名称收窄范围；如果这个依赖本来就允许不存在，就把可选语义显式写出来。

### 类型匹配

先看最基础的情况：没有指定名称时，Go-Spring 会按依赖点声明的类型查找唯一候选。

```go
type Service struct {
	Repo UserRepository `autowire:""`
}
```

构造函数参数也使用同样规则。只要容器里能唯一匹配 `UserService`，注册时就不需要额外声明参数标签。

```go
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

这里类型就是选择条件。注册方提供一个 `UserService`，依赖方声明一个 `UserService`，Go-Spring 只需要确认候选唯一，然后完成注入。所以当依赖点只需要一个对象，并且类型已经能唯一表达目标时，就不需要再引入名称。

### 名称匹配

但只靠类型并不总是够用。当同类型 Bean 出现多个时，Go-Spring 需要额外的名称来缩小范围。看下面这个例子，名称已经成为注入声明的一部分。

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

名称一旦进入注入声明，就不再只是展示信息。它会成为注册方和依赖方之间的约定，因此名称要稳定、可读，并且能表达实例差异。像 `master`、`slave`、`primary`、`readonly` 这类名称，通常比 `db1`、`db2` 更容易长期维护。

### 可选依赖

名称解决的是“多个候选选哪一个”，可选依赖解决的是另一类问题：这个依赖本来就允许不存在。默认情况下，找不到匹配 Bean 会让应用启动失败。这个默认值很重要，因为核心依赖缺失应该停在启动阶段。

如果某个依赖确实只是增强能力，缺失语义就要写进声明里。`?` 的作用就是把“没有候选”从启动错误变成可接受结果。

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

可选依赖适合插件式能力、可选增强或兼容旧系统的过渡场景。反过来，如果核心依赖也标成可选，错误就会从启动阶段推迟到业务运行时。所以这里表达的是“能力可选”，不是“找不到也无所谓”。

## 集合注入

如果字段或构造函数参数声明的是 `[]T` 或 `map[string]T`，Go-Spring 要做的就不是选出一个 Bean，而是收集一组 Bean。这就是集合注入。

### Slice 收集

先看 slice。`[]T` 可以收集所有匹配 Bean。

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

如果使用构造函数注入，参数也可以声明为 `[]Plugin`，Go-Spring 会按同样规则收集候选。

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{Plugins: plugins}
}

func init() {
	gs.Provide(NewApplication)
}
```

未指定标签内容时，Go-Spring 会收集所有匹配 Bean，并按 Bean 名称的字典序放入 slice。这样做的重点不是表达业务顺序，而是让每次启动得到一致结果。

不过，一旦执行顺序本身有业务含义，就不要依赖默认收集规则，而应该在注入声明中显式列出名称。

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

这里的语义是：`auth` 固定在最前，`recovery` 固定在最后，`*` 补齐其他同类型 Bean。`*` 匹配到的部分仍然按 Bean 名称排序。换句话说，只要顺序会影响行为，就应该让顺序出现在标签或配置里，而不是让读者从注册位置里推断。

### Map 收集

如果依赖方关心的是按名称查找，而不是执行顺序，就更适合使用 Map。在 Map 收集里，Bean 名称会成为 Map 的 key。

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

这里 `user?` 表示 `user` 这个 Handler 不存在时跳过。虽然 Map 注入也能使用 `*`，但 Map 本身没有顺序，所以只有在“收集剩余 Bean”这个语义确实有用时才值得使用。

## 配置驱动

前面的例子都把目标写在代码里，但有些选择确实不应该固定在代码中。例如不同部署环境要切换存储后端，或者不同站点要启用不同过滤器链。这时，Go-Spring 允许把注入标签写成 `${...}`，从配置中读取 Bean 名称或名称列表。

单 Bean 注入可以用配置决定具体名称。

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

集合注入也可以用配置决定列表。过滤器链就是一个典型例子。

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

配置值最终仍然会回到名称选择或列表选择上。比如 `storage.provider=oss` 表示选择名为 `oss` 的 Bean，`http.filters=auth,tracing,recovery` 表示按这个顺序注入过滤器。因此，配置驱动适合真实的环境差异；如果选择规则是代码结构的一部分，直接写在注入声明里更清楚。

## 注入目标

注入目标这件事，最后还是回到依赖点自己的类型和需求：它要拿一个 Bean，还是拿一组 Bean。

单 Bean 注入优先按类型匹配，出现多个候选时用名称收窄范围，确实允许缺失时再加 `?`。集合注入用 `[]T` 或 `map[string]T` 承接多个 Bean：slice 可以表达顺序，Map 适合按名称查找。配置驱动不是第三类注入目标，而是把单 Bean 名称或集合列表交给部署环境决定。

把这些选择写清楚以后，Go-Spring 才能在启动阶段给出明确结果：要么找到符合声明的 Bean，要么直接暴露缺失、重复或配置错误。对业务代码来说，最重要的是让注入声明本身说清意图，而不是把选择过程藏在代码顺序或运行时分支里。
