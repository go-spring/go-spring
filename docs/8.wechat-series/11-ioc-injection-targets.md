# Go-Spring 实战第 11 课 —— 注入目标选择：多候选、可选依赖和配置切换如何落到注入点

上一课解决的是依赖写在哪里。依赖点一旦出现，Go-Spring 容器还要判断：这个位置应该拿到哪个值。

在简单服务里，一个类型通常只有一个实现，按类型注入就够了。服务演进以后，同一个接口可能有多个实现，某些能力可能是可选插件，过滤器链还可能来自配置。此时，目标选择规则就会直接影响对象图是否稳定。

Go-Spring 的选择规则可以从静态到动态理解。能按类型唯一注入时，就让类型承担协议；出现多实例时，再引入名称；确实需要按环境切换时，才把选择权交给配置或延迟流程。

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

这里类型就是依赖协议。注册方提供一个 `UserService`，依赖方声明一个 `UserService`，容器只负责把唯一候选连起来。这个方式最稳定，也最适合核心业务链路。

## 名称匹配

当同类型 Bean 出现多个时，类型已经不足以表达选择意图。下面的例子证明名称会成为依赖协议的一部分。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Repository struct {
	ds DataSource `autowire:"slave"`
}
```

`autowire:"slave"` 表示依赖方不仅要求类型匹配，还要求 Bean 名称是 `slave`。构造函数参数中也通过 `TagArg` 表达同样选择。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

名称一旦进入注入声明，就不再是展示信息。它会成为注册方、依赖方和配置之间的约定，因此名称要稳定、可读，并且能表达实例差异。

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
func init() {
	gs.Provide(NewUserController, gs.TagArg("?"))
	gs.Provide(NewUserController, gs.TagArg("my-name?"))
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

这种写法适合“收集所有组件”的场景。如果执行顺序本身有业务含义，就不要把顺序藏在注册过程里，而应该在注入声明中显式列出名称。

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

这里的语义是：显式名称负责表达关键位置，`*` 负责补齐其他同类型 Bean。只要顺序会影响行为，就应该让顺序出现在标签或配置里，而不是让读者从代码注册位置里推断。

`map[string]T` 也能收集多个 Bean。下面的例子证明 Bean 名称会成为 Map 的 key。

```go
type Router struct {
	Handlers map[string]Handler `autowire:""`
}
```

Map 更适合按名称查找或导出注册表，不适合表达执行顺序。只要顺序有语义，优先用 slice。

## 配置驱动

有些选择不能固定在代码里。例如不同部署环境要切换存储后端，或者不同站点要启用不同过滤器链。这时，注入标签可以写成 `${...}`，把选择交给配置。

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

集合注入也支持配置决定列表。下面的例子证明列表选择可以从配置树进入注入标签。

```go
type Chain struct {
	Filters []Filter `autowire:"${http.filters}"`
}
```

这里的语义是，配置值最终仍然会回到名称选择或列表选择上。配置驱动注入适合真实的环境切换点，例如存储后端、过滤器链和插件列表。它的代价是依赖关系的一部分进入配置文件，因此不适合替代普通的按类型或按名称注入。

## 延迟注入

少量创建环可能需要先创建对象，再补齐其中某些依赖。下面的例子证明字段注入可以通过 `lazy` 延迟处理。

```go
type Service struct {
	Dep Dependency `autowire:",lazy"`
}
```

标记为 `lazy` 的字段会在所有非延迟注入完成后统一处理。在此之前字段保持零值，所以初始化逻辑不能提前使用它。

延迟注入适合兜住少量历史代码里的创建环。复杂循环依赖通常说明模块边界已经互相缠住，继续扩大 `lazy` 的使用范围，只会把启动期依赖问题推迟成运行期空值问题。

## 注入目标

注入目标回答的是“这个依赖点拿哪个值”。类型和名称最适合核心链路，可选依赖适合增强能力，集合注入适合插件和链路组合，配置驱动只应该放在真实环境切换处，延迟注入则用来处理少量创建环。

这些规则共同保证了注入点的含义是可推断的：能由类型唯一表达时不引入额外协议，需要多实例时显式命名，需要环境差异时才把选择交给配置。Go-Spring 的注入目标选择，最终服务的是一张稳定、可解释的对象图。
