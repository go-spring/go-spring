# Go-Spring 实战第 14 课：Bean 注册入口：Provide、Module、Group、Configuration 怎么选

在 Go-Spring 项目里，当只有几个对象时，注册 Bean 并不难；真正容易变复杂的是组件变多之后，注册逻辑应该放在哪里、怎样按配置批量展开、怎样封装成可复用模块。

所以这一篇我们不只看“怎么注册”，还要看不同注册入口背后的组织边界。Go-Spring 提供了多种注册入口，它们不是重复能力，而是分别面向不同场景。先把几个入口摆出来看看：

- `gs.Provide()`：注册单个 Bean。
- `gs.Module()`：按条件执行动态注册逻辑。
- `gs.Group()`：根据配置批量注册多个同类型 Bean。
- `Configuration`：通过配置类导出多个子 Bean。
- `app.Provide()`：向当前应用实例注册 Bean，常用来测试。

这些入口不用靠记忆硬背。先看注册逻辑属于单个对象、动态模块、多实例配置、配置类组织，还是当前应用实例专属，就能选到合适的 API。

## gs.Provide：注册普通业务 Bean

`gs.Provide()` 是最基础、最常用的注册方式，通常在包的 `init()` 中调用：

```go
func init() {
	gs.Provide(NewUserService)
}
```

它会把 Bean 定义记录到 Go-Spring 全局注册表，在应用启动时统一合并。绝大多数业务组件都应该使用这种方式注册。

这里要注意，`gs.Provide()` 必须在应用启动前调用。如果启动之后再追加全局 Bean，就会破坏容器启动期解析模型。

## gs.Module：按条件执行动态注册

`gs.Module()` 用来组织一组动态注册逻辑，并支持前置条件。

```go
func RedisModule(r gs.BeanProvider, p flatten.Storage) error {
	var m map[string]RedisConfig
	if err := conf.Bind(p, &m); err != nil {
		return err
	}

	for name, config := range m {
		r.Provide(NewRedisClient, gs.ValueArg(config)).Name(name)
	}
	return nil
}

func init() {
	gs.Module(
		gs.OnProperty("enable.redis").HavingValue("true"),
		RedisModule,
	)
}
```

Module 更适合 Starter 和组件包：它可以读取配置、判断条件，然后批量注册 Bean。也就是说，当注册动作本身依赖配置或环境时，我们就不应该硬塞进一串 `Provide` 调用里，而应该把这段逻辑放进 Module。

## gs.Group：按配置批量生成多实例

`gs.Group()` 是面向多实例配置的便捷注册方式。它会从配置字典中读取多个条目，并为每个条目创建一个 Bean。

```go
type HTTPClientConfig struct {
	BaseURL string        `value:"${baseURL}"`
	Timeout time.Duration `value:"${timeout:=30s}"`
}

func NewHTTPClient(c HTTPClientConfig) (*http.Client, error) {
	return &http.Client{Timeout: c.Timeout}, nil
}

func init() {
	gs.Group("${http.clients}", NewHTTPClient, nil)
}
```

配置中每个字典项都会生成一个同类型 Bean，字典 key 会成为 Bean 名称：

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

接着，Go-Spring 容器会生成名为 `serviceA`、`serviceB` 的两个 `*http.Client` Bean。

如果实例需要释放资源，可以给 `gs.Group()` 提供销毁函数。

## Configuration：从配置类导出多个 Bean

`Configuration` 模式允许一个配置类导出多个子 Bean：

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

容器会扫描配置类公开方法，把符合规则的方法返回值注册为独立 Bean。默认包含方法名匹配 `New.*` 的方法。

可以自定义扫描规则：

```go
func init() {
	gs.Provide(new(DatabaseConfiguration)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"},
			Excludes: []string{".*Internal$"},
		})
}
```

Configuration 适合一组强相关 Bean 的集中组织。这样相关创建逻辑放在同一个类型里，同时返回值仍然作为独立 Bean 管理。

## app.Provide：只影响当前应用实例

`app.Provide()` 通过 `gs.Configure()` 访问，只作用于当前应用实例：

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

这种方式常用在测试里，或者需要在当前启动实例中补充 Bean 的场景。它不会污染全局注册表，所以更适合临时替换和局部组装。换句话说，它解决的是“当前这次启动”的注册问题。

## 按组织边界选择注册入口

可以粗略按这个规则选：普通业务 Bean 用 `gs.Provide()`。

组件包和 Starter 用 `gs.Module()` 组织条件化注册。

同类型多实例配置用 `gs.Group()`。

同一配置类导出多个相关 Bean 时用 `Configuration`。

测试或当前应用实例专属注册用 `app.Provide()`。

## 注册之后还要判断是否生效

`Provide`、`Module`、`Group`、`Configuration` 和 `app.Provide` 解决的是“如何把 Bean 放进容器”。不同入口对应不同组织方式：单个对象、动态模块、批量实例、配置类和测试应用。

Bean 放进容器只是第一步，真正启动时还要根据配置、环境和已有依赖判断哪些定义应该生效，这就是条件注册要处理的问题。
