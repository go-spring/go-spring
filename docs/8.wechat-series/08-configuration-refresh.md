# Go-Spring 实战第 8 课 —— 配置编排与动态刷新：启动期 imports 和运行期 Dync 如何分工

上一篇我们讲了 Profile 多环境配置。基础配置负责公共部分，Profile 配置负责环境差异，多个 Profile 再按照声明顺序叠加。到这一步，配置文件已经不再是一份孤立的 `app.yaml`，而是一组可以组合的配置来源。

但真实服务还会继续往前走一步。启动时，应用可能需要把数据库、Redis、第三方客户端配置拆到不同文件里，也可能需要从配置中心拉取一段外部配置；运行时，某些开关、阈值和超时时间又希望在不重启进程的情况下读取新值。

这两个问题看起来都和“配置变化”有关，但 Go-Spring 把它们放在不同阶段处理。`spring.app.imports` 和变量引用解决启动期的配置编排，`gs.Dync[T]` 和 `PropertiesRefresher` 解决运行期的动态刷新。它们仍然共享同一套 `Properties`、`path`、来源优先级和绑定语义，只是生效时机不同。

## spring.app.imports

`spring.app.imports` 写在配置文件里，用来在当前配置加载过程中继续引入其他配置来源。它适合处理启动期就能确定的组合关系，比如把一个大配置拆成多个文件，或者通过已经注册的 Provider 接入外部配置。

下面这个例子演示本地文件拆分。主配置只保留应用入口信息，并把数据库和 Redis 配置拆出去。

```properties
spring.app.name=book-service
spring.app.imports=./conf/database.properties,./conf/redis.properties
```

这些被导入的配置进入 Go-Spring 以后，不会形成另一套配置空间。它们会继续落到同一个 `Properties` 模型里，然后按第 6 篇讲过的优先级和合并语义参与最终值计算。

如果某个导入来源不是本地文件，而是配置中心、对象存储或环境变量中的一段配置，就需要先通过 Provider 接入。第 5 篇里我们已经看过 Provider 的职责：它负责“从哪里读取配置”，读取之后仍然返回可以展开成 `Properties` 的数据。

```properties
spring.app.imports=envjson:APP_CONFIG
```

这条配置表示使用名为 `envjson` 的 Provider，从 `APP_CONFIG` 这个来源读取配置。Provider 名称、来源地址和可选标记都在这一个字符串里表达，因此 Go-Spring 在启动期就能把外部配置纳入同一轮加载。

## optional 导入

有些导入来源只在特定环境存在。比如开发者本地覆盖文件、临时调试配置，或者某个环境才会注入的外部配置。如果这些来源不存在就直接启动失败，反而会让基础配置难以复用。

Go-Spring 使用 `optional:` 表达这类可选导入。

```properties
spring.app.imports=optional:./conf/local.properties
```

当 `./conf/local.properties` 不存在时，这条导入会被忽略，应用可以继续启动。如果去掉 `optional:`，同样的缺失就会作为配置加载错误返回。

`optional:` 只改变“来源不存在时是否报错”这一件事。只要可选来源实际存在，它仍然会像普通导入一样参与配置合并，后续绑定、校验和动态字段初始化也不会因为它是可选来源而改变。

## 导入顺序

导入配置和声明它的配置文件处在同一类来源里。基础配置里的导入仍然属于基础配置层，Profile 配置里的导入仍然属于 Profile 配置层。也就是说，Profile 仍然高于基础配置，环境变量和命令行参数也仍然高于文件配置。

在同一层内部，Go-Spring 遵循后加载优先。配置文件会先被加入当前层，然后再加载它声明的 imports，因此被导入的配置可以覆盖声明文件里的同名 key。

下面这个例子里，主配置先给出默认端口，又导入一份覆盖文件。

```properties
# app.properties
server.port=8080
spring.app.imports=./conf/server-local.properties
```

```properties
# conf/server-local.properties
server.port=9000
```

最终 `server.port` 会取 `9000`。但如果命令行参数里又写了 `-Dserver.port=7000`，最终仍然是命令行参数生效，因为命令行参数处在更高优先级来源。

导入还需要保持简单。Go-Spring 当前只处理一层 imports，也就是只展开当前配置文件直接声明的导入；被导入的文件即使再声明 `spring.app.imports`，也不会继续递归展开。这个边界能避免启动期配置发现过程变成隐含的链式加载。

## 变量引用

imports 解决的是“配置从哪些来源组合进来”，变量引用解决的是“一个配置值如何复用另一个配置值”。Go-Spring 在绑定和解析配置路径时都会展开 `${...}`，引用目标仍然来自同一套配置空间。

