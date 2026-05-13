# Bean 元信息配置

## 本篇要解决的问题

Bean 不只是一个值或一个构造函数。真实工程中还需要为 Bean 声明名称、生命周期回调、导出接口、启用条件、显式依赖和 root 入口。

Go-Spring 使用链式 API 为 Bean 附加这些元信息。

## 设置 Bean 名称

Go-Spring 使用“类型 + 名称”标识 Bean。未显式指定名称时，默认使用类型的简短名称。

同类型多实例需要使用 `.Name()`：

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}
```

随后可以按名称注入：

```go
type UserRepo struct {
	ds *DataSource `autowire:"slave"`
}

func init() {
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}
```

## 生命周期回调

初始化回调在 Bean 创建并完成依赖注入后执行；销毁回调在容器退出时执行。

通过函数指针设置：

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

初始化失败会导致容器启动失败。销毁失败会被记录，但不会阻塞容器退出。

通过方法名设置：

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

如果初始化和销毁逻辑本来就是对象行为，方法名方式更自然。

## 导出为接口

Go 的接口是隐式实现的。一个结构体可能实现多个接口，甚至无意中实现某个接口。Go-Spring 因此要求显式导出接口：

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

这样既保留具体类型 Bean，也额外注册接口类型 Bean。依赖方按什么类型声明，就注入对应形式。

如果构造函数直接返回接口类型，则不需要额外 `Export`：

```go
func NewUserService() UserService {
	return &userServiceImpl{}
}
```

## 附加激活条件

Bean 可以通过 `.Condition()` 声明启用条件：

```go
func init() {
	gs.Provide(NewDevLogger).Condition(
		gs.OnProperty("spring.profiles.active").
			HavingValue("expr:contains($, 'dev')"),
	)
}
```

条件注册会在后续文章单独展开。本篇只需要理解它是 Bean 元信息的一部分：Bean 注册了，不代表最终一定生效。

## 显式依赖声明

大多数依赖顺序可以通过注入关系推断。如果两个 Bean 没有直接注入关系，但仍需控制初始化顺序，可以使用 `.DependsOn()`：

```go
func init() {
	gs.Provide(NewB).DependsOn(gs.BeanIDFor[A]())
}
```

这表示 `B` 在初始化顺序上依赖 `A`。销毁时则按相反顺序执行。

## 标记为根 Bean

Go-Spring 默认按需创建 Bean，需要从 root bean 出发递归创建依赖图。

实现 `gs.Runner` 或 `gs.Server` 的 Bean 会自动成为 root bean。如果应用关闭了内置 Server，也没有 Runner，但仍希望某个对象进入容器初始化流程，可以使用 `app.Root()`：

```go
func main() {
	bootstrap := &Bootstrap{}

	gs.Configure(func(app gs.App) {
		app.Root(bootstrap)
	}).Run()
}
```

这会把已有对象纳入容器管理，并作为依赖图入口。

## 下一篇

本篇讨论单个 Bean 的元信息。下一篇会系统梳理 Bean 注册入口：`gs.Provide()`、`gs.Module()`、`gs.Group()`、`Configuration` 和 `app.Provide()`。

