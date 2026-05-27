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

## 模块注册

`gs.Provide()` 和 `app.Provide()` 都适合注册单个明确的 Bean。再往前走一步，很多组件包并不是提供一个固定对象，而是要先读取配置，再决定这一组能力要不要展开、展开多少个 Bean。

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

`gs.Module()` 注册的是一段模块函数。第一个参数是属性条件，通常来自 `gs.OnProperty(...)`。应用启动解析时，如果条件满足，Go-Spring 会执行模块函数，并把 `gs.BeanProvider` 和当前配置传进去。模块函数内部可以像普通注册一样调用 `r.Provide(...)`，也可以根据配置循环注册多个 Bean。

这段代码里，`redis.instances` 是否存在决定 Redis 模块是否展开；`redis.instances` 下面的配置内容决定最终会注册哪些客户端。配置影响的是模块提供的 Bean 集合，而不只是某个字段的普通取值。

因此，`gs.Module()` 适合 Starter、组件包和需要按配置展开注册的能力。它不适合承载普通业务对象的零散注册，否则注册入口会变得集中但不清楚：读代码时看不出某个 Bean 到底属于哪个业务包，也看不出这个模块真正表达的能力边界。

## 同构多实例

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

`gs.Group()` 的配置路径必须写成 `${...}` 形式。启动解析时，Go-Spring 会读取 `${http.clients}`，把它绑定成 `map[string]HTTPClientConfig`。其中 `serviceA` 和 `serviceB` 会成为 Bean 名称，两组配置分别传给 `NewHTTPClient`，最终得到两个独立的 `*HTTPClient` Bean。

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

从实现语义上看，`gs.Group()` 可以理解为 Go-Spring 帮我们写好的一类特殊 `gs.Module()`：配置项存在时展开模块，把配置字典的 key 作为 Bean 名称，把 value 作为构造函数参数。

也正因为这样，`gs.Group()` 的边界很窄：配置必须是一个字典，同一个构造函数负责创建所有实例。如果每个实例的创建方式不一致，或者还要注册其他配套 Bean，就应该退回 `gs.Module()`，把完整的注册逻辑写出来。

## 配置类导出

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

默认情况下，Go-Spring 会扫描公开方法里匹配正则 `New.*` 的方法。上面的 `NewDataSource` 和 `NewUserRepository` 都会被识别为构造方法。它们和普通构造函数一样，可以只返回一个对象，也可以返回 `(T, error)`。

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

## 注册入口选择

这些入口的差异，本质上不是“哪个 API 更高级”，而是注册边界不同。

| 场景 | 注册入口 | 语义边界 |
|------|----------|----------|
| 单个稳定 Bean，随包导入提供 | `gs.Provide()` | 包级候选 |
| 当前启动实例专属 Bean | `app.Provide()` | 应用实例 |
| 按配置展开一组注册动作 | `gs.Module()` | 模块能力 |
| 配置字典生成同类型多实例 | `gs.Group()` | 同构多实例 |
| 同一配置类导出多个相关 Bean | `Configuration` | 共享配置和创建上下文 |

普通业务对象优先用 `gs.Provide()` 或 `app.Provide()`，让对象跟着包或当前启动实例走。只有当注册动作本身属于组件能力，或者需要根据配置决定注册集合时，才把它提升到 `gs.Module()`、`gs.Group()` 或 `Configuration`。这样读代码时，看到注册入口就能大致判断它的作用范围，而不是把所有 Bean 都堆到一个看似统一、实际边界模糊的地方。
