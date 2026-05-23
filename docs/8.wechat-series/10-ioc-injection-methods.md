# Go-Spring 实战第 10 课 —— 依赖注入的方式：字段注入和构造函数注入

上一篇咱们把 IoC 容器的定位讲清楚了。这一篇只讲写代码时最常用的两种注入方式：字段注入和构造函数注入。

字段注入就是把依赖写在结构体字段上，通过 `autowire` 标签告诉 Go-Spring 要注入什么。构造函数注入就是把依赖写在构造函数参数里，Go-Spring 创建 Bean 时先准备好参数，再调用构造函数。

## 字段注入

字段注入的写法最直观。结构体需要什么依赖，就在字段上加 `autowire` 标签。

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

`autowire:""` 表示按类型注入。这里 `UserController.Service` 的类型是 `*UserService`，容器里也只有一个 `*UserService`，所以标签里不需要写名字。

如果同一个类型有多个 Bean，就在标签里写 Bean 名称。

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

这段代码表示 `Repository.DB` 注入名为 `slave` 的 `*DataSource`。

使用字段注入时要注意一点：字段是在对象创建之后才被 Go-Spring 填进去的。所以不要在结构体初始化时就使用这些字段。像 Controller、Runner、简单的业务组件，通常都适合用字段注入，因为写法少，代码也直接。

## 构造函数注入

构造函数注入是另一种常见写法：把依赖放到构造函数参数里。

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

`NewUserController` 的参数类型是 `*UserService`，Go-Spring 会先找到这个 Bean，再调用 `NewUserController(service)` 创建 `UserController`。

这种写法有两个好处。第一，依赖都在函数签名里，一眼能看到这个对象需要什么。第二，单元测试可以直接调用 `NewUserController(mockService)`，不一定要启动容器来填字段。

如果创建过程可能失败，构造函数也可以返回 error。

```go
func NewUserController(service *UserService) (*UserController, error) {
	if service == nil {
		return nil, errors.New("missing user service")
	}
	return &UserController{service: service}, nil
}
```

## 构造函数参数怎么指定名字

字段注入可以写 `autowire:"slave"`，但构造函数参数不能写 tag。这个时候要在 `gs.Provide` 里用 `gs.TagArg` 指定参数。

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

`gs.TagArg("slave")` 的意思和字段上的 `autowire:"slave"` 一样，都是指定注入名为 `slave` 的 Bean。区别只是位置不同：字段注入写在字段 tag 上，构造函数注入写在注册语句里。

如果参数按类型就能唯一匹配，`TagArg` 可以省略。

```go
func init() {
	gs.Provide(NewDataSource)
	gs.Provide(NewRepository)
}
```

只有同类型多个 Bean、需要按名称指定时，才需要在 `gs.Provide` 里写 `TagArg`。

## 怎么选

实际写代码时不用把这个问题想得太复杂。

如果只是普通 Controller、Runner，或者结构体里有几个依赖字段，用字段注入就可以。

如果你希望创建对象时就把依赖传进去，或者单元测试里直接手写构造函数，那就用构造函数注入。

如果要指定 Bean 名称，字段注入写 `autowire:"name"`，构造函数注入写 `gs.TagArg("name")`。

本章先掌握这几种写法就够了：字段注入看字段 tag，构造函数注入看函数参数，参数需要额外说明时再用 `TagArg`。
