# Go-Spring 实战第 14 课 —— Bean 注册函数：Provide、Module、Group 以及 Configuration

前面的文章中我们都是使用 `gs.provide()` 来注册 Bean 的。实际上 Go-Spring 还提供了其他方法可以注册 Bean。本文咱们就来详细看一下。

## `gs.Provide`

`gs.Provide()` 是最常用的注册 bean 的入口。它的作用是把一个对象或者构造函数交给 Go-Spring 容器。它的用法也最简单，通常在 Go `init` 函数里调用。

看个例子。

```go
func init() {
	gs.Provide(NewUserService)
}
```

`gs.Provide()` 会把 Bean 的元信息记录到 Go-Spring 的全局注册表。每次新建一个 IoC 容器时，Go-Spring 会将全局注册表里的 bean 元信息复制一份，和直接注册到 IoC 容器里的 bean 元信息进行合并，然后再统一处理。

> 什么是全局注册表？通常来说，应用启动只需要一个 IoC 容器，但是在测试模式下，每个测试函数都希望使用只属于自己的独立的 IoC 容器，这样就会多次创建和启动 IoC 容器。因此，为了实现数据隔离，Go-Spring 提出了全局注册表的概念。

一般来说，`gs.provide` 只需要在 app 启动前执行就可以了，但是为了规范和统一，而且从语义上来讲，我们更建议在 `init()` 函数里调用。

## `app.Provide`

通常情况下，我们只需要将 bean 元信息注册到全局注册表。但是，有些情况下我们希望将 bean 直接注册到 IoC 容器，尤其是在单测需要启动多个 IoC 容器的情况下。每个测试可能都需要自己的 bean 配置，这时候定制化的 bean 就需要直接注册到 ioc 容器里了。

`app.Provide()` 可以将一个 bean 直接注册到 ioc 容器里。另外，`app.Provide()` 需要配合 `gs.Configure()` 回调函数一起使用。示例如下：

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

如果 `app.Provide()` 注册的 bean 和其他方式注册的 bean 同类型同名，那么不会自动覆盖旧定义，而是直接报错。如果我们需要使用覆盖语义，可以使用名称或者条件等来激活不同的 bean。

## `gs.Module`

通常情况下，我们只需要注册单个 bean 就好，但有时候，尤其是我们打算应用模块化理念时，会遇到同时注册多个 bean 的情况。当然，我们不是说使用一个函数同时注册多个 bean，那样每个 bean 的元信息是没法控制的。更合适的说法是，我们希望注册一组 bean。

`gs.Module()` 就是为了满足这种需求而设计的。它接受一个回调函数，我们可以在回调函数里面根据配置信息注册多个 bean。这些 bean 可以是相同类型，也可以不是。

`gs.Module()` 是实现 Go-Spring Starter 机制的基石。像 Redis Starter、JPA Starter 等组件包，都可以使用 `gs.Module()` 来实现。

代码如下：

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

`gs.Module()` 同样将回调函数注册到全局注册表。然后多个 IoC 容器启动时，各自从全局注册表复制一份回调函数列表到自己内部，这样也是为了实现多 ioc 容器时的数据隔离。

`gs.Module()` 的第一个参数是属性条件，通常使用 `gs.OnProperty(...)`，因为大多数情况下我们是根据配置来控制整个模块是否生效的。

`gs.Module()` 的第二个参数是模块函数，它接受 `gs.BeanProvider` 和 `flatten.Storage` 作为参数，并返回一个错误。`gs.BeanProvider` 用于向 ioc 容器直接注册 Bean。`flatten.Storage` 用于读取配置。

ioc 容器启动时，会根据 module 的条件来判断是否执行模块函数。如果条件不满足，那么模块函数就不会被调用，也就是一组 bean 集体不被注册。

在上面的代码里，`redis.instances` 这个配置是否存在决定了 Redis 模块是否激活。这在。。。情况下非常合适。

## `gs.Group`

`gs.Group()` 是 `gs.Module()` 的一个特殊版本，用来控制一组相同类型 bean 的注册。它接受一个配置项，这个配置项的子 key 是创建的 bean 的名称。一个子 key 对应一个 bean。

示例如下：

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

在上面的代码中，`gs.Group()` 会根据 `${http.clients}` 配置项的内容，展开出多个 Bean。对于上面的代码，我们可以配合使用下面的配置。

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

对于上面的配置，我们会注册 `serviceA` 和 `serviceB` 两个 bean 实例。

在这种情况下，依赖方通常需要按照名称区分需要注入的 bean 实例。

```go
type ReportService struct {
	Client *HTTPClient `autowire:"serviceA"`
}
```

如果每个实例都需要释放资源，那么可以给 `gs.Group()` 传入销毁函数。这个销毁函数会绑定到每一个由配置项展开出来的 Bean 上。

```go
func init() {
	gs.Group("${http.clients}", NewHTTPClient, (*HTTPClient).Close)
}
```

`gs.Group()` 在创建多实例 client 场景时非常有用。

## `Configuration`

`Configuration` 是一种非常 java 的注册 bean 的方式，它可以将 bean 的方法导出成 bean。这样，我们可以实现另一种封闭式的 bean 注册风格。

看个例子。

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

在上面的示例中，`DatabaseConfiguration` 本身先作为普通 Bean 进行注册。它可以接收配置绑定，也可以接收其他注入。但是我们通过 `.Configuration()` 调用告诉 Go-Spring：这个 bean 是一个特殊的配置 bean，它的方法也可以注册成 Bean。

默认情况下，Go-Spring 会扫描公开方法里匹配正则 `New.*` 的方法。在上面的例子中，`NewDataSource` 和 `NewUserRepository` 都会被识别为构造方法，他们分别会创建 `*DataSource` 和 `*UserRepository` 类型的 bean。同时，对于 `NewUserRepository` 这个构造函数而言，通过 `NewDataSource` 创建的 `*DataSource` bean 就是它的参数。

和普通构造函数一样，这些被识别为子 bean 的方法可以只返回一个对象 `T`，也可以返回 `(T, error)`。

另外，配置类方法导出的子 Bean 会得到自动生成的名称，形式是“配置 Bean 名称 + 方法名”，中间用下划线连接，比如 `DatabaseConfiguration_NewDataSource`。

如果需要调整扫描范围，我们可以显式地声明包含和排除规则。示例如下：

```go
func init() {
	gs.Provide(new(DatabaseConfiguration)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"},
			Excludes: []string{".*Internal$"},
		})
}
```

`Configuration` 非常特殊，我们承认它的独特价值，但是也要警惕它的复杂性和隐蔽性。

本质上，`Configuration` 的注册方式和下面这种只使用 `gs.Provide` 的方式等价。代码如下：

todo (补充代码示例，你能明白的)

## 如何选择

我们看到 Go-Spring 提供了多种 api 来满足不同的需求，我们一定要优先使用最简单的方式来实现，当我们觉得其他方式能简化代码、简化表达的时候再使用。
