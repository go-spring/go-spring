# Go-Spring 实战第 8 课：配置编排与刷新：启动期 imports 和运行期 Dync 如何分工

Go-Spring 的配置系统走到这里，已经不只是“读取几个文件”了。统一模型、绑定、校验、来源、优先级和 Profile 都确定以后，配置还会遇到两个更接近真实服务的场景。

一个场景发生在启动期。基础配置可能需要导入本地覆盖文件、远程配置或开发者私有配置，配置值之间也可能需要互相引用。另一个场景发生在运行期。某些开关、阈值或超时时间希望在应用不重启的情况下读取最新值。

这两个场景不能混在一起理解。`spring.app.imports` 和变量引用解决启动期配置编排，`gs.Dync[T]` 和 `PropertiesRefresher` 解决运行期动态读取。它们仍然复用同一套 path 和绑定语法，因为底层配置模型没有换。

## imports 在启动期组合配置来源

`spring.app.imports` 可以把一个大配置拆成多个文件，也可以接入 Provider 提供的外部配置。

下面的配置演示了启动期导入多个来源。这里要先看 `spring.app.imports` 的值，Go-Spring 会在配置加载阶段把这些来源继续纳入同一套配置模型。

```properties
spring.app.imports=./dev.properties,http://config-server/app.properties
```

可选导入使用 `optional:` 前缀。

```properties
spring.app.imports=optional:./local.overrides
```

可选配置不存在时不会报错，适合本地覆盖文件、开发者私有配置或非必需的外部配置。

导入遵循后加载优先原则，即基础配置中导入的配置优先级高于基础配置本身；Profile 配置中导入的配置优先级高于对应 Profile 配置本身。

## 变量引用在同一 path 空间里拼装值

配置值可以引用其他配置项。

下面这些写法覆盖了引用已有配置、提供默认值、组合多个片段和读取环境变量几个常见场景。关键点是引用仍然在 Go-Spring 的配置 path 空间里查找。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

这样一来，就能支持几类常见用途。

- 抽取公共前缀。
- 组合多个配置项。
- 引用环境变量。
- 为引用值提供默认值。

变量引用的解析仍然基于同一套配置 path，所以它和绑定、优先级合并保持一致，不需要再引入额外的查找规则。

## 嵌套引用只适合少量拼装

Go-Spring 支持嵌套引用。

```properties
env=prod
config.file=config/${env}.properties
```

Go-Spring 会递归解析依赖，最终得到 `config/prod.properties`。

嵌套引用适合少量组合。不过，如果逻辑继续变复杂，Profile、条件注册或业务代码通常会是更清楚的表达位置。

## Dync 表达运行期需要重新读取的值

如果某个配置需要在运行期读取最新值，可以将字段声明为 `gs.Dync[T]`。

下面的结构体里，`Port` 是启动期绑定后的普通字段，后面几个字段是运行期可读取最新值的动态字段。这个对比很重要，因为 Go-Spring 不会把所有字段都变成动态配置。

```go
type AppConfig struct {
	Port int `value:"${server.port}"`

	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

使用时再调用 `Value()` 读取当前值。

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	timeout := a.Config.Timeout.Value()
	_ = timeout
}
```

`gs.Dync[T]` 是并发安全的，适合在多个 goroutine 中读取。

## PropertiesRefresher 只提交通过校验的动态字段

应用运行后，可以通过 `PropertiesRefresher` 触发刷新。

下面的例子演示刷新入口。`RefreshProperties()` 会重新读取并更新动态配置，普通字段仍然保持启动时绑定的值。

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

动态刷新只影响声明为 `gs.Dync[T]` 的字段。普通配置字段在启动绑定后保持不变，所以刷新不会悄悄改变已有对象的普通字段。

刷新会先预校验所有动态配置，保证原子提交，即要么全部更新成功，要么保持旧值不变，不会出现部分字段已经更新、部分字段失败的中间状态。这样做是为了避免运行期配置进入一种“半新半旧”的状态。

## 动态刷新不负责资源生命周期重建

动态配置适合开关、阈值、超时时间等轻量值。连接池这类资源的平滑切换通常还需要业务层配合，例如使用版本号触发资源重载，并逐步回收旧资源。

Go-Spring 配置系统提供的是动态值的读取与刷新语义，不直接替代资源生命周期管理。我们可以动态读取一个新的超时时间，但是否要重建客户端、如何迁移旧连接，仍然由资源管理逻辑决定。

## 配置系统的边界停在参数安全进入应用

至此，Go-Spring 配置系统已经从表达模型、绑定、复杂类型、校验、来源、优先级、Profile 走到编排和刷新。它解决的是应用如何从外部获得参数，并在启动期或运行期安全使用这些参数。

配置解决的是“参数怎样进入应用”。接下来，应用里的对象怎样创建、组合和管理，就交给 Go-Spring 的 IoC 容器继续回答。
