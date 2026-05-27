# Go-Spring 实战第 13 课 —— Bean 元信息：注册语句还能说明哪些容器语义

上一篇讲的是 Bean 注册类型：结构体指针、构造函数和函数。它们回答的是“这个 Bean 从哪里来，Go-Spring 要不要负责创建它”。但在真实项目里，只知道对象怎么来还不够。

同一个类型可能有多个实例，需要名称区分；一个实现可能要按接口注入，需要显式导出；对象创建完成后可能要检查连接，退出时还要释放资源；某些 Bean 只在配置或 Profile 命中时生效；少数对象没有直接注入关系，但初始化顺序仍然要被约束。也就是说，注册 Bean 时还会附加一层元信息，用来说明这个对象在 Go-Spring 容器里怎样参与装配。

这层元信息不是业务状态，也不是给人看的注释。它会在启动期被 Go-Spring 读取，并影响依赖匹配、条件裁剪、初始化顺序、接口注入和创建入口。

```go
func init() {
	gs.Provide(NewUserService).
		Name("default").
		Export(gs.As[UserService]()).
		Condition(gs.OnProperty("user.service.enabled")).
		Init(CheckUserService).
		Destroy(CloseUserService)
}
```

这条注册语句里，构造函数 `NewUserService` 说明对象怎么创建；后面的链式调用说明这个 Bean 在容器里的身份、可见接口、生效条件和生命周期动作。把这些信息放在注册处，Go-Spring 才能在真正创建 Bean 之前统一解析它们。

| 元信息 | 解决的问题 | 主要生效阶段 |
|--------|------------|--------------|
| `Name` | 同类型多实例如何区分 | 依赖匹配 |
| `Export` | 具体实现按哪些接口暴露 | 依赖匹配 |
| `Condition` / `OnProfiles` | 当前启动是否启用这个 Bean | 解析裁剪 |
| `DependsOn` | 没有注入关系时如何补充顺序约束 | 初始化和销毁 |
| `Init` / `Destroy` | 对象创建后和退出前要做什么 | 生命周期 |
| `app.Root` | Bean 从哪里开始创建 | 创建入口 |

## Bean 名称

Go-Spring 使用“类型 + 名称”标识 Bean。类型来自注册对象本身，或者构造函数返回值；名称没有显式指定时，会使用默认名称。

只有一个实例时，默认名称通常不会进入业务代码。问题出现在同类型多实例场景：类型只能说明“我要一个 `*DataSource`”，不能说明“我要主库还是从库”。这时名称就不再是展示信息，而是注册方和依赖方之间的选择协议。

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewReplicaDataSource).Name("replica")
}

type UserRepository struct {
	DS *DataSource `autowire:"replica"`
}
```

构造函数注入也使用同样语义，只是名称放在注册阶段的 `TagArg` 里。

```go
func NewUserRepository(ds *DataSource) *UserRepository {
	return &UserRepository{DS: ds}
}

