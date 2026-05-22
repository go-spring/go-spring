# Go-Spring 实战第 8 课 —— 变量引用与动态刷新：配置值如何复用和更新

到这里，Go-Spring 配置系统的主线已经基本走完了。在前面几篇里，我们先用 `Properties` 和 `path` 建立了统一的配置模型，然后把配置绑定到了 Go 类型，随后处理了 `Duration`、`Time`、`Slice`、`Map` 等复杂类型，并使用 `expr` 标签拦截非法配置，再往后，我们把 Reader、Provider、环境变量和命令行参数等不同来源也都接入进来，然后又讨论了配置的优先级、合并语义以及 Profile 机制。

这一篇作为 Go-Spring 配置系列的最后一篇，咱们讨论最后两个比较重要的问题。第一个问题是同一个地址、目录、端口或环境变量可能要被多个配置项复用，如果每个地方都手写一遍，后续修改很容易遗漏。第二个问题是开关、阈值、超时时间这类轻量参数，有时希望在不重启进程的情况下能够读取新值。

在 Go-Spring 中，我们可以用变量引用来解决配置复用问题，用动态刷新来解决配置更新问题。

## 变量引用

Go-Spring 在解析配置值时会展开 `${...}` 格式的表达式（即变量引用），展开后会用对应的配置值进行替换。

下面的例子覆盖了几种常见的变量引用的写法。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

在上面的例子中，`${port}` 表示必须能在配置空间中找到 `port` 配置项，否则解析就会失败。`${port:=8080}` 表示找不到 `port` 时可以使用默认值，解析不会失败。`app.home` 和 `app.url` 展示的是普通文本与引用值混合。`REDIS_PASSWORD` 展示的是对环境变量的引用。

注意，变量引用本身不会改变来源的优先级，因为变量引用的解析是在完成所有配置合并之后才进行的。也就是说，如果 `app.host` 同时出现在基础配置、Profile 配置和环境变量里，那么 `${app.host}` 解析到的值仍然是优先级合并之后的最终值，而不是变量引用出现的那一层或者之前层的配置值。

另外，Go-Spring 目前不支持递归引用。而且我们也不推荐使用递归引用。

## 动态刷新

Go-Spring 支持在运行期刷新配置的值，只需要我们将动态字段声明为 `gs.Dync[T]` 类型即可，value 标签和 expr 标签和普通字段完全相同。`gs.Dync[T]` 的内部使用 atomic.Value 实现了并发安全的读写。

看个例子：

```go
type AppConfig struct {
	Port int `value:"${server.port}"`

	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

我们可以通过 `Value()` 方法读取当前的最新值。示例如下：

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	timeout := a.Config.Timeout.Value()
	_ = timeout
}
```

### PropertiesRefresher

配置动态刷新需要外部动作来触发。Go-Spring 要求在运行时调用 `gs.PropertiesRefresher` 的 `RefreshProperties()` 方法来触发刷新。

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

调用 `RefreshProperties()` 方法时，Go-Spring 会重新加载所有配置，按优先级进行合并，然后和旧配置进行 diff 找出差异项，然后找到差异项对应的动态字段，最后把新的值应用到动态字段上。

`RefreshProperties()` 执行时，首先对所有的差异项进行类型检查和 expr 表达式验证，如果有任何一个差异项验证失败，就会终止刷新。这样可以避免应用进入“部分字段已经更新、部分字段仍然是旧值”的混乱状态。

我们可以使用定时刷新或者推送刷新等不同方式来触发刷新。频繁的检查可能带来性能问题，所以建议在生产环境优化动态刷新的策略。

### 资源重建

对于开关、阈值、超时时间这类轻量参数，我们其实不用过多关心它们是什么时候被更新的。但是对于连接池等有自己生命周期的对象，我们需要关注它们的重建和回收。

通常来说，我们应当使用定期重建资源的策略，即为资源设置失效时间，在超过失效时间时资源就会被回收。这种策略可以平滑地切换资源，避免瞬间切换对新资源造成较大冲击。这种策略可以避免监听配置的变化，不仅减少了动态刷新实现的复杂性，也保障了资源切换时的稳定性。

## Go-Spring 配置体系

到这里，我们已经彻底掌握了 Go-Spring 配置系统的使用方法。接下来，咱们开始讨论 Go-Spring 的 IoC。