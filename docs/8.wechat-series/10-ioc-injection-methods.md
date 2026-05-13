# Bean 注入方式

## 本篇要解决的问题

Go-Spring 的依赖注入可以从两个维度理解：

- 注入方式：依赖通过什么语法形式声明。
- 注入目标：最终注入哪个 Bean、哪些 Bean 或哪个配置值。

本篇只讨论注入方式。Go-Spring 主要支持结构体字段注入和构造函数参数注入。

## 结构体字段注入

字段注入通过 `autowire` 或 `inject` 标签声明依赖：

```go
type UserController struct {
	Service UserService `autowire:""`
}
```

容器创建 `UserController` 后，会根据字段类型和标签内容查找匹配的 Bean，并填充到字段上。

字段注入的优点是直接、简洁，不需要额外构造函数。对于依赖关系简单、对象只是承载少量外部组件的场景，它能减少样板代码。

需要注意的是，字段注入会让依赖在对象创建后才被填充。构造函数内部不能使用这些字段，初始化逻辑也要放到依赖注入完成之后的生命周期阶段。

## 构造函数参数注入

构造函数注入通过函数参数声明依赖：

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

容器在创建 Bean 时会分析构造函数参数，并注入匹配的依赖。

这种方式符合 Go 的普通构造函数写法，不需要特殊语法。依赖在对象创建时就已经完整传入，因此边界更清晰，也更利于保持对象不可变。

## 如何选择

大多数情况下，更推荐构造函数注入：

- 依赖关系集中体现在构造函数签名上。
- 对象创建后依赖已经完整。
- 更容易手写纯单元测试。
- 不依赖结构体字段可导出性和标签解析。

字段注入适合以下场景：

- 依赖关系非常简单，写构造函数收益不高。
- 需要和配置字段放在同一个结构体上声明。
- 某些循环依赖场景需要先创建实例再填充字段。

这不是二选一的教条。关键是让依赖关系在代码中足够清楚，同时不要为简单对象制造过多模板。

## 构造函数注入与参数标签

Go 语言不能给函数参数添加 tag。Go-Spring 因此在注册构造函数时使用 `Arg` 参数补充绑定信息，例如 `gs.TagArg`。

```go
func NewRepository(ds *DataSource) *Repository {
	return &Repository{ds: ds}
}

func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

这里的 `TagArg("slave")` 表示构造函数第一个参数注入名为 `slave` 的 Bean。

简单按类型注入时可以省略 `TagArg`，容器会自动推断。

## 边界

本篇只讨论依赖声明的形态。下一篇会展开注入目标：按类型注入、按名称注入、可空注入、集合注入、配置项驱动注入和延迟注入。

