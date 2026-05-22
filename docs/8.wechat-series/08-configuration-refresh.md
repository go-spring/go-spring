# Go-Spring 实战第 8 课 —— 变量引用与动态刷新：配置值如何复用和更新

到这里，Go-Spring 配置系统的主线已经基本走完了。在前面几篇里，我们先用 `Properties` 和 `path` 建立了统一的配置模型，然后把配置绑定到了 Go 类型，随后处理了 `Duration`、`Time`、`Slice`、`Map` 等复杂类型，并使用 `expr` 标签拦截非法配置，再往后，我们把 Reader、Provider、环境变量和命令行参数等不同来源也都接入进来，然后又讨论了配置的优先级、合并语义以及 Profile 机制。

这一篇作为 Go-Spring 配置系列的最后一篇，咱们讨论最后两个比较重要的问题。第一个问题是同一个地址、目录、端口或环境变量可能要被多个配置项复用，如果每个地方都手写一遍，后续修改很容易遗漏。第二个问题是开关、阈值、超时时间这类轻量参数，有时希望在不重启进程的情况下能够读取新值。

在 Go-Spring 中，我们可以用变量引用来解决配置复用问题，用动态刷新来解决配置更新问题。

## 变量引用

变量引用解决的是“一个配置值如何复用另一个配置值”的问题。Go-Spring 在解析配置值时会展开 `${...}` 格式的表达式，展开后会用对应的配置值进行替换。

下面的例子覆盖了几种常见的变量引用的写法。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

在上面的例子中，`${port}` 表示必须能在配置空间中找到 `port` 配置项，否则解析就会失败。`${port:=8080}` 表示找不到 `port` 时可以使用默认值，解析不会失败。`app.home` 和 `app.url` 展示的是普通文本与引用值混合。`REDIS_PASSWORD` 展示的是对环境变量的引用。

变量引用本身不改变来源优先级。假设 `app.host` 同时出现在基础配置、Profile 配置和环境变量里，那么 `${app.host}` 解析到的仍然是优先级合并之后的最终值。也就是说，引用发生在同一棵配置树上，而不是绕过 Go-Spring 已经建立好的配置合并规则。

递归引用要控制在可读范围内。如果一个配置值需要经过多层引用才能判断结果，排查成本会明显上升。此时更清楚的做法通常是把环境差异放到 Profile，把对象创建差异交给条件注册，把复杂判断留给业务代码。Go-Spring 会检测循环引用并返回错误，但循环检测只能阻止错误配置启动，不能替代清晰的配置建模。

## 动态刷新

启动期配置绑定完成后，普通字段会保持启动时的值。之后即使外部配置发生变化，普通字段也不会自动改变。Go-Spring 要求运行期动态值必须显式声明为 `gs.Dync[T]`，这样代码读起来就能区分哪些配置是启动稳定值，哪些配置允许刷新。

下面的结构体展示了这个边界。`Port` 是普通字段，绑定后保持启动时的值；另外几个字段使用 `gs.Dync[T]`，可以在刷新后读取新值。

```go
type AppConfig struct {
	Port int `value:"${server.port}"`

	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

使用动态值时，需要通过 `Value()` 读取当前已提交的值。

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	timeout := a.Config.Timeout.Value()
	_ = timeout
}
```

`gs.Dync[T]` 的读取是并发安全的，适合在请求处理、后台任务或多个 goroutine 中读取。需要注意的是，`Value()` 只是读取当前值；外部配置发生变化以后，仍然需要一次刷新动作把新配置重新加载进来。

`PropertiesRefresher` 是 Go-Spring 注册到 IoC 容器里的内置 Bean，用来触发运行期配置刷新。调用 `RefreshProperties()` 时，Go-Spring 会重新加载配置来源，按既有优先级合并，再把结果应用到已经登记的动态字段上。

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

刷新只影响 `gs.Dync[T]` 字段。普通字段仍然保持启动时绑定的值，所以运行期刷新不会悄悄改写已经构造好的普通对象字段。

Go-Spring 会在提交前预先绑定所有动态字段。只要任意一个字段无法完成类型转换、必填值缺失或校验失败，本轮刷新就会失败，旧值继续保留。这样可以避免应用进入“部分字段已经更新、部分字段仍然是旧值”的中间状态。

动态刷新适合表达轻量值，例如开关、阈值、超时时间、采样率和限流参数。但连接池、客户端、消费者订阅和后台调度器通常有自己的生命周期。配置里的地址或容量改变以后，是否要重建资源、什么时候切换、旧资源如何回收，都需要业务层自己的资源管理逻辑配合。

所以，Go-Spring 配置系统提供的是动态值刷新语义，而不是通用资源重建框架。到这里，配置系统的职责就收束了：它负责让外部参数以确定的规则进入应用，并在启动期或运行期安全地变成 Go 值。接下来，应用内部的对象如何创建、组合和管理，就进入 IoC 容器的范围。
