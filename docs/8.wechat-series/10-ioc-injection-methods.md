# Go-Spring 实战第 10 课 —— 依赖注入的方式：字段注入和构造函数注入

上一篇咱们把 Go-Spring 使用 IoC 容器的初衷和背景讲清楚了，偏抽象。本文开始，咱们讲讲 Go-Spring IoC 容器的实际用法。

第一篇来介绍如何使用依赖注入，依赖注入发生在哪些地方。当然，其实也很简单：一是字段注入，二是构造函数注入。

## 字段注入

字段注入是指把依赖写在结构体的字段上，通过 `autowire`（或 `inject`）标签告诉 Go-Spring 要注入什么。

这种写法最简单也最直观。看个例子：

```go
type UserService struct{}

type UserController struct {
	Service *UserService `autowire:""`
}

func init() {
	gs.Provide(new(UserService))
	gs.Provide(new(UserController))
}
```

在上面的代码中，`autowire:""` 标签没有值，表示只需按照类型注入。因为这里 `UserController.Service` 的类型是 `*UserService`，容器里也只有一个 `*UserService`，所以不需要写名字。

如果同一个类型有多个 Bean，就需要在标签里写注入的 Bean 的名称。示例如下：

```go
type Repository struct {
	DB *DataSource `autowire:"slave"`
}

func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
	gs.Provide(new(Repository))
}
```

在上面的代码中，我们注册了 `master` 和 `slave` 两个 `*DataSource` 的 Bean。如果 `Repository.DataSource` 仍然只按照类型注入，就会发现有两个相同的 `*DataSource`。这种情况下，我们就需要在 `autowire` 标签里写清楚注入的 Bean 的名称。

## 构造函数注入

构造函数注入是指把依赖写在构造函数参数里，Go-Spring 创建 Bean 时会首先分析构造函数的参数类型，然后根据参数类型从容器里找到对应的依赖，然后使用反射调用构造函数来创建出 Bean。

示例如下：

```go
type UserController struct {
	service *UserService
}

func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(new(UserService))
	gs.Provide(NewUserController)
}
```

使用构造函数注入有两个好处。第一，依赖全都在函数签名上，一眼就能看到这个对象需要什么。第二，单元测试可以直接调用 `NewUserController(mockService)`，不一定要启动完整的 IoC 容器来填满所有依赖。

我们也可以在执行构造函数的时候返回 error，这样可以暴露更详细的错误信息。

示例如下：

```go
func NewUserController(service *UserService) (*UserController, error) {
	if service == nil {
		return nil, errors.New("missing user service")
	}
	return &UserController{service: service}, nil
}
```

todo （这里缺一段）

## 构造函数参数

我们对比上面示例的时候可以发现，字段注入可以通过名字指定注入的 Bean 名称，那是因为我们使用 Go 提供的 Tag 机制。但是 Go 没有在构造函数参数上提供 Tag 机制，我们还能指定参数的 Bean 名称么？

看个例子。

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

虽然 Go 没有提供在构造函数参数上写 Tag 的机制，但是 Go-Spring 通过 `gs.Provide` 提供了为构造函数参数指定 Bean 名称的功能。`gs.TagArg("slave")` 的意思和字段上的 `autowire:"slave"` 一样，都是指定注入名为 `slave` 的 Bean。

如果构造函数的参数都是 bean 注入，而且都可以通过类型进行唯一匹配，那么我们可以省略 `TagArg`。

示例如下：

```go
func init() {
	gs.Provide(NewDataSource)
	gs.Provide(NewRepository)
}
```

对于更复杂的构造函数参数注入，我们会在后面的章节专门介绍，这里就不展开了。

## 哪种方式更好

我们推荐在所有情况下都使用构造函数注入，因为它具有更强的规则性和统一性。但是我们也不排斥在简单的情况下使用字段注入。
