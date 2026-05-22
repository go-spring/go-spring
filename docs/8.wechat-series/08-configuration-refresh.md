# Go-Spring 实战第 8 课 —— 变量引用与动态刷新：配置值如何复用和更新

上一篇我们讲了 Profile 多环境配置。然后我们知道了，基础配置负责公共部分，Profile 配置负责环境差异，这样组织之后，配置文件不会因为环境增加而被整份复制。

但真实项目里的配置还会继续出现两个问题。第一个问题发生在启动期：同一个地址、目录、端口或环境变量可能要被多个配置项复用，如果每个地方都手写一遍，后续修改很容易遗漏。第二个问题发生在运行期：开关、阈值、超时时间这类轻量参数，有时希望在不重启进程的情况下读取新值。

Go-Spring 把这两个问题放在不同阶段处理。变量引用解决启动期的配置复用，动态刷新解决运行期的配置更新。它们仍然共享同一套 `Properties`、`path`、来源优先级和绑定语义，只是生效时机不同。

## 变量引用

变量引用解决的是“一个配置值如何复用另一个配置值”。Go-Spring 在解析配置值时会展开 `${...}`，引用目标来自同一套配置空间，所以它不会额外引入一套独立的变量系统。

下面的例子覆盖了几种常见写法。它们证明的不是简单的字符串拼接能力，而是所有引用都会回到统一的 `path` 查找规则里。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}

env=prod
config.file=${CONFIG_FILE:=config/${env}.properties}
```

`${port}` 表示必须能在配置空间中找到 `port`，否则解析会失败。`${port:=8080}` 表示找不到 `port` 时使用默认值。`app.home` 和 `app.url` 展示的是普通文本与引用值混合。`REDIS_PASSWORD` 适合承接运行平台已经提供的原始环境变量。

最后两行展示的是递归引用：如果没有显式提供 `CONFIG_FILE`，`config.file` 会继续引用 `env`，最后得到 `config/prod.properties`。如果平台注入了 `CONFIG_FILE`，则直接使用平台给出的路径。

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
