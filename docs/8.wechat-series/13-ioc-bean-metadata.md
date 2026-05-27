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

Go-Spring 支持在实例创建之后执行初始化动作，支持在容器退出时执行销毁动作。我们可以使用 `Init` 设置初始化动作，使用 `Destroy` 设置销毁动作。

`Init` 注册的回调函数会在 Bean 创建并且完成依赖注入后执行。如果 `Init` 返回错误，Go-Spring 会终止启动。

`Destroy` 注册的回调函数会在容器退出时执行，适合关闭连接、停止后台任务、刷写缓冲区等。销毁失败会被记录，但退出流程会继续。

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

上面的代码中，我们在注册 `RedisClient` 时，通过独立函数的形式，指定了 `CheckRedisClient` 作为初始化动作，`CloseRedisClient` 作为销毁动作。

如果初始化和销毁动作本来就是对象自己的方法，我们也可以直接声明方法名。示例如下：

```go
type Worker struct{}

func NewWorker() *Worker {
	...
}

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

上面的代码中，我们在注册 `Worker` 时，通过方法名的形式，指定了 `Start` 作为初始化动作，`Stop` 作为销毁动作。

无论是 Init 还是 Destroy，它们的函数原型都是一样的，都是 `func(*Bean)` 或者 `func(*Bean) error`。

> 在 Go 中，方法可以和普通函数一样，只是认为接收者是函数的第一个参数即可。

## 接口导出

在 Go 里接口是隐式实现的，这就导致我们无法预先知道 bean 实例实现了哪些接口。当然我们可以在 bean 匹配的时候对每个 bean 进行类型探测，但是一个是匹配的过程很慢，性能变差，另一个是可能会误判。因此 Go-Spring 要求必须显式导出 bean 实现的接口。

我们可以使用 `Export` 显式导出 bean 实现的接口。代码如下：

```go
type UserService interface {
	Get(id int) (*User, error)
}

todo （缺乏使用案例呢）

type UserServiceImpl struct{}

func NewUserServiceImpl() *UserServiceImpl {
	return &UserServiceImpl{}
}

func init() {
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())
}
```

上面的代码中，我们通过 `.Export(gs.As[UserService]())` 表达了 `UserServiceImpl` 实现了 `UserService` 接口这个含义。

如果一个 Bean 需要承担多个角色，也可以导出多个接口。

```go
todo (下面这个例子不合适)
func init() {
	gs.Provide(NewJob).
		Export(gs.As[Job]()).
		Export(gs.As[gs.Runner]())
}
```

但是如果构造函数本身就返回的是接口类型，那么就不需要再写 `Export` 了，除非它的底层对象还实现了其他接口。

```go
func NewUserService() UserService {
	return &UserServiceImpl{}
}

func init() {
	gs.Provide(NewUserService)
}
```

## 条件

最好的情况是，我们需要哪种 bean，就只注册哪种 bean。如果不需要，就不注册。但是在模块化和框架化的需求下，我们注册的 bean 不一定被需要。我们需要满足某种条件时再激活当前 bean。

Go-Spring 提供了 `Condition` 来实现条件化激活。常见的 `Condition` 有 `OnProperty`、`OnBean`、`OnProfiles` 等。本篇咱们不讲 condition 有哪些，只讲在 bean 注册的时候怎么使用 `Condition`。

看个例子。

```go
func init() {
	gs.Provide(NewAuditLogger).Condition(
		gs.OnProperty("audit.enabled").HavingValue("true"),
	)
}
```

上面这段代码表示，只有 `audit.enabled=true` 时，`NewAuditLogger` 这个候选 Bean 才参与本次装配。条件不满足时，它不会被创建。

如果我们只是想按环境启用 Bean，那么可以使用 `.OnProfiles()`，这样更直接。

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

上面的代码表示，只有当前`spring.profiles.active=dev` 时，`NewDevLogger` 这个候选 Bean 才参与本次装配。条件不满足时，它不会被创建。

需要注意的是，条件滥用会让装配过程变得复杂，因此，除非是框架类的场景，否则不推荐大家滥用 `Condition`。

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
