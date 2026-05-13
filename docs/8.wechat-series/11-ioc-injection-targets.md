# Bean 注入目标

声明了依赖，不等于容器已经知道该给什么。

无论使用字段注入还是构造函数注入，容器最终都要回答一个更具体的问题：应该把哪个值放到这个注入点上？

围绕这个问题，Go-Spring 支持多种注入目标：

- 单个 Bean。
- 多个 Bean。
- 由配置项指定的 Bean。
- 延迟注入的 Bean。

## 注入单个 Bean

最常见的是按类型注入唯一 Bean：

```go
type Service struct {
	Repo UserRepository `autowire:""`
}
```

构造函数也可以省略 `TagArg`，让容器按类型推断：

```go
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

当同类型 Bean 有多个时，需要按名称区分。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Service struct {
	ds DataSource `autowire:"slave"`
}
```

构造函数中使用同样语义：

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

## 可空注入

默认情况下，找不到匹配 Bean 会启动失败。如果依赖是可选的，可以使用 `?` 标记：

```go
type Service struct {
	OptionalDep Dep `autowire:"?"`
	NamedDep    Dep `autowire:"my-name?"`
}
```

构造函数参数也可以这样写：

```go
func init() {
	gs.Provide(NewUserController, gs.TagArg("?"))
	gs.Provide(NewUserController, gs.TagArg("my-name?"))
}
```

可空注入适合插件式依赖、可选增强能力或兼容旧系统的过渡场景。核心依赖不建议设为可空，否则错误会被推迟到运行期。

## 注入多个 Bean

当依赖是 `[]T` 时，容器会收集所有匹配 Bean：

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

未指定 tag 时，切片按 Bean 名称字典序排序，保证结果确定。

如果需要控制顺序，可以显式列出名称：

```go
type Chain struct {
	Filters []Filter `autowire:"auth?,tracing,recovery"`
}
```

列表中可以使用 `name?` 表示不存在时跳过，也可以使用一次 `*` 表示收集剩余未显式列出的 Bean：

```go
type Chain struct {
	Filters []Filter `autowire:"auth,*,recovery"`
}
```

`map[string]T` 也可以收集多个 Bean，key 是 Bean 名称：

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

Map 本身无序，因此更适合按名称查找，不适合表达执行顺序。

## 通过配置项决定注入目标

注入标签可以写成 `${...}`，由配置项决定注入哪个 Bean 或哪些 Bean：

```go
type Service struct {
	Storage Storage `autowire:"${storage.provider}"`
}
```

构造函数参数同理：

```go
func init() {
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

集合注入也支持配置驱动：

```go
type Chain struct {
	Filters []Filter `autowire:"${http.filters}"`
}
```

这种方式适合通过配置切换实现，例如存储后端、过滤器链、插件列表。

## 延迟注入

延迟注入用于某些循环依赖场景，仅适用于结构体字段注入：

```go
type Service struct {
	Dep Dependency `autowire:",lazy"`
}
```

标记为 `lazy` 的字段会在所有非延迟注入完成后统一处理。在此之前字段保持零值，因此初始化逻辑中不能提前使用它。

## 动态能力要克制

注入目标越动态，排查成本越高。核心链路优先使用按类型或按名称的显式注入；配置驱动注入适合真正需要运行环境切换的点；可空和延迟注入应保持克制。

接下来进入 Bean 类型与构造函数参数绑定，看看 Go-Spring 如何处理结构体指针、构造函数、函数指针以及 `TagArg`、`ValueArg`、`BindArg`、`IndexArg`。
