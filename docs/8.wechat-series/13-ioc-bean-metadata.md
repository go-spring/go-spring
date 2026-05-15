# Go-Spring 实战第 13 课：Bean 元信息：名称、导出、生命周期和 root 入口怎样改变装配行为

Bean 的创建方式决定 Go-Spring 容器怎样拿到对象。但真实工程里，只知道对象类型还不够。

同一个接口可能有多个实现，一个对象可能需要初始化和销毁，一个组件可能只在特定条件下生效，还有一些 Bean 虽然没有被别人注入，却必须驱动应用启动。遇到这些情况时，类型只能回答“它是什么”，不能回答“它在本次装配里怎样被使用”。

Go-Spring 通过 Bean 元信息补上这些运行语义。名称、接口导出、生命周期、条件、显式依赖和 root 入口，都会改变 Bean 在容器中的行为。

## Bean 名称是多实例场景的依赖协议

Go-Spring 使用“类型 + 名称”标识 Bean。未显式指定名称时，默认使用类型的简短名称。

当同类型只有一个实例时，默认名称通常不会进入业务代码。但同类型多实例一出现，名称就会成为依赖协议的一部分。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}
```

依赖方随后可以按名称选择目标 Bean。

```go
type UserRepo struct {
	ds *DataSource `autowire:"slave"`
}
```

构造函数注入也使用同样语义，只是名称放在注册阶段的 `TagArg` 里。

```go
func init() {
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}
```

因为调用方会按这个名称选择目标，所以名称不是展示信息，而是注册方、依赖方和配置之间的约定。命名越稳定，后续排查多实例注入问题时越直接。

## 生命周期回调要进入容器顺序

初始化回调在 Bean 创建并完成依赖注入后执行，销毁回调在 Go-Spring 容器退出时执行。把这些动作交给容器，核心价值是让它们进入统一顺序，而不是散落在业务启动和退出分支里。

如果生命周期动作更像注册时附加的外部行为，可以用函数指针挂到 Bean 定义上。

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

初始化失败会让容器启动失败；销毁失败会被记录，但不会阻塞容器退出。这个语义保留了启动期失败的硬约束，也避免退出流程因为单个清理错误卡住。

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

方法名方式适合对象自带生命周期。外部函数方式适合在注册处补充集成动作。两者进入的是同一条容器生命周期链路。

## 接口导出需要在注册处明确

Go 的接口是隐式实现的。一个结构体可能实现多个接口，也可能无意中满足某个接口。如果容器自动把结构体导出为所有可匹配接口，注入结果就会变得难以推断。

Go-Spring 要求注册处显式声明接口导出。

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

这样既保留具体类型 Bean，也额外暴露接口类型 Bean。依赖方按具体类型声明，就拿到具体实现；按接口声明，就走接口导出关系。

如果构造函数直接返回接口类型，就不需要额外 `Export`。

```go
func NewUserService() UserService {
	return &userServiceImpl{}
}
```

显式导出让接口关系停留在注册语句里，而不是依赖容器猜测结构体“可能应该”暴露哪些接口。

## 条件属于 Bean 是否生效的装配信息

Bean 注册进容器，不代表本次启动一定生效。可选组件、默认实现和环境实现都需要在启动解析阶段判断。

```go
func init() {
	gs.Provide(NewDevLogger).Condition(
		gs.OnProperty("spring.profiles.active").
			HavingValue("expr:contains($, 'dev')"),
	)
}
```

这里的条件不是业务运行期分支，而是 Bean 元信息的一部分。Go-Spring 会在解析阶段计算条件，满足条件的 Bean 才会进入后续依赖图。

条件注册会在后面的文章展开。放在元信息这里看，重点是理解一件事，注册描述候选定义，条件决定候选定义是否参与本次装配。

## DependsOn 只补没有注入关系的顺序

大多数初始化顺序可以通过注入关系推断。如果 `B` 注入了 `A`，Go-Spring 容器自然会先准备好 `A`。

有些对象没有直接注入关系，却仍然需要控制初始化顺序，这时可以使用 `.DependsOn()`。

```go
func init() {
	gs.Provide(NewB).DependsOn(gs.BeanIDFor[A]())
}
```

这表示 `B` 在初始化顺序上依赖 `A`。销毁时则按相反顺序执行。

`DependsOn` 适合补充纯顺序约束，不适合替代真正的依赖注入。如果对象运行时确实需要使用另一个对象，应该把依赖显式声明出来。

## Root Bean 决定按需创建从哪里展开

Go-Spring 默认按需创建 Bean。容器不会把所有注册定义都无条件实例化，而是从 root bean 出发递归创建依赖图。

实现 `gs.Runner` 或 `gs.Server` 的 Bean 会自动成为 root bean。如果应用关闭了内置 Server，也没有 Runner，但仍希望某个对象进入容器初始化流程，可以使用 `app.Root()`。

```go
func main() {
	bootstrap := &Bootstrap{}

	gs.Configure(func(app gs.App) {
		app.Root(bootstrap)
	}).Run()
}
```

这样会把已有对象纳入 Go-Spring 容器管理，并作为依赖图入口。它解决的是“谁来驱动本次对象图展开”的问题。

## 元信息让 Bean 从类型变成装配节点

Bean 的类型说明它是什么，元信息说明它在 Go-Spring 容器里怎样被命名、怎样暴露接口、是否参与装配、何时初始化和销毁，以及是否作为 root 入口驱动依赖图。

单个 Bean 的元信息清楚以后，视角就可以扩到注册入口。`gs.Provide()`、`gs.Module()`、`gs.Group()`、`Configuration` 和 `app.Provide()` 解决的是不同组织边界下怎样把 Bean 定义放进容器。
