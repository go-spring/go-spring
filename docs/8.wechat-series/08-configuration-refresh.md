# Go-Spring 实战第 8 课 —— 变量引用与动态刷新：配置值如何复用和更新

到这里，Go-Spring 配置系统的主线已经基本走完了。在前面几篇里，我们先用 `Properties` 和 `path` 建立了统一的配置模型，然后把配置绑定到了 Go 类型，随后处理了 `Duration`、`Time`、`Slice`、`Map` 等复杂类型，并用 `expr` 标签拦截非法配置，再往后，我们把 Reader、Provider、环境变量和命令行参数等不同来源接入进来，然后又讨论了配置优先级、合并语义和 Profile 机制。

这一篇是 Go-Spring 配置系列的最后一篇，咱们来讨论两个收尾但很重要的问题。第一个问题是配置复用，同一个地址、目录、端口或密码，可能会被多个配置项引用，如果每个地方都手写一遍，后续修改很容易出现遗漏。第二个问题是运行期更新，开关、阈值、超时时间这类轻量参数，有时希望在不重启进程的情况下能够读到新值。

在 Go-Spring 中，配置复用可以通过变量引用解决，运行期更新可以通过动态刷新解决。

## 变量引用

Go-Spring 在解析配置值时，会展开 `${...}` 格式的表达式，也就是变量引用。展开时，Go-Spring 会找到对应的配置值，并把引用表达式替换成这个值。

下面的例子覆盖了几种常见的变量引用的写法。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

在上面的例子中，`${port}` 表示必须能在配置空间中找到 `port` 配置项，否则解析就会失败。`${port:=8080}` 表示找不到 `port` 时可以使用默认值，解析不会失败。`app.home` 和 `app.url` 展示的是普通文本与引用值的混合使用。`REDIS_PASSWORD` 展示的是对环境变量的引用。

需要注意的是，变量引用本身不会改变配置来源的优先级，因为变量引用是在所有配置完成合并之后才解析的。也就是说，如果 `app.host` 同时出现在基础配置、Profile 配置和环境变量里，那么 `${app.host}` 解析到的仍然是按优先级合并后的最终值，而不是变量引用所在来源或其之前来源中的值。

另外，Go-Spring 不支持递归引用，而且我们也不推荐在配置中设计递归引用。

## 动态刷新

Go-Spring 支持在运行期刷新配置的值。我们只需要把需要动态更新的字段声明为 `gs.Dync[T]` 类型即可，`value` 标签和 `expr` 标签的写法、语义都与普通字段相同。`gs.Dync[T]` 内部使用 `atomic.Value` 实现并发安全的读写。

看个例子：

```go
type AppConfig struct {
	Port int `value:"${server.port}"`

	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

读取动态配置时，我们需要通过 `Value()` 方法拿到最新的值：

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	timeout := a.Config.Timeout.Value()
	_ = timeout
}
```

### PropertiesRefresher

动态刷新需要由外部动作触发。Go-Spring 要求在运行时调用 `gs.PropertiesRefresher` 的 `RefreshProperties()` 方法来执行刷新。

`gs.PropertiesRefresher` 是 Go-Spring 注册到 IoC 容器里的内置 Bean，使用时直接注入即可。

看个例子：

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

调用 `RefreshProperties()` 方法时，Go-Spring 会重新加载所有配置，并按优先级完成合并，然后会与旧配置进行 diff，找出发生变化的配置项，然后会定位这些配置项对应的动态字段，最后把新值应用到动态字段上。

`RefreshProperties()` 在执行时，会首先对所有差异项进行类型检查和 `expr` 表达式验证。只要有任何一个差异项验证失败，整个刷新过程就会终止。这样可以避免应用进入“部分字段已经更新、部分字段仍然是旧值”的混乱状态。

我们可以使用定时刷新、推送刷新等不同方式来触发刷新。由于频繁检查可能带来额外开销，因此在生产环境中，需要根据配置来源和业务场景设计合适的刷新策略。

### 资源重建

对于开关、阈值、超时时间这类轻量参数，我们通常不需要过多关心它们具体在什么时候被更新。但是对于连接池等拥有独立生命周期的对象，就需要关注资源的重建和回收。

通常来说，更稳妥的做法是使用定期重建资源的策略，即为资源设置失效时间，超过失效时间后回收旧资源并创建新资源。这种方式可以让资源切换更加平滑，避免瞬间切换给新资源带来过大冲击。同时也不需要专门监听每一次配置变化，既降低了动态刷新实现的复杂度，也提升了资源切换时的稳定性。

## Go-Spring 配置体系

到这里，我们已经完整介绍了 Go-Spring 配置体系。但一个完整的应用还需要回答更多得问题，比如对象应该由谁创建，依赖关系应该怎样组织，初始化和销毁逻辑应该放在哪里，多个组件之间又应该如何协作。因此，接下来开始介绍 Go-Spring 的 IoC 部分，看看 Go-Spring 是如何管理对象、装配依赖，并把这些配置真正作用到应用组件上的。
