# Go-Spring 实战第 13 课：Bean 元信息：名称、接口导出、生命周期和条件配置

我们已经知道 Go-Spring 的 Bean 可以由结构体指针、构造函数或函数指针产生。但一个 Bean 真正进入容器后，只靠类型就不够了。

真实工程中还会为 Bean 声明名称、生命周期回调、导出接口、启用条件、显式依赖和 root 入口。这些信息决定 Bean 如何被查找、何时创建、是否生效，以及在应用生命周期中扮演什么角色。

Go-Spring 使用链式 API 为 Bean 附加这些元信息。

可以把元信息理解成 Bean 的运行说明书——类型决定它是什么，元信息决定它在容器里怎样被使用。

## 名称就是多实例场景的依赖协议

Go-Spring 使用“类型 + 名称”标识 Bean。未显式指定名称时，默认使用类型的简短名称。

同类型多实例需要使用 `.Name()`。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}
```

随后就可以按名称注入。

```go
type UserRepo struct {
	ds *DataSource `autowire:"slave"`
}

func init() {
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}
```

名称不是装饰信息，而是多实例场景下的依赖协议。因为调用方会按这个名称选择目标 Bean，所以它和配置 key 一样，后续会进入调用方和配置之间的约定。

## 初始化和销毁回调要进生命周期

初始化回调在 Bean 创建并完成依赖注入后执行；销毁回调在 Go-Spring 容器退出时执行。

如果生命周期动作更像注册时附加的外部动作，可以用函数指针挂到 Bean 定义上。

```go
func InitMyService(s *MyService) error {
	s.client = redis.NewClient(/* ... */)
	return s.client.Ping().Err()
}

func DestroyMyService(s *MyService) error {
	return s.client.Close()
}

func init() {
	gs.Provide(NewMyService).
		Init(InitMyService).
		Destroy(DestroyMyService)
}
```

如果初始化失败，容器会启动失败；如果销毁失败，错误会被记录，但不会阻塞容器退出。

如果初始化和销毁本来就是对象自己的行为，可以在类型上定义方法，再在注册时声明方法名。

```go
type MyService struct {
	client *redis.Client
}

func (s *MyService) Init() error {
	s.client = redis.NewClient(/* ... */)
	return s.client.Ping().Err()
}

func (s *MyService) Destroy() error {
	return s.client.Close()
}

func init() {
	gs.Provide(NewMyService).
		InitMethod("Init").
		DestroyMethod("Destroy")
}
```

如果初始化和销毁逻辑本来就是对象行为，方法名方式更自然。反过来，如果生命周期动作更像注册时补上的外部动作，用外部函数也能把它清楚地挂到注册处。

## 接口导出必须显式声明

Go 的接口是隐式实现的。一个结构体可能实现多个接口，甚至无意中实现某个接口。所以 Go-Spring 要求显式导出接口。

```go
type UserService interface {
	Get(id int) (*User, error)
}

type UserServiceImpl struct{}

func NewUserServiceImpl() *UserServiceImpl {
	return &UserServiceImpl{}
}

func init() {
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())
}
```

这样既保留具体类型 Bean，也额外注册接口类型 Bean。依赖方按什么类型声明，就注入对应形式，关系会更明确。

如果构造函数直接返回接口类型，则不需要额外 `Export`。

```go
func NewUserService() UserService {
	return &userServiceImpl{}
}
```

显式导出让接口关系在注册处可见，也减少了“碰巧实现了某个接口”带来的意外注入。换句话说，接口注入不是靠猜，而是靠注册时明确声明。

## 条件也属于 Bean 的元信息

Bean 可以通过 `.Condition()` 声明启用条件。

```go
func init() {
	gs.Provide(NewDevLogger).Condition(
		gs.OnProperty("spring.profiles.active").
			HavingValue("expr:contains($, 'dev')"),
	)
}
```

条件注册会在后续文章单独展开。这里先把它简单理解为 Bean 元信息的一部分，即 Bean 注册了，不代表最终一定生效。

## 没有注入关系时用 DependsOn 补顺序

大多数依赖顺序可以通过注入关系推断。如果两个 Bean 没有直接注入关系，但仍需控制初始化顺序，可以使用 `.DependsOn()`。

```go
func init() {
	gs.Provide(NewB).DependsOn(gs.BeanIDFor[A]())
}
```

这表示 `B` 在初始化顺序上依赖 `A`。销毁时则按相反顺序执行。

## Root Bean 决定依赖图从哪里开始

Go-Spring 默认按需创建 Bean，需要从 root bean 出发递归创建依赖图。

实现 `gs.Runner` 或 `gs.Server` 的 Bean 会自动成为 root bean。如果应用关闭了内置 Server，也没有 Runner，但仍希望某个对象进入容器初始化流程，可以使用 `app.Root()`。

```go
func main() {
	bootstrap := &Bootstrap{}

	gs.Configure(func(app gs.App) {
		app.Root(bootstrap)
	}).Run()
}
```

这样会把已有对象纳入 Go-Spring 容器管理，并作为依赖图入口。

## 元信息最终会改变容器行为

Bean 的元信息决定它在 Go-Spring 容器中如何被命名、如何暴露接口、是否参与装配、何时执行生命周期回调，以及是否作为根对象驱动依赖图。

单个 Bean 的元信息讲清楚以后，视角就可以扩到注册入口，即 `gs.Provide()`、`gs.Module()`、`gs.Group()`、`Configuration` 和 `app.Provide()` 分别负责不同组织边界。
