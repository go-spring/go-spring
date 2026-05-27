# Go-Spring 实战第 13 课 —— Bean 元信息：名称、生命周期、接口导出、条件和显式依赖

我们在注册 Bean 时，除了告诉容器如何创建对象，通常还需要补充一些其他信息。例如：

- 当同类型的 Bean 有多个实例时，需要为它们分别命名，以作区分；
- 在 Bean 实例创建完成之后，需要执行一些初始化动作；
- 在 IoC 容器退出时，需要对 Bean 实例执行一些销毁动作；
- 同一个 Bean 实例需要以不同的接口身份对外暴露；
- 某些 Bean 实例只应在特定条件满足时激活，否则就不创建。

在 Go-Spring 中，这些需求都可以通过 Bean 注册时附加的元信息来表达。

## Bean 名称

在 Go-Spring 中，表示一个 Bean 需要类型和名字两个信息。在容器中只有一个同类型的 bean 时，我们通常不同关心它的名字。使用默认生成的名字即可。但如果容器中多个同类型的 bean 时，我们需要为它们分别命名。

看个例子。

```go
type UserRepository struct {
	DS *DataSource `autowire:"replica"`
}

func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewReplicaDataSource).Name("replica")
}
```

上面的代码中，我们在注册 bean 的时候，注册了 `master` 和 `replica` 两个 `dataSource` 类型的 bean。使用的时候，只用到了 `replica` 这一个 bean。

我们可以在结构体字段上指定要注入的 bean 的名字，也可以为构造函数的参数绑定指定要注入的 bean 的名字。代码如下：

```go
func NewUserRepository(ds *DataSource) *UserRepository {
	return &UserRepository{DS: ds}
}

func init() {
	gs.Provide(NewUserRepository, gs.TagArg("replica"))
}
```

需要注意的是，命名要尽量表达实例之间的差异。比如 `master`、`replica`、`readonly` 这类名称能让人直接看出用途。不要是含混不清。。。

## 生命周期

Go-Spring 支持在实例创建之后执行初始化动作，支持在容器退出时执行销毁动作。

我们可以使用 `Init` 设置初始化动作，使用 `Destroy` 设置销毁动作。`Init` 注册的回调函数会在 Bean 创建并且完成依赖注入后执行。如果 `Init` 返回错误，Go-Spring 会终止启动。`Destroy` 注册的回调函数会在容器退出时执行，适合关闭连接、停止后台任务、刷写缓冲区等。销毁失败会被记录，但退出流程会继续。

看个例子。

```go
func CheckRedisClient(c *RedisClient) error {
	return c.Ping()
}

func CloseRedisClient(c *RedisClient) error {
	return c.Close()
}

func init() {
	gs.Provide(&RedisClient{}).
		Init(CheckRedisClient).
		Destroy(CloseRedisClient)
}
```

无论是 Init 还是 Destroy，它们的函数原型都是一样的，都是 `func(*Bean)` 或者 `func(*Bean) error`。

如果初始化和销毁动作本来就是对象自己的方法，我们也可以直接声明方法名。示例如下：

```go
type Worker struct{}

func (w *Worker) Start() error {
	return nil
}

func (w *Worker) Stop() error {
	return nil
}

func init() {
	gs.Provide(NewWorker).
		InitMethod("Start").
		DestroyMethod("Stop")
}
```

这里的判断标准很实用：只要动作需要纳入 Go-Spring 的启动和退出顺序，就放到 `Init`、`Destroy` 里。这样无论 Bean 来自构造函数还是已有对象，容器都能用同一套生命周期处理它。

## 接口导出

Go 里接口是隐式实现的。一个结构体只要方法集合匹配，就实现了某个接口。这个特性很方便，但容器不能因此自动把结构体暴露成所有可能的接口。

原因很简单：结构体可能实现很多接口，也可能无意中满足某个接口。如果容器自动推断，依赖方按接口注入时，很难知道这个实现是不是作者有意暴露出来的。

Go-Spring 的做法是让注册语句显式说明接口导出关系。

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

`.Export(gs.As[UserService]())` 的意思是：这个实现除了按 `*UserServiceImpl` 参与装配，也允许按 `UserService` 接口被注入。

如果一个 Bean 需要承担多个角色，可以导出多个接口。

```go
func init() {
	gs.Provide(NewJob).
		Export(gs.As[Job]()).
		Export(gs.As[gs.Runner]())
}
```

如果构造函数本身就返回接口类型，则不需要再写 `Export`。

```go
func NewUserService() UserService {
	return &UserServiceImpl{}
}
```

这两种写法都可以。关键是接口关系要在代码里明确出现，而不是让容器猜。

## 装配条件

注册 Bean 不等于本次启动一定使用它。

可选组件、默认实现、环境实现和 Starter 扩展，经常需要根据配置或已有 Bean 来决定是否启用。这个判断应该发生在启动阶段，而不是在每次业务调用里写 `if`。

```go
func init() {
	gs.Provide(NewAuditLogger).Condition(
		gs.OnProperty("audit.enabled").HavingValue("true"),
	)
}
```

上面这段代码表示：只有 `audit.enabled=true` 时，`NewAuditLogger` 这个候选 Bean 才参与本次装配。条件不满足时，它不会被创建，也不会执行 `Init` 或 `Destroy`。

按环境启用 Bean 时，用 `.OnProfiles()` 更直接。

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

`.OnProfiles("dev")` 表示这个 Bean 属于 `dev` Profile。它和手写 `OnProperty("spring.profiles.active")` 能表达相近结果，但读起来更清楚：这是环境装配规则，不是普通功能开关。

条件的边界也要守住。`Condition` 适合决定一个基础设施组件、默认实现或环境实现是否参与启动；订单状态、用户类型、租户策略这类运行期业务分支，仍然应该写在业务代码里。

## 显式依赖

大多数初始化顺序不需要手写。只要 `Service` 注入了 `Repository`，Go-Spring 就会先准备 `Repository`，再创建 `Service`。

但有些对象没有直接注入关系，却仍然需要顺序约束。比如缓存预热任务并不调用迁移器对象，但它必须等数据库迁移完成后再执行。这时可以使用 `DependsOn`。

```go
func init() {
	gs.Provide(NewDatabaseMigrator).Name("main")

	gs.Provide(NewCacheWarmer).
		DependsOn(gs.BeanIDFor[*DatabaseMigrator]("main"))
}
```

这表示 `NewCacheWarmer` 对应的 Bean 在初始化顺序上依赖名为 `main` 的 `*DatabaseMigrator`。退出时，Go-Spring 会按相反顺序处理。

`DependsOn` 只应该补充顺序约束，不应该隐藏真正的运行期依赖。如果一个对象在业务逻辑里确实要调用另一个对象，就把它写成字段或构造函数参数。否则注册语句只能说明顺序，业务代码里却看不出真实依赖。

## Bean 元信息

`gs.Provide(NewService)` 说明 Bean 怎么来，后面的链式调用说明它在容器里怎么用。

`Name` 解决同类型多实例选择，`Init` 和 `Destroy` 接入启动与退出，`Export` 解决接口注入，`Condition` 和 `OnProfiles` 控制本次启动是否启用，`DependsOn` 补充初始化顺序。

把这些信息写在注册处，代码读起来会更直接：看到注册语句，就能知道这个 Bean 的名称、生命周期动作、接口身份、生效条件和顺序约束。业务代码拿到的则是已经完成装配的普通 Go 对象，不需要在运行时再回头理解容器规则。
