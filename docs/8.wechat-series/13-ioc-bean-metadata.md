# Go-Spring 实战第 13 课 —— Bean 元信息：名称、生命周期、接口导出、条件和显式依赖

在我们注册 Bean 时，可能会遇到各种需求。

比如有多个同类型的 Bean 实例，那么我们希望为每个 bean 命名。比如我们希望在 bean 对象创建之后，执行一些初始化动作。比如我们希望在容器结束时，执行一些销毁动作。比如我们希望 bean 能实现不同的接口，尽管大家都共享底层实现，但是对外是不同的身份。比如我们希望在某些条件下才激活当前 bean，条件不满足则不创建它。等等。

Go-Spring 在注册 bean 的时候，我们可以为 Bean 添加各种元信息。可以帮助我们实现这些需求。

## Bean 名称

名称最常见的用途，是区分同类型的多个 Bean。

如果容器里只有一个 `*DataSource`，依赖方只写类型就够了。

```go
type UserRepository struct {
	DS *DataSource `autowire:""`
}
```

但一旦同类型有多个实例，类型就只能说明“我要一个 `*DataSource`”，不能说明“我要哪一个”。这时就要在注册处给 Bean 命名，再在依赖处使用这个名字。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewReplicaDataSource).Name("replica")
}

type UserRepository struct {
	DS *DataSource `autowire:"replica"`
}
```

这里的 `replica` 不是展示名称，而是依赖选择条件。字段注入用 `autowire:"replica"`，构造函数注入则用 `gs.TagArg("replica")`。

```go
func NewUserRepository(ds *DataSource) *UserRepository {
	return &UserRepository{DS: ds}
}

func init() {
	gs.Provide(NewUserRepository, gs.TagArg("replica"))
}
```

所以命名要尽量表达实例差异。`master`、`replica`、`readonly` 这类名称能让人直接看出用途；临时编号、缩写或含义不明的名字，会让多实例注入变得难排查。

## 生命周期动作

有些 Bean 被容器接管以后，还需要在固定时机执行额外动作。启动时要检查资源是否可用，退出时要释放连接、停止后台任务或刷写缓冲区，这些都属于生命周期动作。

这和 Bean 是通过构造函数注册，还是通过结构体指针注册没有必然关系。结构体指针注册时，对象在注册前已经创建好了，但它仍然可能需要容器在注入完成后执行初始化，在退出时执行销毁。

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

`Init` 会在 Bean 创建并完成依赖注入后执行。如果 `Init` 返回错误，Go-Spring 会终止启动，因为继续运行只会得到一组不可用的 Bean。

`Destroy` 会在容器退出时执行，适合关闭连接、停止后台任务、刷写缓冲区。销毁失败会被记录，但退出流程仍然要继续处理其他资源。

如果初始化和销毁动作本来就是对象自己的方法，可以直接声明方法名。这种写法在结构体指针注册时也很常见。

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
