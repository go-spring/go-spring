# Go-Spring 实战第 13 课 —— Bean 元信息：名称、导出、生命周期和 root 入口如何改变装配行为

Bean 的创建方式决定 Go-Spring 容器怎样拿到对象。但真实工程里，只知道对象类型还不够。

同一个接口可能有多个实现，一个对象可能需要初始化和销毁，一个组件可能只在特定条件下生效，还有一些 Bean 虽然没有被别人注入，却必须驱动应用启动。遇到这些情况时，类型只能回答“它是什么”，不能回答“它在本次装配里怎样被使用”。

Go-Spring 通过 Bean 元信息补上这些运行语义。名称、接口导出、生命周期、条件、显式依赖和 root 入口，都会改变 Bean 在容器中的行为。

## Bean 名称

下面的例子证明名称不是展示信息，而是多实例场景里的依赖协议。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type UserRepo struct {
	ds *DataSource `autowire:"slave"`
}
```

Go-Spring 使用“类型 + 名称”标识 Bean。未显式指定名称时，默认名称来自类型；当同类型只有一个实例时，默认名称通常不会进入业务代码。

同类型多实例一出现，名称就会成为注册方和依赖方之间的协议。构造函数注入也使用同样语义，只是名称放在注册阶段的 `TagArg` 里。

```go
func init() {
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}
```

因此，Bean 名称要稳定、可读，并且能表达实例差异。命名越清楚，排查多实例注入问题时越直接。

## 接口导出

Go 的接口是隐式实现的。一个结构体可能实现多个接口，也可能无意中满足某个接口。如果容器自动把结构体导出为所有可匹配接口，注入结果就会变得难以推断。

下面的例子证明 Go-Spring 要求在注册处显式声明接口导出。

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

## 生命周期回调

很多对象不是创建出来就结束了。连接池需要建立连接，消费者需要启动前检查，应用退出时还要关闭资源。下面的例子证明生命周期回调会进入 Go-Spring 的统一顺序。

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

初始化回调在 Bean 创建并完成依赖注入后执行，初始化失败会让容器启动失败。销毁回调在 Go-Spring 容器退出时执行，销毁失败会被记录，但不会阻塞容器退出。

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

## 条件

Bean 注册进容器，不代表本次启动一定生效。可选组件、默认实现和环境实现都需要在启动解析阶段判断。下面的例子证明条件属于 Bean 元信息，而不是业务运行期分支。

```go
func init() {
	gs.Provide(NewDevLogger).Condition(
		gs.OnProperty("spring.profiles.active").
			HavingValue("expr:contains($, 'dev')"),
	)
}
```

Go-Spring 会在解析阶段计算条件。满足条件的 Bean 才会进入后续依赖图，不满足条件的 Bean 会被裁剪。

放在元信息这里看，重点是理解一件事：注册描述候选定义，条件决定候选定义是否参与本次装配。条件不是业务代码里的 `if` 分支，而是对象图形成前的筛选规则。

## DependsOn

大多数初始化顺序可以通过注入关系推断。如果 `B` 注入了 `A`，Go-Spring 容器自然会先准备好 `A`。

有些对象没有直接注入关系，却仍然需要控制初始化顺序。下面的例子证明 `.DependsOn()` 只补充顺序约束。

```go
func init() {
	gs.Provide(NewB).DependsOn(gs.BeanIDFor[A]())
}
```

这表示 `B` 在初始化顺序上依赖 `A`。销毁时则按相反顺序执行。

`DependsOn` 适合补充纯顺序约束，不适合替代真正的依赖注入。如果对象运行时确实需要使用另一个对象，应该把依赖显式声明出来。

## Root Bean

Go-Spring 默认按需创建 Bean。容器不会把所有注册定义都无条件实例化，而是从 root bean 出发递归创建依赖图。

实现 `gs.Runner` 或 `gs.Server` 的 Bean 会自动成为 root bean。如果应用关闭了内置 Server，也没有 Runner，但仍希望某个对象进入容器初始化流程，可以使用 `app.Root()`。下面的例子证明 root 入口可以显式指定。

```go
func main() {
	bootstrap := &Bootstrap{}

	gs.Configure(func(app gs.App) {
		app.Root(bootstrap)
	}).Run()
}
```

这个例子里，`bootstrap` 会被纳入 Go-Spring 容器管理，并作为依赖图入口。`Root Bean` 解决的是“谁来驱动本次对象图展开”的问题；它不改变 Bean 的创建规则，只改变对象图从哪里开始展开。

## Bean 元信息

Bean 的类型说明它是什么，元信息说明它在 Go-Spring 容器里怎样被使用。

名称用于多实例识别，接口导出用于依赖协议，生命周期用于启动和退出动作，条件用于判断本次启动是否生效，显式依赖用于补充纯顺序约束，root 入口用于决定对象图从哪里展开。Go-Spring 正是通过这些元信息，把类型之外的运行语义留在容器可以统一解析的位置。
