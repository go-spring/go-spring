# Go-Spring 实战第 11 课 —— 依赖注入的目标：单 Bean 注入和集合注入

上一课我们讲的是依赖写在哪里，也就是字段注入和构造函数注入。位置确定以后，Go-Spring 还要继续判断另一个问题：这个依赖点最终拿到什么。

换个角度看，这个问题跟依赖点自己的类型和需求直接相关。字段是 `UserRepository`，构造函数参数是 `UserService`，容器要找的是一个 Bean；字段是 `[]Filter` 或 `map[string]Handler`，容器要准备的就是一组 Bean。

沿着这个角度往下看，注入目标就分成两类：单 Bean 注入和集合注入。配置驱动也会出现，但它不是新的目标类型，只是把“选哪个 Bean”或“收集哪些 Bean”这件事交给配置来决定。

## 单 Bean 注入

当字段或构造函数参数声明的是一个普通 Bean 类型时，Go-Spring 会从候选 Bean 中选出一个注入进去。最直接的情况是不写名称，只按依赖点声明的类型匹配。

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

这里类型就是选择条件。注册方提供一个 `UserService`，依赖方声明一个 `UserService`，Go-Spring 只需要确认候选唯一，然后完成注入。

如果同类型 Bean 有多个，类型只能说明“要这一类对象”，不能说明“要哪一个对象”。这时名称就进入了注入声明。

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

名称一旦进入注入声明，就不再只是展示信息。它会成为注册方和依赖方之间的约定。像 `master`、`slave`、`primary`、`readonly` 这类名称，通常比 `db1`、`db2` 更能说明实例差异。

还有一种情况不是多个候选，而是依赖本身允许不存在。Go-Spring 默认在找不到匹配 Bean 时让启动失败；如果这个依赖只是增强能力，可以用 `?` 把缺失语义写出来。

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

这里表达的是“能力可选”。如果一个依赖是对象正常工作的前提，把它标成可选只会把启动阶段的错误推迟到运行时。

## 集合注入

如果字段或构造函数参数声明的是 `[]T` 或 `map[string]T`，Go-Spring 要做的就不是选出一个 Bean，而是收集一组 Bean。这就是集合注入。

先看 slice。`[]T` 表示依赖方需要一组同类型 Bean。

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

构造函数参数也可以声明为 `[]Plugin`，Go-Spring 会按同样规则收集候选。

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{Plugins: plugins}
}

func init() {
	gs.Provide(NewApplication)
}
```

未指定标签内容时，Go-Spring 会收集所有匹配 Bean，并按 Bean 名称的字典序放入 slice。这个顺序是稳定的，但它不一定就是业务顺序。

如果执行顺序本身有含义，可以在注入声明中显式列出名称。

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

这里 `auth` 固定在最前，`recovery` 固定在最后，`*` 补齐其他同类型 Bean。`*` 匹配到的部分仍然按 Bean 名称排序。

如果依赖方关心的是按名称查找，而不是执行顺序，可以把目标声明成 `map[string]T`。在 Map 收集里，Bean 名称会成为 Map 的 key。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

需要限制 Map 中的条目时，也可以在标签里列出名称。

```go
type Router struct {
	Handlers map[string]Handler `autowire:"auth,user?,order"`
}
```

这里 `user?` 表示 `user` 这个 Handler 不存在时跳过。Map 注入也能使用 `*`，只是 Map 本身没有顺序，`*` 在这里表达的是“补齐剩余条目”，不是控制位置。

## 配置驱动

前面的例子都把目标写在代码里。有些选择会跟部署环境或站点配置一起变化，这时可以把注入标签写成 `${...}`，从配置中读取 Bean 名称或名称列表。

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

集合注入也可以用配置决定列表。

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

配置值最终仍然会回到名称选择或列表选择上。比如 `storage.provider=oss` 表示选择名为 `oss` 的 Bean，`http.filters=auth,tracing,recovery` 表示按这个顺序注入过滤器。也就是说，配置驱动改变的是选择来源，不改变注入目标本身。

## 注入目标

注入目标这件事，最后还是回到依赖点自己的类型和需求：它要拿一个 Bean，还是拿一组 Bean。

普通 Bean 类型对应单 Bean 注入，`[]T` 和 `map[string]T` 对应集合注入。名称、`?`、`*` 和 `${...}` 都是在这个基础上继续补充选择语义。

这样看，Go-Spring 的注入目标并不复杂。依赖点先用类型说明目标形态，再用名称、可选标记或配置表达更具体的选择。容器在启动阶段把这些声明解析成确定结果，业务代码拿到的仍然是普通 Go 对象。
