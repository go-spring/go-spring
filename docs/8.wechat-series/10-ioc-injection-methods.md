# Go-Spring 实战第 10 课 —— 依赖注入方式：对象创建边界决定依赖写在哪里

配置体系解决了值怎样进入应用的问题。进入 IoC 容器以后，另一个问题会马上出现：业务对象之间的依赖应该写在字段上，还是写在构造函数参数里。

这个选择看起来像代码风格，实际决定的是对象什么时候进入可用状态。如果对象可以先创建，再由容器补齐外部依赖，字段注入会很轻；如果对象一创建出来就必须拿到完整依赖，构造函数注入会把边界表达得更清楚。

Go-Spring 同时支持字段注入和构造函数注入。它不是要求所有对象统一成一种写法，而是让不同对象把自己的创建边界说清楚。

## 字段注入

先看一个证明字段注入语义的最小例子。`UserController` 自己没有构造函数，依赖通过字段上的 `autowire` 标签声明。

```go
type UserController struct {
	Service UserService `autowire:""`
}

func init() {
	gs.Provide(new(UserController))
}
```

Go-Spring 创建 `UserController` 实例后，会根据字段类型和标签内容查找匹配的 Bean，再把结果写入 `Service` 字段。`autowire:""` 表示按类型匹配；如果类型能唯一确定目标 Bean，标签里不需要再写名称。

这个语义带来一个很明确的边界：字段值是在对象创建之后才进入的。因此，构造阶段不能依赖这些字段，涉及路由挂载、连接建立、参数校验这类动作，也应该放到依赖注入之后的生命周期阶段。

字段注入适合控制器、配置承载对象和少量简单组件。它能减少样板代码，但它表达的是“先有对象，再补依赖”。

## 构造函数注入

如果依赖是对象成立的前提，就应该让依赖出现在构造函数签名里。下面的例子证明的是：Go-Spring 会在调用构造函数前先解析参数依赖。

```go
type UserController struct {
	service UserService
}

func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController)
}
```

这里 `UserController` 只有在 `UserService` 解析成功后才会被创建。对象一旦从 `NewUserController` 返回，就已经处在可用状态，字段也可以保持未导出。

这种方式更接近普通 Go 代码的依赖表达。单元测试可以直接调用 `NewUserController(mockService)`，不需要为了填字段启动容器。创建过程如果可能失败，还可以让构造函数返回 `(T, error)`，把失败保留在启动阶段。

## Arg

构造函数有一个 Go 语言层面的限制：函数参数不能写 tag。因此，当参数需要按名称选择 Bean、从配置读取值，或者使用注册期固定值时，Go-Spring 会在注册阶段用 `Arg` 补充这些语义。

下面的例子证明 `TagArg` 可以为构造函数参数补上字段标签里原本能表达的名称选择。

```go
func NewRepository(ds *DataSource) *Repository {
	return &Repository{ds: ds}
}

func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")

	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

`NewRepository` 仍然是普通 Go 函数。真正的容器语义集中在注册语句里，`gs.TagArg("slave")` 表示这个参数注入名为 `slave` 的 `*DataSource` Bean。

如果参数能按类型唯一匹配，`TagArg` 可以省略。只有同类型多实例、配置绑定或固定值进入参数时，才需要显式补充参数绑定信息。这样既不破坏构造函数签名，也能让容器得到足够明确的注入规则。

## 创建边界

选择注入方式时，判断点不是“哪种写法更高级”，而是依赖在什么时候必须可用。

依赖复杂、创建时就要完整可用的对象，优先使用构造函数注入。依赖关系会集中出现在函数签名里，对象创建完成后状态也更稳定。

对象只是承载少量依赖，或者配置字段和依赖字段自然放在同一个结构体上时，字段注入可以保持代码轻量。少量循环依赖场景也可能需要先创建实例，再由容器补齐字段，但这通常应该被当成边界折中，而不是默认风格。

依赖写在哪里确定以后，Go-Spring 还要继续回答更具体的问题：这个注入点到底应该拿到哪个 Bean，是单个候选、命名候选、可选依赖，还是一组按规则排列的 Bean。
