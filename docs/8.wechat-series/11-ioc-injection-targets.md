# Go-Spring 实战第 11 课：注入目标选择：多候选、可选依赖和配置切换怎么落到注入点

字段注入和构造函数注入解决的是依赖写在哪里。真正启动时，Go-Spring 容器还要继续判断哪个值能进入这个注入点。

在简单服务里，一个类型通常只有一个实现，按类型注入就够了。服务演进以后，同一个接口可能有多个实现，某些能力可能是可选插件，过滤器链还可能来自配置。此时，注入目标的选择规则就会直接影响对象图是否稳定。

Go-Spring 的注入目标可以从静态到动态逐步理解。能按类型唯一注入时，就让类型承担协议；出现多实例时，再引入名称；确实需要按环境切换实现时，才把选择权交给配置或延迟流程。

## 单实例依赖优先让类型和名称承担协议

最稳定的注入方式是按类型找到唯一 Bean。下面这个字段没有指定名称，Go-Spring 容器会根据 `UserRepository` 类型查找唯一候选。

```go
type Service struct {
	Repo UserRepository `autowire:""`
}
```

构造函数参数也可以使用同样语义。只要 `UserService` 能唯一匹配，注册时就不需要额外 `TagArg`。

```go
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

同类型 Bean 出现多个以后，类型已经不足以表达选择意图，这时要把名称纳入依赖协议。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Service struct {
	ds DataSource `autowire:"slave"`
}
```

构造函数中也通过 `TagArg` 表达同样的选择。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

名称一旦进入注入声明，就不再是装饰信息。它会成为注册方和依赖方之间的协议，因此名称要稳定、可读，并且能表达实例差异。

## 可选依赖必须把缺失语义写进声明

默认情况下，找不到匹配 Bean 会让应用启动失败。这是 Go-Spring 的启动期模型希望保留的行为，因为核心依赖缺失应该尽早暴露。

如果某个依赖确实是可选增强能力，注入声明必须显式写出 `?`。

```go
type Service struct {
	OptionalDep Dep `autowire:"?"`
	NamedDep    Dep `autowire:"my-name?"`
}
```

构造函数参数也可以使用相同语义。

```go
func init() {
	gs.Provide(NewUserController, gs.TagArg("?"))
	gs.Provide(NewUserController, gs.TagArg("my-name?"))
}
```

可空注入适合插件式能力、可选增强或兼容旧系统的过渡场景。反过来，如果核心依赖也标成可空，错误就会从启动阶段推迟到业务运行时，所以这里表达的是“能力可选”，不是“依赖找不到也无所谓”。

## 集合注入要先确定顺序是否有业务含义

当依赖类型是 `[]T` 时，Go-Spring 容器会收集所有匹配 Bean。下面的例子把多个插件按 `Plugin` 接口导出，再统一注入到应用对象里。

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

未指定 tag 时，切片按 Bean 名称字典序排序，保证结果确定。这个顺序适合“收集所有组件”的场景，但不一定适合表达业务执行链。

如果执行顺序有语义，就应该在注入声明里显式列出名称。

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

`map[string]T` 也能收集多个 Bean，key 是 Bean 名称。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

Map 本身无序，所以它更适合按名称查找或导出注册表，不适合表达执行顺序。只要顺序会影响行为，就应该优先用切片并显式声明排序规则。

## 配置驱动只放在真实环境切换处

有些选择不能固定在代码里。例如不同部署环境要切换存储后端，或者不同站点要启用不同过滤器链。这时，注入标签可以写成 `${...}`，由配置项决定目标 Bean。

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

集合注入也支持配置决定列表。

```go
type Chain struct {
	Filters []Filter `autowire:"${http.filters}"`
}
```

配置驱动注入适合真正的环境切换点，例如存储后端、过滤器链和插件列表。它的代价是依赖关系的一部分会进入配置文件，因此不适合替代普通的按类型或按名称注入。

## 延迟注入只能兜住少量创建环

延迟注入主要用于处理少量循环依赖，目前放在结构体字段注入上。

```go
type Service struct {
	Dep Dependency `autowire:",lazy"`
}
```

标记为 `lazy` 的字段会在所有非延迟注入完成后统一处理。在此之前字段保持零值，所以初始化逻辑不能提前使用它。

这类能力适合作为过渡手段。复杂循环依赖通常说明模块边界已经互相缠住，继续扩大 `lazy` 的使用范围，只会把启动期依赖问题推迟成运行期空值问题。

## 注入目标越动态，越要收紧使用位置

注入目标回答的是“这个依赖点拿哪个值”。类型和名称最适合核心链路，可空依赖适合可选能力，集合注入适合插件和链路组合，配置驱动只应该放在真实环境切换处，延迟注入则用来兜少量创建环。

注入目标确定以后，还要继续往前看 Bean 自己从哪里来。结构体指针、构造函数和函数值进入容器时，会对应不同的创建策略。
