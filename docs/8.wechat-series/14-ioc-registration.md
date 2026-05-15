# Go-Spring 实战第 14 课：Bean 注册入口：不同组织边界下 Provide、Module、Group 和 app.Provide 怎么选

只有几个对象时，注册 Bean 通常就是几行 `gs.Provide()`。组件变多以后，真正难维护的不是调用哪个函数，而是注册逻辑应该放在哪个边界里。

普通业务对象、Starter 模块、多实例配置、配置类导出的子 Bean、测试里临时替换的对象，它们都可以进入 Go-Spring 容器，但不应该挤在同一种注册入口里。入口选错了，后续配置、条件和测试隔离都会变得别扭。

所以，注册入口不按 API 清单展开，而是按组织边界来判断。边界清楚以后，`Provide`、`Module`、`Group`、`Configuration` 和 `app.Provide` 的选择就会自然很多。

## gs.Provide 适合启动前已确定的单个业务 Bean

`gs.Provide()` 是最基础、最常用的注册方式，通常在包的 `init()` 中调用。它适合启动前已经确定的单个业务 Bean。

```go
func init() {
	gs.Provide(NewUserService)
}
```

这条注册语句会把 Bean 定义记录到 Go-Spring 全局注册表。应用启动时，Go-Spring 会把全局注册表里的候选定义合并进当前容器，再统一解析依赖图。

因为 Go-Spring 的依赖图是在启动期一次性解析的，所以 `gs.Provide()` 的语义是“启动前提供候选定义”。如果应用进入运行阶段以后再追加全局 Bean，就会破坏这套启动期模型。

## gs.Module 把依赖配置的注册逻辑放进模块边界

有些注册动作本身依赖配置或环境。比如 Redis Starter 需要先读取配置，再按配置内容注册一个或多个客户端。这个场景用一串散落的 `Provide` 不容易表达，`gs.Module()` 更适合承接这种模块边界。

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

Module 可以读取配置、判断条件，然后批量注册 Bean。它更适合 Starter 和组件包，因为注册逻辑本身就是模块能力的一部分。

## gs.Group 用配置字典展开同类型多实例

如果配置天然是一组同类型实例，`gs.Group()` 可以直接按配置字典展开 Bean。下面的 `http.clients` 中每个字典项都会生成一个 `*http.Client` Bean。

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

配置里的 key 会成为 Bean 名称。

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

启动解析后，Go-Spring 容器会得到名为 `serviceA` 和 `serviceB` 的两个 `*http.Client` Bean。如果实例需要释放资源，可以给 `gs.Group()` 提供销毁函数。

`Group` 的边界是“同一个构造函数 + 多组配置”。如果每个实例的创建逻辑差异很大，放回 Module 或显式 Provide 会更清楚。

## Configuration 适合一组强相关 Bean 的集中导出

有些 Bean 不是简单的单点注册，而是一组强相关对象。它们共享一段配置，也有固定的创建关系。`Configuration` 模式适合把这些创建方法收在一个配置类里，再由 Go-Spring 扫描公开方法导出子 Bean。

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

需要调整扫描范围时，可以显式声明规则。

```go
func init() {
	gs.Provide(new(DatabaseConfiguration)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"},
			Excludes: []string{".*Internal$"},
		})
}
```

Configuration 的价值在于把相关创建逻辑集中到同一个类型里，同时让返回值仍然作为独立 Bean 参与依赖注入和生命周期管理。

## app.Provide 只影响当前应用实例

`app.Provide()` 通过 `gs.Configure()` 访问，只作用于当前应用实例。它适合测试、局部替换和当前启动专属的补充 Bean。

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

这类注册不会污染全局注册表。测试里可以给当前容器补一个 mock，或者覆盖某个启动实例的局部依赖，而不影响其他测试和其他启动流程。

## 注册入口按组织边界选择

普通业务 Bean 使用 `gs.Provide()`，因为它的定义在启动前已经确定。注册动作依赖配置或环境时，用 `gs.Module()` 把逻辑收进模块边界。同类型多实例来自配置字典时，用 `gs.Group()` 展开。多个相关 Bean 由同一个配置类导出时，用 `Configuration`。测试或当前应用实例专属注册，则放到 `app.Provide()`。

这个选择标准不是 API 偏好，而是 Bean 定义属于哪个组织边界。边界选对以后，条件、配置和测试替换都会跟着落在合适的位置。

## 注册完成后还要经过条件裁剪

`Provide`、`Module`、`Group`、`Configuration` 和 `app.Provide` 解决的是“如何把 Bean 定义放进容器”。定义进入容器后，Go-Spring 还会根据配置、环境和已有依赖判断哪些 Bean 在本次启动中生效。

这一步就是条件注册要处理的问题。它不改变注册入口的职责，而是在解析阶段裁剪候选对象图。
