# Go-Spring 实战第 8 课 —— 变量引用与动态刷新：配置值如何复用和更新

到这里，Go-Spring 配置系统的主线已经基本走完了。在前面几篇里，我们先用 `Properties` 和 `path` 建立了统一的配置模型，然后把配置绑定到了 Go 类型，接着处理了 `Duration`、`Time`、`Slice`、`Map` 等复杂类型，然后我们又用 `expr` 标签拦截非法配置，还把 Reader、Provider、环境变量和命令行参数等不同来源接入进来，最后讨论了配置优先级、合并语义和 Profile 机制。

这一篇是 Go-Spring 配置系列的最后一篇，咱们来讨论两个很重要的收尾问题。第一个问题是配置复用。同一个地址、目录、端口或密码可能会被多个配置项引用，如果每个地方都手写一遍，那后续修改就很容易出现遗漏。第二个问题是运行期更新。对于开关、阈值、超时时间这类轻量参数，我们有时希望在不重启进程的情况下也能够读到新值。

在 Go-Spring 中，配置复用可以通过变量引用解决，运行期更新可以通过动态刷新解决。

## 变量引用

Go-Spring 在解析配置值时，会展开 `${...}` 格式的表达式，也就是变量引用。展开时，Go-Spring 会找到对应的配置值，并把引用表达式替换成这个值。

下面的例子覆盖了几种常见的变量引用写法。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

在上面的例子中，`${port}` 表示必须能在配置空间中找到 `port` 配置项，否则解析就会失败。`${port:=8080}` 表示找不到 `port` 时可以使用默认值，解析不会失败。`app.home` 和 `app.url` 展示的是普通文本与引用值的混合使用。`REDIS_PASSWORD` 展示的是对环境变量的引用。

变量引用最常见的场景，是把一些基础值抽出来复用。比如服务地址、数据目录、外部系统域名这类配置，往往会被多个配置项组合使用。使用变量引用以后，基础值只需要维护一份，其他配置项通过引用来派生即可。

需要注意的是，变量引用本身不会改变配置来源的优先级，因为变量引用是在所有配置完成合并之后才解析的。也就是说，如果 `app.host` 同时出现在基础配置、Profile 配置和环境变量里，那么 `${app.host}` 解析到的仍然是按优先级合并后的最终值，而不是变量引用所在来源或其之前来源中的值。

另外，Go-Spring 不支持递归引用，也不推荐在配置中设计递归引用。

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

调用 `RefreshProperties()` 方法时，Go-Spring 会重新加载所有配置，并按优先级完成合并。接着，它会与旧配置进行 diff，找出发生变化的配置项，然后定位这些配置项对应的动态字段，最后把新值应用到动态字段上。

`RefreshProperties()` 在执行时，会首先对所有差异项进行类型检查和 `expr` 表达式验证。只要有任何一个差异项验证失败，整个刷新过程就会终止。这样可以避免应用进入“部分字段已经更新、部分字段仍然是旧值”的混乱状态。

我们可以使用定时刷新、推送刷新等不同方式来触发刷新。由于频繁检查可能带来额外开销，因此在生产环境中，需要根据配置来源和业务场景设计合适的刷新策略。

### 资源重建

对于开关、阈值、超时时间这类轻量参数，我们通常不需要过多关心它们具体在什么时候被更新。但是，对于数据库连接池、Redis 客户端、消息队列连接、HTTP 客户端等拥有独立生命周期的对象，就需要额外关注资源的重建和回收。因为配置值刷新了，并不意味着已经创建出来的资源对象也会自动变化。

比如连接池的地址、用户名、密码、最大连接数发生变化后，旧连接池里可能还有正在执行的请求，直接关闭的话会影响线上流量。如果立刻切到新连接池，也可能因为连接复用被打断、短时间重新建连而带来额外抖动。因此，资源重建不应该简单理解为“配置一变就马上销毁旧对象、创建新对象”。

更稳妥的做法，是把资源生命周期交给专门的资源管理逻辑处理。它可以在创建资源时读取当前配置，然后在关键配置确实发生变化或资源超过失效时间后，再创建新资源，同时让旧资源继续服务已经进入的请求，等请求自然结束后再回收。这样的话，既不需要监听每一次配置变化，也能让资源切换更加平滑。

## Go-Spring 配置体系

到这里，我们已经完整介绍了 Go-Spring 配置体系。但一个完整的应用还需要回答更多的问题，比如对象应该由谁创建，依赖关系应该怎样组织，初始化和销毁逻辑应该放在哪里，多个组件之间又应该如何协作。因此，接下来开始介绍 Go-Spring 的 IoC 部分，看看 Go-Spring 是如何管理对象、装配依赖，并把这些配置真正作用到应用组件上的。
