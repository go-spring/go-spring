# Go-Spring 实战第 11 课 —— 依赖注入的目标：单 Bean 注入和集合注入

上一篇我们讲的是依赖写在哪里的问题，也就是字段注入和构造函数注入。本篇我们讲依赖注入目标的问题。

大部分情况下，注入目标是唯一的。比如字段是 `UserRepository`，构造函数参数是 `UserService`，这种情况我们称为单 Bean 注入。少数情况下，注入目标是一组 Bean。比如字段是 `[]Filter` 或 `map[string]Handler`，这种情况我们称为集合注入。

## 单 Bean 注入

当字段或构造函数参数声明的是一个普通 Bean 类型时，Go-Spring 会从候选 Bean 中选出唯一一个注入到依赖点，这就是单 Bean 注入。最简单的情况是不写名称，只按依赖点声明的类型进行匹配。

```go
type Service struct {
	Repo UserRepository `autowire:""`
}
```

构造函数参数同样适用该规则。只要容器能唯一匹配 `UserService`，那注册时就可以不额外声明参数标签。

```go
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

类型本身就是选择条件。注册方提供一个 `UserService`，依赖方声明一个 `UserService`，Go-Spring 就能根据类型找到唯一的候选 Bean。

但有时候，同类型的 Bean 会有多个。这时类型只能说明“要这一类对象”，不能说明“要哪一个对象”。因此我们需要补充选择条件，也就是指定 Bean 名称。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Repository struct {
	DS DataSource `autowire:"slave"`
}
```

在上面的代码中，`autowire:"slave"` 表示依赖方不仅要求类型匹配，还要求 Bean 名称必须是 `slave`。

我们也可以在构造函数参数中通过 `TagArg` 表达同样的选择，比如 `gs.TagArg("slave")` 表示注入名称为 `slave` 的 Bean。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

通常情况下，我们希望注入的 Bean 必须存在，如果不存在就要报错。但如果我们确实需要允许某个依赖缺失，那么可以用 `?` 表示可选依赖。

我们可以对类型依赖和命名依赖都使用 `?` 表示可选。示例如下：

```go
type Service struct {
	OptionalDep Dep `autowire:"?"`
	NamedDep    Dep `autowire:"my-name?"`
}
```

构造函数参数也可以使用相同的语义。

```go
func NewService(dep Dep, named Dep) *Service {
	return &Service{OptionalDep: dep, NamedDep: named}
}

func init() {
	gs.Provide(NewService, gs.TagArg("?"), gs.TagArg("my-name?"))
}
```

虽然 Go-Spring 提供了可选依赖机制，但是我们建议只在确有必要时使用。否则，本应在启动阶段暴露的问题可能会被推迟到运行时，增加排查成本。

## 集合注入

如果字段或者构造函数参数声明的是 `[]T` 或 `map[string]T`，那 Go-Spring 要做的就不是选出一个 Bean，而是收集一组 Bean。这就是集合注入。

先来看 slice。`[]T` 表示依赖方需要一组相同类型的 Bean。我们可以完全通过类型收集 Bean。

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

也可以通过构造函数参数收集 Bean。

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{Plugins: plugins}
}

func init() {
	gs.Provide(NewApplication)
}
```

当我们没有指定收集顺序时，Go-Spring 会按照收集到的 Bean 的默认名称排序。但是这个顺序未必符合业务需要，此时我们可以在注入声明中显式列出要注入的 Bean 名称。示例如下：

```go
type Chain struct {
	Filters []Filter `autowire:"auth?,tracing,recovery"`
}
```

对于上述代码，Go-Spring 会按照 `auth`、`tracing`、`recovery` 的顺序收集 Bean。仔细观察代码，我们发现 `auth` 后面还带了一个 `?`，这表示它是可选的。也就是说，`auth` 对应的 Bean 可以不存在。

有时候我们只想固定少数特殊 Bean 的位置，其他 Bean 的顺序没有关系。此时我们可以使用下面的写法：

```go
type Chain struct {
	Filters []Filter `autowire:"auth,*,recovery"`
}
```

这里 `auth` 固定在最前，`recovery` 固定在最后，`*` 用来补齐其他同类型的 Bean。也就是说，我们可以用 `*` 表示“补齐剩余同类型 Bean”。

对于 slice，`*` 匹配到的部分仍然是按照 Bean 名称排序的。

如果依赖方更关心按名称查找，而不是注入顺序，那我们可以把目标声明成 `map[string]T`。在 map 收集里，Bean 名称就是 map 的 key，依赖方可以通过 key 获取对应的 Bean。

map 注入使用和 slice 注入相同的语法，只是 map 注入不强调顺序。看一些例子。

我们可以完全依靠类型收集 Bean。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

也可以混合使用类型和名称，以及 `?` 和 `*`。

```go
type Router struct {
	Handlers map[string]Handler `autowire:"auth,user?,*"`
}
```

## 配置驱动

在前面的例子里，我们都是把注入目标写在代码里的。但有时候，具体选择会随部署环境或站点配置变化。这时候我们可以把注入标签写成 `${...}`，表示从配置中读取 Bean 名称或名称列表。看一些例子。

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

构造函数参数也可以用同样的方式读取配置中的列表。

```go
func init() {
	gs.Provide(NewChain, gs.TagArg("${http.filters}"))
}
```

配置驱动改变的是选择来源，而不是注入目标本身。如果我们在配置中设置了 `storage.provider=oss`，那么表示选择名为 `oss` 的 Bean；如果我们在配置中设置了 `http.filters=auth,tracing,recovery`，那么表示按这个顺序注入过滤器。理解了这一点，我们就能在类型、名称和配置之间做出清晰选择。
