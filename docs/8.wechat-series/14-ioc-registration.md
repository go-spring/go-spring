# Bean 注册 API

当项目只有几个对象时，注册 Bean 并不难；真正复杂的是组件变多之后，注册逻辑应该放在哪里、怎样按配置批量展开、怎样封装成可复用模块。

Go-Spring 提供多种注册入口。它们不是重复能力，而是分别面向不同场景：

- `gs.Provide()`：注册单个 Bean。
- `gs.Module()`：按条件执行动态注册逻辑。
- `gs.Group()`：根据配置批量注册多个同类型 Bean。
- `Configuration`：通过配置类导出多个子 Bean。
- `app.Provide()`：向当前应用实例注册 Bean，常用于测试。

## gs.Provide

`gs.Provide()` 是最基础、最常用的注册方式，通常在包的 `init()` 中调用：

```go
func init() {
	gs.Provide(NewUserService)
}
```

它会把 Bean 定义记录到全局注册表，在应用启动时统一合并。绝大多数业务组件都应该使用这种方式注册。

需要注意的是，`gs.Provide()` 必须在应用启动前调用。

## gs.Module

`gs.Module()` 用于组织一组动态注册逻辑，并支持前置条件。

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

Module 适合 Starter 和组件包：它可以读取配置、判断条件，然后批量注册 Bean。

## gs.Group

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

对应配置：

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

容器会生成名为 `serviceA`、`serviceB` 的两个 `*http.Client` Bean。

如果实例需要释放资源，可以给 `gs.Group()` 提供销毁函数。

## Configuration

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

Configuration 适合一组强相关 Bean 的集中组织。

## app.Provide

`app.Provide()` 通过 `gs.Configure()` 访问，只作用于当前应用实例：

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

这种方式常用于测试，或者需要在当前启动实例中补充 Bean 的场景。

## 如何选择

普通业务 Bean 用 `gs.Provide()`。

组件包和 Starter 用 `gs.Module()` 组织条件化注册。

同类型多实例配置用 `gs.Group()`。

同一配置类导出多个相关 Bean 时用 `Configuration`。

测试或当前应用实例专属注册用 `app.Provide()`。

## 注册只是起点

`Provide`、`Module`、`Group`、`Configuration` 和 `app.Provide` 解决的是“如何把 Bean 放进容器”。不同入口对应不同组织方式：单个对象、动态模块、批量实例、配置类和测试应用。

放进容器之后，还要判断哪些 Bean 最终生效。这个问题进入条件注册机制。