下面的例子覆盖了几种常见写法。它们证明的不是字符串拼接能力，而是所有引用都回到统一的 `path` 查找规则里。

```properties
server.port=${port}
server.port=${port:=8080}
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api
redis.password=${REDIS_PASSWORD:=}
```

`${port}` 表示必须能在配置空间中找到 `port`。`${port:=8080}` 表示找不到时使用默认值。`app.home` 和 `app.url` 展示的是普通文本与引用值混合。`REDIS_PASSWORD` 则适合承接运行平台已经提供的原始环境变量。

变量引用本身不改变来源优先级。假设 `app.host` 同时出现在基础配置和环境变量里，那么 `${app.host}` 解析到的仍然是优先级合并之后的最终值。也就是说，引用发生在同一棵配置树上，而不是额外开辟一套变量系统。

## 递归引用

变量引用可以递归展开。这个能力适合少量配置片段的复用，尤其是默认值里还需要引用其他配置项的时候。

下面的例子表示：如果没有显式提供 `CONFIG_FILE`，就根据当前 `env` 拼出默认配置文件路径。

```properties
env=prod
config.file=${CONFIG_FILE:=config/${env}.properties}
```

当 `CONFIG_FILE` 不存在时，`config.file` 会解析成 `config/prod.properties`。如果平台注入了 `CONFIG_FILE`，则直接使用平台给出的路径。

递归引用要控制在可读范围内。如果一个配置值需要经过多层引用才能判断结果，排查成本会明显上升。此时更清楚的做法通常是把环境差异放到 Profile，把对象创建差异交给条件注册，把复杂判断留给业务代码。Go-Spring 会检测循环引用并返回错误，但循环检测只能阻止错误配置启动，不能替代清晰的配置建模。

## gs.Dync[T]

启动期编排结束后，普通字段会完成一次绑定。之后即使外部配置发生变化，普通字段也不会自动改变。Go-Spring 要求运行期动态值必须显式声明为 `gs.Dync[T]`，这样代码读起来就能区分哪些配置是启动稳定值，哪些配置允许刷新。

下面的结构体展示了这个分界。`Port` 是普通字段，绑定后保持启动时的值；另外几个字段使用 `gs.Dync[T]`，可以在刷新后读取新值。

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

## PropertiesRefresher

`PropertiesRefresher` 是 Go-Spring 注册到 IoC 容器里的内置 Bean，用来触发运行期配置刷新。调用 `RefreshProperties()` 时，Go-Spring 会重新加载配置来源，按既有优先级合并，再把结果应用到已经登记的动态字段上。

下面的例子演示一个刷新入口。它通过环境变量修改 `service.timeout`，然后触发刷新。

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

Go-Spring 还会在提交前预先绑定所有动态字段。只要任意一个字段无法完成类型转换、必填值缺失或校验失败，本轮刷新就会失败，旧值继续保留。这样可以避免应用进入“部分字段已经更新、部分字段仍然是旧值”的中间状态。

## 资源生命周期

动态刷新适合表达轻量值，例如开关、阈值、超时时间、采样率和限流参数。业务代码每次读取 `gs.Dync[T]`，就能拿到最近一次成功刷新提交的值。

但有些配置并不是单独改变一个值就能完成切换。连接池、客户端、消费者订阅和后台调度器通常都有自己的生命周期。配置里的地址或容量改变以后，是否要重建资源、什么时候切换、旧资源如何回收，都需要资源管理逻辑配合。

因此，Go-Spring 配置系统提供的是动态值刷新语义，而不是通用资源重建框架。比较稳妥的做法是把动态配置作为触发条件，例如配置里维护一个版本号或开关；业务层观察到版本变化后，再按自己的生命周期规则创建新资源、切换引用，并逐步释放旧资源。

## 配置编排与动态刷新

配置编排回答的是启动期问题：哪些配置来源要参与本次启动，它们如何合并，配置值之间怎样复用。`spring.app.imports`、Provider、Profile、变量引用最终都会回到同一套 `Properties` 和 `path` 语义里。

动态刷新回答的是运行期问题：哪些值允许在不重启应用的情况下更新，以及更新失败时如何保持旧值。`gs.Dync[T]` 把动态字段显式标出来，`PropertiesRefresher` 负责重新加载并提交通过校验的新值。

到这里，Go-Spring 配置系统的职责就收束了：它负责让外部参数以确定的规则进入应用，并在启动期或运行期安全地变成 Go 值。接下来，应用内部的对象如何创建、组合和管理，就进入 IoC 容器的范围。
