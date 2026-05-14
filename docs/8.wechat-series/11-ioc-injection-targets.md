# Go-Spring 实战第 11 课：注入目标选择：单 Bean、多 Bean、配置驱动和延迟注入

上一篇我们看的是“依赖怎样声明”。但声明了依赖以后，不等于 Go-Spring 容器已经知道该给什么。

无论使用字段注入还是构造函数注入，Go-Spring 容器最终都要回答一个更具体的问题，即哪个值会进入这个注入点？

围绕这个问题，Go-Spring 支持多种注入目标。

- 单个 Bean。
- 多个 Bean。
- 由配置项指定的 Bean。
- 延迟注入的 Bean。

可以把它们理解成从静态到动态的一组选择。越动态，表达力越强，依赖关系也越容易分散到配置或延迟流程里。后面看每一种目标时，也顺带看它适合放在哪些位置。

实际使用时可以先抓一条顺序，即按类型能唯一注入时直接按类型；出现多实例时引入名称；确实要通过环境切换实现时，再把选择权交给配置。

## 唯一依赖先靠类型和名称确定

最常见的是按类型注入唯一 Bean。

```go
type Service struct {
	Repo UserRepository `autowire:""`
}
```

构造函数也可以省略 `TagArg`，让容器按类型推断。

```go
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

如果同类型 Bean 有多个，就需要按名称区分。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Service struct {
	ds DataSource `autowire:"slave"`
}
```

构造函数中使用同样语义。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

这时候名称就是依赖协议的一部分。因为调用方已经通过名称表达了选择意图，所以名称稳定、可读，后面排查依赖关系也会更直接。

## 可选依赖必须显式标出来

默认情况下，找不到匹配 Bean 会启动失败。如果依赖是可选的，可以使用 `?` 标记。

```go
type Service struct {
	OptionalDep Dep `autowire:"?"`
	NamedDep    Dep `autowire:"my-name?"`
}
```

构造函数参数也可以这样写。

```go
func init() {
	gs.Provide(NewUserController, gs.TagArg("?"))
	gs.Provide(NewUserController, gs.TagArg("my-name?"))
}
```

可空注入适合插件式依赖、可选增强能力或兼容旧系统的过渡场景。不过，核心依赖如果也设为可空，缺失依赖的错误会被推迟到运行期。这里的重点是表达“可选能力”。

## 集合注入先想清楚顺序

接着看集合注入。这里的关键是，依赖是 `[]T` 时，Go-Spring 容器会收集所有匹配 Bean。

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

如果需要控制顺序，可以显式列出名称。

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

`map[string]T` 也可以收集多个 Bean，key 是 Bean 名称。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

Map 本身无序，所以更适合按名称查找，不适合表达执行顺序。

## 配置驱动适合真正的环境切换

注入标签可以写成 `${...}`，由配置项决定注入哪个 Bean 或哪些 Bean。

```go
type Service struct {
	Storage Storage `autowire:"${storage.provider}"`
}
```

构造函数参数同理。

```go
func init() {
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

集合注入也支持配置驱动。

```go
type Chain struct {
	Filters []Filter `autowire:"${http.filters}"`
}
```

这种方式适合通过配置切换实现，例如存储后端、过滤器链、插件列表。不过它会把依赖关系的一部分移到配置文件里，因此更适合确实需要运行环境切换的点。

## 延迟注入只兜少量循环依赖

延迟注入主要用来处理少量循环依赖，目前放在结构体字段注入上。

```go
type Service struct {
	Dep Dependency `autowire:",lazy"`
}
```

标记为 `lazy` 的字段会在所有非延迟注入完成后统一处理。在此之前字段保持零值，初始化逻辑如果提前使用它，就会把循环依赖问题变成运行期空值问题。

## 动态注入能力要放在合适的位置

注入目标越动态，依赖关系越容易分散。核心链路通常用按类型或按名称的显式注入；配置驱动注入放在需要运行环境切换的点；可空和延迟注入更多用于可选能力和少量循环依赖。

注入目标回答的是“依赖给谁”。再往前一步，还要看 Bean 自己从哪里来，即结构体指针、构造函数和函数指针会对应不同的创建策略。
