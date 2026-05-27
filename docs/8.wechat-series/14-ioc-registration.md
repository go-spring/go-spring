# Go-Spring 实战第 14 课 —— Bean 注册函数：Provide、Module、Group 与 Configuration

前面两篇已经讲过 Bean 可以从结构体指针、构造函数和函数本身注册进来，也讲过注册语句后面还能附加名称、接口导出、生命周期和条件。到这里还有一个问题没有展开：这些注册语句应该放在哪个入口里？

最常见的入口当然是 `gs.Provide()`。但真实项目里不只有单个 Bean 的注册。测试或当前启动实例会需要局部 Bean；Starter 会根据配置批量提供 Bean；有些同类型实例天然来自一组配置；还有一些相关对象更适合收在同一个配置类里统一导出。这些场景对应的就是 `app.Provide()`、`gs.Module()`、`gs.Group()` 和 `Configuration`。

所以，注册函数的选择不是 API 偏好，而是一个边界问题：**这个 Bean 定义属于包级能力、当前应用实例、配置驱动的模块、配置字典，还是某个配置类。**

## 包级定义

如果一个 Bean 的来源在启动前已经确定，注册逻辑也不依赖当前应用实例的配置，那么它适合用 `gs.Provide()` 放进包级注册。

最小写法就是在 `init()` 中注册一个构造函数。

```go
func init() {
	gs.Provide(NewUserService)
}
```

`gs.Provide()` 会把 Bean 定义记录到 Go-Spring 的全局注册表。应用启动时，当前容器会合并这些全局定义，再统一完成条件判断、参数绑定、实例创建、注入和生命周期处理。

这也是为什么 `gs.Provide()` 必须在包初始化阶段调用。进入 `gs.Configure()` 之后，应用已经开始准备当前启动实例，再追加全局定义就会让注册时机变得不可预测。Go-Spring 会直接 `panic`，提示 `gs.Provide can only be called in init function`。

因此，普通业务组件、框架基础组件、包级默认实现，都适合放在 `gs.Provide()`。它表达的是“只要这个包被导入，就向 Go-Spring 提供这类候选 Bean”。

这里要注意，`gs.Provide()` 解决的是注册入口问题，不是 Bean 类型问题。它既可以接收已经创建好的结构体指针，也可以接收构造函数；这些内容已经在第 12 课讲过，本篇不再重复展开。

## 应用实例

有些 Bean 不应该进入全局注册表，只应该影响当前这一次启动。比如测试里临时提供的替身实现，命令行工具里某个专用组件，或者同一进程中不同启动实例需要的局部对象。这类注册应该放进 `app.Provide()`。

`app.Provide()` 出现在 `gs.Configure()` 的回调里。

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

这条注册语句不会写入全局注册表，只会加入当前应用实例的容器。也就是说，它和包级 `gs.Provide()` 一样会参与本次启动的统一解析，但作用范围只限于当前 `Run()` 或测试流程。

这个边界在测试里尤其重要。测试可以通过 `app.Provide()` 提供替身 Bean，而不污染其他测试和其他启动流程。不过，如果全局里已经有同类型同名 Bean，单纯再注册一个局部 Bean 并不会自动覆盖旧定义；两个定义都生效时会触发重复 Bean 检查。需要替换默认实现时，通常要配合名称、条件或 `OnMissingBean` 这类元信息，把默认实现和测试替身的关系写清楚。

所以，`app.Provide()` 不是运行期动态注册入口。它仍然发生在启动配置阶段，只是注册边界从“包级全局”缩小到了“当前应用实例”。

## 模块展开

`gs.Provide()` 和 `app.Provide()` 都适合注册单个明确的 Bean。问题是，很多组件包并不是提供一个固定对象，而是要先读取配置，再决定注册哪些 Bean。

比如 Redis Starter 可能需要读取一组实例配置，然后为每个实例注册一个客户端。这个注册动作本身就是模块能力的一部分，更适合放进 `gs.Module()`。

```go
func RedisModule(r gs.BeanProvider, p flatten.Storage) error {
	var instances map[string]RedisConfig
	if err := conf.Bind(p, &instances, "${redis.instances}"); err != nil {
		return err
	}

	for name, cfg := range instances {
		r.Provide(NewRedisClient, gs.ValueArg(cfg)).Name(name)
	}
	return nil
}

func init() {
	gs.Module(
		gs.OnProperty("redis.instances"),
		RedisModule,
	)
}
```

`gs.Module()` 注册的是一段模块函数。应用启动解析时，如果模块条件满足，Go-Spring 会执行这个函数，并把 `gs.BeanProvider` 和当前配置传进去。模块函数内部可以像普通注册一样调用 `r.Provide(...)`，也可以根据配置循环注册多个 Bean。

这段代码里，`redis.instances` 是否存在决定 Redis 模块是否展开；`redis.instances` 下面的配置内容决定最终会注册哪些客户端。配置影响的是模块提供的 Bean 集合，而不只是某个字段的普通取值。

因此，`gs.Module()` 适合 Starter、组件包和需要按配置展开注册的能力。它不适合承载普通业务对象的零散注册，否则注册入口会变得集中但不清楚：读代码时看不出某个 Bean 到底属于哪个业务包，也看不出这个模块真正表达的能力边界。

## 配置字典

有些模块虽然也会按配置注册多个 Bean，但模式非常固定：同一个构造函数，对应配置字典里的多组参数，每个 key 生成一个 Bean 名称。这个场景可以从 `gs.Module()` 进一步收敛成 `gs.Group()`。

