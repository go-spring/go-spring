# Go-Spring 实战第 11 课 —— 依赖注入的目标：单 Bean 注入和集合注入

上一篇我们讲的是依赖写在哪里的问题，也就是字段注入和构造函数注入。本篇我们讲依赖注入目标的问题。

大部分情况下，注入目标是唯一的，比如字段是 `UserRepository`，构造函数参数是 `UserService`，这种情况我们称为单 Bean 注入。只有一些很少的情况，注入目标是多个 Bean，比如字段是 `[]Filter` 或 `map[string]Handler`，这种情况我们称为集合注入。

## 单 Bean 注入

当字段或构造函数参数声明的是一个普通 Bean 类型时，Go-Spring 会从候选 Bean 中选出唯一一个注入进去，我们称为单 Bean 注入。最简单的情况是不写名称，只按依赖点声明的类型进行匹配。

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

类型本身就是选择条件。注册方提供一个 `UserService`，依赖方声明一个 `UserService`，Go-Spring 也只能找到这一个 `UserService`。

但有时候，同类型的 Bean 会有多个，这时候类型就只能说明“要这一类对象”，不能说明“要哪一个对象”。因此我们需要加强选择条件，也就是指定 Bean 名称。

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

我们也可以在构造函数参数中通过 `TagArg` 表达同样的选择，比如 `gs.TagArg("slave")` 表示注入名字为 `slave` 的 Bean。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

通常情况下，我们希望注入的 Bean 一定是存在的，如果不存在就报错。但如果我们允许注入的 Bean 不存在，那么可以用 `?` 来表示可选依赖。

我们可以对类型依赖和命名依赖都使用 `?` 表示可选。示例如下：

```go
type Service struct {
	OptionalDep Dep `autowire:"?"`
	NamedDep    Dep `autowire:"my-name?"`
}
```

当然，构造函数参数也可以使用相同的表达语义。

```go
func NewService(dep Dep, named Dep) *Service {
	return &Service{OptionalDep: dep, NamedDep: named}
}

func init() {
	gs.Provide(NewService, gs.TagArg("?"), gs.TagArg("my-name?"))
}
```

虽然 Go-Spring 提供了可选依赖的机制，但是我们推荐只有在十分必要时才使用，否则可能会把启动时的错误推迟到运行时，增加排查成本。

## 集合注入

如果字段或者构造函数参数声明的是 `[]T` 或 `map[string]T`，那 Go-Spring 要做的就不是选出一个 Bean，而是收集一组 Bean。这就是集合注入。

先来看 slice。`[]T` 表示依赖方需要一组相同类型的 Bean。如果我们可以仅依靠类型来确定注入的 Bean，那代码可以这样写。

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

或者我们使用构造函数参数。

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{Plugins: plugins}
}

func init() {
	gs.Provide(NewApplication)
}
```

当我们使用 slice 时，我们实际上是期望某种顺序的。当我们未指定 Bean 的收集顺序时，Go-Spring 会按照收集到的 Bean 的默认名称进行排序。但这种排序未必是用户想要的，此时我们可以在注入声明中显式列出被注入的 Bean 名称。示例如下：

```go
type Chain struct {
	Filters []Filter `autowire:"auth?,tracing,recovery"`
}
```

对于上述代码，Go-Spring 会按照 `auth`、`tracing`、`recovery` 的顺序收集 Bean。仔细观察上述代码，我们发现 `auth` 后面带了一个 `?`，根据前面的叙述，我们知道 `?` 表示可选，所以在上面的收集结果中，`auth` 是可以不存在的。

有时候我们仅仅希望固定一些特殊 Bean 的位置，其他 Bean 的顺序没有关系。此时我们可以使用下面的写法：

```go
type Chain struct {
	Filters []Filter `autowire:"auth,*,recovery"`
}
```

这里 `auth` 固定在最前，`recovery` 固定在最后，`*` 用来补齐其他同类型的 Bean。也就是说，我们可以使用 `*` 来表示“补齐剩余同类型 Bean”这个含义。

对于 slice，`*` 匹配到的部分仍然是按照 Bean 名称排序的。

如果依赖方更关心的是按照名称进行查找，而不是关心注入的顺序，则我们可以把目标声明成 `map[string]T`。在 Map 收集里，Bean 名称就是 Map 的 key。我们可以通过 key 来查找获取对应的 Bean。

map 注入使用和 slice 注入相同的语法，只是 Map 注入没有顺序。可以看一些例子。

我们可以完全使用类型确定注入的 Bean。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

也可以使用类型和名称混合注入，同时也可以使用 `?` 和 `*` 。

```go
type Router struct {
	Handlers map[string]Handler `autowire:"auth,user?,*"`
}
```

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
