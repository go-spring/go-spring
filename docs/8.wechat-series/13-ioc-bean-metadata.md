# Go-Spring 实战第 13 课 —— Bean 元信息：把名称、接口、生命周期和条件写在注册处

注册 Bean 不只是调用一次 `gs.Provide`。`gs.Provide` 会返回当前 Bean 的定义，我们可以继续在后面追加名称、接口导出、生命周期、条件和顺序约束。

```go
func init() {
	gs.Provide(NewRedisClient, gs.TagArg("${redis}")).
		Name("cache").
		Export(gs.As[Cache]()).
		Condition(gs.OnProperty("redis.enabled").HavingValue("true")).
		Init(CheckRedisClient).
		Destroy(CloseRedisClient)
}
```

这段代码把一个 Bean 的使用方式一次说清楚了：名称是 `cache`，可以按 `Cache` 接口注入，只有 `redis.enabled=true` 时启用，创建后检查连接，退出时关闭连接。

下面就按这些注册语句里的信息逐个看。

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

## 生命周期动作

构造函数适合创建对象，但不是所有启动动作都应该塞进构造函数。

比如 Redis Client 创建出来以后，可能还要 `Ping` 一次确认连接可用；应用退出时，还要把连接关闭。这类动作和容器启动、退出顺序有关，适合交给 `Init` 和 `Destroy`。

```go
func CheckRedisClient(c *RedisClient) error {
	return c.Ping()
}

func CloseRedisClient(c *RedisClient) error {
	return c.Close()
}

func init() {
	gs.Provide(NewRedisClient).
		Init(CheckRedisClient).
		Destroy(CloseRedisClient)
}
```

`Init` 会在 Bean 创建并完成依赖注入后执行。如果 `Init` 返回错误，Go-Spring 会终止启动，因为继续运行只会得到一组不可用的 Bean。

`Destroy` 会在容器退出时执行，适合关闭连接、停止后台任务、刷写缓冲区。销毁失败会被记录，但退出流程仍然要继续处理其他资源。

如果初始化和销毁动作本来就是对象自己的方法，可以直接声明方法名。

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

这里的判断标准很实用：创建对象必须具备的参数校验，放在构造函数里；依赖注入完成后才能做的探测、启动前检查和退出清理，放在 `Init`、`Destroy` 里。

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

## 创建入口

Go-Spring 默认按需创建 Bean。注册只是提供候选定义，容器不会因为某个 Bean 已经注册，就无条件创建它。

应用里常见的创建入口来自以 `gs.Runner` 或 `gs.Server` 身份参与装配的 Bean。它们代表启动任务或长期服务，会被应用生命周期收集并驱动执行。

如果应用里没有这类入口，但仍希望某个对象在启动时被创建并完成依赖注入，可以使用 `app.Root()`。

```go
type AppEntry struct {
	Service *UserService `autowire:""`
}

func main() {
	gs.Configure(func(app gs.App) {
		app.Root(&AppEntry{})
	}).Run()
}
```

`app.Root()` 不是“创建所有 Bean”的开关，而是指定从哪个 Bean 开始创建。上面的例子里，Go-Spring 会创建 `AppEntry`，再创建它声明的 `UserService` 依赖，以及这些依赖继续需要的 Bean。

如果某个 Bean 注册了却没有执行 `Init`，不要只看注册语句。还要检查两个条件：它是否被条件保留下来，以及它是否会被创建入口带起来。

## Bean 元信息

`gs.Provide(NewService)` 说明 Bean 怎么来，后面的链式调用说明它在容器里怎么用。

`Name` 解决同类型多实例选择，`Export` 解决接口注入，`Init` 和 `Destroy` 接入启动与退出，`Condition` 和 `OnProfiles` 控制本次启动是否启用，`DependsOn` 补充初始化顺序，`app.Root` 指定从哪个 Bean 开始创建。

把这些信息写在注册处，代码读起来会更直接：看到注册语句，就能知道这个 Bean 的名称、接口身份、生效条件和生命周期动作。业务代码拿到的则是已经完成装配的普通 Go 对象，不需要在运行时再回头理解容器规则。
