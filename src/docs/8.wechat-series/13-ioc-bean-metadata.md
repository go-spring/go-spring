# Go-Spring 实战第 13 课 —— Bean 元信息：名称、生命周期、接口导出、条件和显式依赖

我们在注册 Bean 时，除了告诉容器如何创建对象，通常还需要补充一些元信息。例如：

- 当同类型的 Bean 有多个实例时，需要为它们分别命名；
- 在 Bean 实例创建完成之后，需要执行一些初始化动作；
- 在 IoC 容器退出时，需要对 Bean 实例执行一些销毁动作；
- 同一个 Bean 实例需要以不同的接口身份对外暴露；
- 某些 Bean 实例只应在特定条件满足时激活，否则就不创建。

在 Go-Spring 中，这些需求都可以通过 Bean 注册时附加的元信息来表达。

## Bean 名称

在 Go-Spring 中，一个 Bean 由类型和名字两者共同标识。当容器中只有一个同类型 Bean 时，我们通常不用关心它的名字。但当容器中有多个同类型 Bean 时，就需要为它们分别命名。

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

上面的代码中，我们在注册 Bean 的时候，注册了 `master` 和 `replica` 两个 `*DataSource` 类型的 Bean。但是在使用的时候，只用到了 `replica` 这
一个 Bean。

除了在结构体字段上指定要注入的 Bean 名字，我们也可以在构造函数参数绑定时指定。代码如下：

```go
func NewUserRepository(ds *DataSource) *UserRepository {
	return &UserRepository{DS: ds}
}

func init() {
	gs.Provide(NewUserRepository, gs.TagArg("replica"))
}
```

需要注意的是，命名要尽量表达实例之间的差异。比如 `master`、`replica`、`readonly` 这类名称能让人直接看出用途。

## 生命周期

有些 Bean 被容器创建出来之后，还需要在固定时机执行一些额外动作。比如在启动时检查资源是否可用，在退出时关闭连接、停止后台任务或者刷写缓冲区。

在 Go-Spring 中，我们可以使用 `Init` 设置初始化动作，使用 `Destroy` 设置销毁动作。

- `Init` 注册的回调函数会在 Bean 创建并且完成依赖注入后执行。如果 `Init` 返回错误，Go-Spring 会终止启动。
- `Destroy` 注册的回调函数会在容器退出时执行。如果销毁失败，会记录下来，但退出流程会继续。

传给 `Init` 和 `Destroy` 的函数签名是一样的，都是 `func(*Bean)` 或者 `func(*Bean) error`。

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

如果初始化和销毁动作本来就是对象自己的方法，我们也可以使用 `InitMethod` 和 `DestroyMethod` 直接声明方法名。只要方法的签名是 `func()` 或者 `func() error`。

示例如下：

```go
type Worker struct{}

func NewWorker() *Worker {
	return &Worker{}
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

## 接口导出

在 Go 里接口是隐式实现的。一个类型只要方法集合匹配，就实现了某个接口。这个特性很方便，但也意味着一个类型可能无意中满足了很多接口。

Go-Spring 没有把这种隐式实现关系自动扩展成装配身份。如果我们需要表达一个 Bean 实例可以按某个接口参与装配，那么需要显式导出这个接口。

我们可以使用 `Export` 显式导出 Bean 实现的接口。代码如下：

```go
type UserService interface {
	Get(id int) (*User, error)
}

type UserServiceImpl struct{}

func NewUserServiceImpl() *UserServiceImpl {
	return &UserServiceImpl{}
}

func (s *UserServiceImpl) Get(id int) (*User, error) {
	return nil, nil
}

type UserController struct {
	Service UserService `autowire:""`
}

func init() {
	gs.Provide(&UserController{})
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())
}
```

上面的代码中，`UserController` 依赖的是 `UserService` 接口，而实际注册的是 `*UserServiceImpl`。我们通过 `.Export(gs.As[UserService]())` 表示这个实现可以按 `UserService` 接口参与装配。

如果一个 Bean 需要承担多个角色，也可以导出多个接口。

```go
type HealthChecker interface {
	Check() error
}

type MetricsExporter interface {
	ExportMetrics() error
}

type ObservabilityAgent struct{}

func NewObservabilityAgent() *ObservabilityAgent {
	return &ObservabilityAgent{}
}

func (a *ObservabilityAgent) Check() error {
	return nil
}

func (a *ObservabilityAgent) ExportMetrics() error {
	return nil
}

func init() {
	gs.Provide(NewObservabilityAgent).
		Export(
			gs.As[HealthChecker](),
			gs.As[MetricsExporter](),
		)
}
```

上面的代码中，`ObservabilityAgent` 实现了 `HealthChecker` 和 `MetricsExporter` 两个接口。我们通过 `.Export(...)` 表示这个实现可以按这两个接口参与装配。

如果构造函数返回的就是接口类型，那么这个返回的接口类型就已经是 Bean 的装配类型了，就不需要再写同一个接口的 `Export` 了。示例如下：

```go
func NewUserService() UserService {
	return &UserServiceImpl{}
}

func init() {
	gs.Provide(NewUserService)
}
```

## 条件

理想情况下，需要哪种 Bean 就只注册哪种 Bean，不需要就不注册。但是在模块化和框架化场景下，注册代码通常提前写在模块内部，最终是否启用要由配置、环境或者已有 Bean 决定。这时就需要让当前 Bean 只在条件满足时才能激活。

Go-Spring 提供了 `Condition` 来实现条件化激活。常见的 `Condition` 有 `OnProperty`、`OnBean`、`OnProfiles` 等。本篇咱们不展开各种 Condition 的具体语义，只看在 Bean 注册时怎么使用 `Condition`。

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

上面的代码表示，只有当前 `spring.profiles.active=dev` 时，`NewDevLogger` 这个候选 Bean 才参与本次装配。条件不满足时，它不会被创建。

如果一个 Bean 可以在多个环境中启用，可以把多个 Profile 写在一起，比如 `.OnProfiles("dev,test")`。

需要注意的是，条件滥用会让装配过程变得复杂。因此，除非是框架类的场景，否则不推荐大家滥用 `Condition`。

## 显式依赖

大多数情况下，我们不需要手动声明 Bean 之间的依赖关系，结构体字段或者构造函数参数已经表达了这些关系。

但偶尔会有一些顺序依赖并不体现在字段或者参数上。这时我们可以使用 `DependsOn` 来显式表达 Bean 之间的依赖关系。示例如下：

```go
func init() {
	gs.Provide(NewDatabaseMigrator)

	gs.Provide(NewCacheWarmer).
		DependsOn(gs.BeanIDFor[*DatabaseMigrator]())
}
```

上面的代码表示，我们注册了 `NewDatabaseMigrator` 和 `NewCacheWarmer` 两个 Bean，其中后者显式依赖前者。在创建 Bean 的时候，Go-Spring 会先创建 `*DatabaseMigrator`，然后再创建 `*CacheWarmer`。在容器退出的时候，Go-Spring 会先销毁 `*CacheWarmer`，然后再销毁 `*DatabaseMigrator`。

`gs.BeanIDFor[T]()` 表示通过类型定位被依赖的 Bean。如果被依赖的是命名 Bean，可以把名字一起写进去，如 `gs.BeanIDFor[*DataSource]("master")`。

我们鼓励使用结构体字段或者构造函数参数来表达依赖关系，而不是滥用 `DependsOn`。