func init() {
	gs.Provide(NewUserRepository, gs.TagArg("replica"))
}
```

因此，Bean 名称要稳定、可读，并且能表达实例差异。`master`、`replica`、`readonly` 这类名称说明的是装配意图；随机缩写或临时编号会让后续排查多实例注入问题变得困难。

## 接口导出

Go 的接口是隐式实现的。一个结构体可能实现多个接口，也可能因为方法集合刚好匹配而无意中满足某个接口。如果容器自动把结构体导出为所有可匹配接口，依赖结果会变得难以推断。

所以 Go-Spring 要求接口导出在注册处显式声明。

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

`.Export(gs.As[UserService]())` 表示这个具体实现除了按 `*UserServiceImpl` 参与装配，还可以按 `UserService` 接口参与装配。依赖方声明具体类型，就拿到具体实现；依赖方声明接口，就走接口导出关系。

如果一个实现需要承担多个框架角色，可以多次导出接口。

```go
func init() {
	gs.Provide(NewJob).
		Export(gs.As[Job]()).
		Export(gs.As[gs.Runner]())
}
```

如果构造函数本身就返回接口类型，就不需要额外 `Export`。

```go
func NewUserService() UserService {
	return &UserServiceImpl{}
}
```

这两种写法的共同点是：接口关系由注册代码明确给出，而不是让 Go-Spring 猜测结构体“可能应该”暴露哪些接口。

## 生命周期动作

构造函数负责创建对象，但有些动作更适合放到容器生命周期里。比如对象完成依赖注入后检查连接，应用退出前关闭资源，或者把第三方组件接入统一的启动和关闭顺序。

Go-Spring 通过 `Init` 和 `Destroy` 为 Bean 附加生命周期动作。

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

初始化回调在 Bean 创建并完成依赖注入后执行。初始化失败会让容器启动失败，因为这时继续运行只会得到一组不可用的 Bean。销毁回调在容器退出时执行，用来关闭连接、停止后台任务或刷写缓冲区；如果销毁失败，Go-Spring 会记录错误，但退出流程仍然要继续收束其他资源。

如果初始化和销毁本来就是对象自己的行为，也可以在类型上定义方法，再在注册时声明方法名。

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

外部函数适合在注册处补充集成动作，方法名方式适合对象自带生命周期。无论哪种写法，进入的都是同一条容器生命周期链路。

这里还有一个边界：能在构造函数里完成的必填参数校验和对象创建失败，通常应该留在构造函数里表达；依赖注入完成后才具备条件的检查、启动前探测和退出清理，才更适合放进 `Init` 和 `Destroy`。

## 装配条件

Bean 注册进容器，不代表本次启动一定会使用它。可选组件、默认实现、环境实现和 Starter 扩展，都经常需要在启动解析阶段判断是否生效。

条件也是 Bean 元信息。它附加在 Bean 定义上，告诉 Go-Spring：这个候选定义只有满足条件时才参与本次装配。

```go
func init() {
	gs.Provide(NewAuditLogger).Condition(
		gs.OnProperty("audit.enabled").HavingValue("true"),
	)
}
```

Go-Spring 会在解析阶段计算条件。条件满足，Bean 定义继续参与依赖匹配、创建和初始化；条件不满足，Bean 定义会被裁剪，后续也不会执行生命周期回调。

按 Profile 启用 Bean 时，可以使用更直接的 `.OnProfiles()`。

```go
func init() {
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

`.OnProfiles("dev")` 本质上仍然是条件元信息，但它把语义固定在 Profile 维度上。读注册代码时，能直接看出这个 Bean 属于开发环境实现，而不是普通配置开关。

因此，条件解决的是启动期 Bean 裁剪，不是业务运行期的 `if` 分支。条件越接近装配语义，启动结果越容易预测；业务状态、订单类型、用户策略这类运行期判断，仍然应该留在业务代码里。

## 显式依赖

大多数初始化顺序可以通过注入关系推断。如果 `Service` 注入了 `Repository`，Go-Spring 会先准备好 `Repository`，再创建 `Service`。退出时顺序相反，先销毁 `Service`，再销毁 `Repository`。

但有些对象没有直接注入关系，却仍然需要顺序约束。比如缓存预热任务并不持有迁移器对象，但它必须等数据库迁移完成后再执行。这种情况下可以使用 `DependsOn`。

```go
func init() {
	gs.Provide(NewDatabaseMigrator).Name("main")

	gs.Provide(NewCacheWarmer).
		DependsOn(gs.BeanIDFor[*DatabaseMigrator]("main"))
}
```

这表示 `NewCacheWarmer` 对应的 Bean 在初始化顺序上依赖名为 `main` 的 `*DatabaseMigrator`。销毁时，Go-Spring 会按相反顺序处理。

`DependsOn` 只补充顺序约束，不应该替代真正的依赖注入。如果对象运行时确实要调用另一个对象，就应该把它写成字段或构造函数参数；否则注册语句只能说明启动顺序，业务代码里却看不出真实依赖。

## 创建入口

Go-Spring 默认按需创建 Bean。注册只是提供候选定义，容器不会因为某个 Bean 已经注册，就无条件把它实例化。真正创建对象时，Go-Spring 会从创建入口开始，逐层创建入口直接或间接依赖到的 Bean。

应用里常见的入口来自以 `gs.Runner` 或 `gs.Server` 身份参与装配的 Bean。这类对象代表启动任务或长期服务，会被应用生命周期收集并驱动执行。如果一个应用没有这类入口，但仍希望某个对象进入容器初始化流程，可以使用 `app.Root()` 显式指定创建入口。

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

`app.Root()` 的语义是指定从哪个 Bean 开始创建，而不是强制创建所有已注册 Bean。上面的例子里，Go-Spring 会创建 `AppEntry`，再沿着它声明的依赖创建 `UserService` 以及更深层依赖。没有被入口直接或间接依赖到的候选定义，即使注册成功，也不会因为注册本身被实例化。

所以排查“某个 Bean 为什么没有执行初始化”时，要同时看两个问题：它是否通过了条件裁剪，以及它是否会被创建入口带起来。前者决定它有没有资格参与本次装配，后者决定它会不会真的被创建。

## Bean 元信息

Bean 的类型说明它是什么，构造函数说明它怎样创建，元信息说明它在 Go-Spring 容器里怎样被使用。

名称让同类型多实例可区分，接口导出让依赖边界可见，生命周期动作把启动和退出纳入统一顺序，条件决定候选定义是否参与本次启动，显式依赖补充纯顺序约束，root 入口决定从哪个 Bean 开始创建。

把这些信息放在注册语句里，Go-Spring 就能在启动期统一解析本次装配。运行期业务代码面对的是已经创建、已经注入、已经裁剪完成的普通 Go 对象，而不是一组需要临时判断和动态查找的容器规则。
