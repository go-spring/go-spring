# Go-Spring 实战第 8 课：配置导入、变量引用与动态刷新：启动期和运行期配置怎么处理

Go-Spring 的配置系统走到这里，已经不只是“读取几个文件”了。我们有了统一模型、绑定、校验、来源、优先级和 Profile，最后还要处理两个更高级的场景：启动期如何组合配置，运行期又如何读取最新配置。

Go-Spring 的高级配置能力主要集中在两个方向：

- 启动期配置编排：导入其他配置、引用变量、组合配置值。
- 运行期配置刷新：在不重启应用的情况下读取最新配置。

这两类能力仍然复用同一套 path 和绑定语法。这样一来，启动期配置和运行期配置就不会变成两套互不兼容的模型。

## 配置导入

`spring.app.imports` 可以把一个大配置拆成多个文件，也可以接入 Provider 提供的外部配置。

```properties
spring.app.imports=./dev.properties,http://config-server/app.properties
```

可选导入使用 `optional:` 前缀：

```properties
spring.app.imports=optional:./local.overrides
```

可选配置不存在时不会报错，适合本地覆盖文件、开发者私有配置或非必需的外部配置。

导入遵循后加载优先原则。也就是说，基础配置中导入的配置优先级高于基础配置本身；Profile 配置中导入的配置优先级高于对应 Profile 配置本身。

## 变量引用

配置值可以引用其他配置项：

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

这样一来，就能支持几类常见用途：

- 抽取公共前缀。
- 组合多个配置项。
- 引用环境变量。
- 为引用值提供默认值。

变量引用的解析仍然基于同一套配置 path，因此和绑定、优先级合并保持一致。我们不需要为变量再发明一套查找规则。

## 嵌套引用

Go-Spring 支持嵌套引用：

```properties
env=prod
config.file=config/${env}.properties
```

Go-Spring 会递归解析依赖，最终得到 `config/prod.properties`。

嵌套引用适合少量组合。如果逻辑继续变复杂，就不建议把复杂环境逻辑写进配置字符串。复杂逻辑应回到 Profile、条件注册或业务代码中表达。

## 动态配置

如果某个配置需要在运行期读取最新值，可以将字段声明为 `gs.Dync[T]`：

```go
type AppConfig struct {
	Port int `value:"${server.port}"`

	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

使用时再调用 `Value()` 读取当前值：

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	timeout := a.Config.Timeout.Value()
	_ = timeout
}
```

`gs.Dync[T]` 是并发安全的，适合在多个 goroutine 中读取。

## 运行期刷新

应用运行后，可以通过 `PropertiesRefresher` 触发刷新：

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

动态刷新只影响声明为 `gs.Dync[T]` 的字段。普通配置字段在启动绑定后保持不变，所以不会因为刷新而悄悄改变已有对象的普通字段。这个边界要记住。

刷新会先预校验所有动态配置，保证原子提交：要么全部更新成功，要么保持旧值不变，不会出现部分字段已经更新、部分字段失败的中间状态。

## 使用边界

动态配置适合开关、阈值、超时时间等轻量值。连接池这类资源的平滑切换通常还需要业务层配合，例如使用版本号触发资源重载，并逐步回收旧资源。

Go-Spring 配置系统提供的是动态值的读取与刷新语义，不直接替代资源生命周期管理。也就是说，我们可以动态读取一个新的超时时间，但是否要重建客户端、如何迁移旧连接，仍然应该由资源管理逻辑负责。

## 配置板块先收在这里

至此，Go-Spring 配置系统已经从表达模型、绑定、复杂类型、校验、来源、优先级、Profile 走到编排和刷新。它解决的是应用如何从外部获得参数，并在启动期或运行期安全使用这些参数。

接下来进入 IoC 容器，看看 Go-Spring 为什么要以 Bean 为中心来组织应用对象。