先看一个 HTTP 客户端的例子。

```go
type HTTPClientConfig struct {
	BaseURL string        `value:"${baseURL}"`
	Timeout time.Duration `value:"${timeout:=30s}"`
}

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(c HTTPClientConfig) (*HTTPClient, error) {
	return &HTTPClient{
		baseURL: c.BaseURL,
		client:  &http.Client{Timeout: c.Timeout},
	}, nil
}

func (c *HTTPClient) Close() error {
	return nil
}

func init() {
	gs.Group("${http.clients}", NewHTTPClient, nil)
}
```

对应的配置可以写成一个字典。

```yaml
http:
  clients:
    serviceA:
      baseURL: "http://a.example.com"
      timeout: 30s
    serviceB:
      baseURL: "http://b.example.com"
      timeout: 60s
```

启动解析时，`gs.Group()` 会读取 `${http.clients}`，把它绑定成 `map[string]HTTPClientConfig`。其中 `serviceA` 和 `serviceB` 会成为 Bean 名称，两组配置分别传给 `NewHTTPClient`，最终得到两个独立的 `*HTTPClient` Bean。

依赖方需要区分实例时，可以按名称注入。

```go
type ReportService struct {
	Client *HTTPClient `autowire:"serviceA"`
}
```

如果每个实例都需要释放资源，可以给 `gs.Group()` 传入销毁函数。这个销毁函数会绑定到每一个由配置项展开出来的 Bean 上。

```go
func init() {
	gs.Group("${http.clients}", NewHTTPClient, (*HTTPClient).Close)
}
```

`gs.Group()` 的边界很窄：配置必须是一个字典，字典 key 是 Bean 名称，字典 value 是构造函数参数，同一个构造函数负责创建所有实例。如果每个实例的创建方式不一致，或者还要注册其他配套 Bean，就应该退回 `gs.Module()`，把完整的注册逻辑写出来。

## 配置类

还有一种情况不是“同类型多实例”，而是“一组相关 Bean 共享同一段配置和创建上下文”。这时可以把它们收进一个配置类，再通过 `Configuration` 导出多个方法 Bean。

比如数据库相关对象通常共享一组数据库配置。

```go
type DatabaseConfiguration struct {
	MaxOpenConns int `value:"${db.max-open-conns:=10}"`
}

func (c *DatabaseConfiguration) NewDataSource() *DataSource {
	return NewDataSource(c.MaxOpenConns)
}

func (c *DatabaseConfiguration) NewUserRepository(ds *DataSource) *UserRepository {
	return NewUserRepository(ds)
}

func init() {
	gs.Provide(new(DatabaseConfiguration)).Configuration()
}
```

`DatabaseConfiguration` 本身先作为普通 Bean 注册。它可以接收配置绑定，也可以接收其他注入。随后 `.Configuration()` 告诉 Go-Spring：启动解析时扫描这个配置类的方法，把符合规则的方法返回值也注册成 Bean。

默认情况下，Go-Spring 会扫描公开方法里匹配 `New.*` 的方法。上面的 `NewDataSource` 和 `NewUserRepository` 都会被识别为构造方法。它们和普通构造函数一样，可以只返回一个对象，也可以返回 `(T, error)`。

如果需要调整扫描范围，可以显式声明包含和排除规则。

```go
func init() {
	gs.Provide(new(DatabaseConfiguration)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"},
			Excludes: []string{".*Internal$"},
		})
}
```

`Configuration` 的价值在于组织相关创建逻辑。配置类承接共享配置和共同上下文，方法返回值仍然作为独立 Bean 参与后续装配和生命周期处理。

不过它不应该被当成“大型注册中心”。如果一组 Bean 没有共同配置，也没有稳定的创建关系，只是因为文件放在一起方便，就不适合塞进同一个配置类。那样会让注册位置变集中，但语义边界反而变弱。

还有一个细节：配置类方法导出的 Bean 会得到自动生成的名称，形式是“配置 Bean 名称 + 方法名”，中间用下划线连接，比如 `DatabaseConfiguration_NewDataSource`。大多数情况下我们按类型注入，不需要关心这个名称；如果确实要按名称选择，就要把这个命名规则考虑进去。

## 注册入口怎么选

这几种注册入口最终都会产生 Bean 定义，区别在于它们表达的归属边界不同。

| 场景 | 注册入口 | 语义边界 |
|------|----------|----------|
| 单个稳定 Bean，随包导入提供 | `gs.Provide()` | 包级能力 |
| 当前启动实例专属 Bean | `app.Provide()` | 应用实例 |
| 按配置或环境展开一组注册动作 | `gs.Module()` | 模块能力 |
| 配置字典生成同类型多实例 | `gs.Group()` | 同构多实例 |
| 同一配置类导出多个相关 Bean | `Configuration` | 共享配置和创建上下文 |

选择入口时，可以先问两个问题。

第一，这个注册动作是不是只声明一个已经确定的 Bean？如果是，再看它属于包级能力还是当前应用实例，分别使用 `gs.Provide()` 和 `app.Provide()`。

第二，这个注册动作是否会在启动解析时展开出更多 Bean？如果展开逻辑很自由，用 `gs.Module()`；如果只是配置字典到同类型实例的映射，用 `gs.Group()`；如果是一组方法共享同一个配置类，用 `Configuration`。

边界选对以后，配置、条件、测试替身和生命周期都会落在更自然的位置。注册函数就不只是“把对象交给容器”的入口，也是在代码里说明 Bean 定义归属关系的地方。
