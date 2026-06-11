# Go-Spring 实战第 31 课 —— 能力地图：按应用生命周期理解配置、IoC、运行时和验证

前面几课把日志、HTTP、Starter、测试和 Mock 补齐后，Go-Spring 的主要能力已经逐步展开。单独看每一块时，我们很容易把它们记成一组 API：配置怎么读，Bean 怎么注册，Server 怎么启动，Mock 怎么写。

但真实项目里，问题通常不是“还缺哪个 API”，而是“这些能力为什么要放在同一套框架里”。如果只是几个函数调用，直接写 Go 代码往往更轻。只有当应用开始面对多环境配置、条件装配、组件复用、统一启动关闭和持续验证时，生命周期模型才有价值。

因此，Go-Spring 的能力地图应该按应用生命周期来理解：配置接住外部输入，IoC 组织运行对象，运行时协调启动和退出，日志与 HTTP 承担观测和服务入口，Starter 沉淀组件复用，测试与 Mock 持续验证这些装配、行为和外部边界。

## 外部输入

服务端应用首先面对的是外部差异。端口、数据库地址、功能开关、Profile、日志输出和客户端参数都会随环境变化。如果这些差异散落在业务代码里，后续装配和排查都会变困难。

Go-Spring 配置系统的抽象是 `Properties` 和 Path。无论输入来自文件、环境变量、命令行参数还是导入配置，最终都进入同一棵扁平化配置树，再由绑定、类型转换、表达式校验、优先级合并和动态刷新使用。

业务代码拿到的是结构化配置，而不是到处读取字符串 key。

```go
type ServerConfig struct {
	Addr         string        `value:"${spring.http.server.addr:=:9090}"`
	ReadTimeout  time.Duration `value:"${spring.http.server.readTimeout:=5s}"`
	WriteTimeout time.Duration `value:"${spring.http.server.writeTimeout:=5s}"`
}
```

这里的语义来自配置系统：path 精确匹配，默认值写在绑定标签中，`time.Duration` 由类型转换完成。配置系统解决的是“环境差异如何进入应用，并以可验证的结构被使用”。

所以，能力地图的第一层不是 IoC，也不是 HTTP，而是配置输入。没有统一输入，后面的条件装配、Starter 约定和测试配置都缺少共同语言。

## Bean 装配

有了配置输入，应用还需要把对象组织起来。服务、仓储、客户端、控制器和运行期组件之间存在依赖关系，如果全部手动拼装，条件选择、多实例和生命周期回调很快会混在启动代码里。

Go-Spring IoC 容器把这类问题收敛到 Bean 定义和装配规则。它支持字段注入、构造函数注入、按类型或名称选择、集合注入、配置驱动注入、生命周期回调、接口导出、条件注册和 Profile 条件。

构造函数仍然保持普通 Go 函数，装配选择放在注册阶段表达。

```go
func NewUserController(svc *UserService) *UserController {
	return &UserController{svc: svc}
}

func init() {
	gs.Provide(NewUserController)
}
```

如果同类型 Bean 有多个候选，注册语句可以用 `TagArg` 补充名称；如果参数来自配置，可以用 `${...}` 绑定；如果 Bean 只在某个配置存在时启用，可以挂 `Condition`。这些规则共同决定容器怎样解析装配关系。

IoC 容器解决的是“对象如何被创建、组合、选择和释放”。它不是为了隐藏 Go 代码，而是把装配语义集中起来，让启动期能够一次性发现缺失依赖、条件冲突或配置错误。

## 启动与退出链路

配置和对象关系准备好之后，应用还需要从进程入口进入可服务状态，并在退出时有序收束资源。这个阶段由 Go-Spring 应用运行时串起来。

启动阶段大致按这样的链路推进：加载配置，初始化日志，启动 IoC 容器，执行 Runner，启动 Server。退出阶段则取消 root context，停止 Server，关闭容器，执行 Destroy，并 flush 日志。

长生命周期服务不是普通 Bean 方法，而是进入应用运行时调度的 Server。

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

`Run` 负责启动服务并配合 Ready 信号，`Stop` 负责优雅停止。内置 HTTP Server 就是这个模型的一个实现：先监听端口，等待 Ready，再处理请求；退出时调用 `http.Server.Shutdown`。

运行时解决的是“应用怎样可靠启动和可靠退出”。它把配置、容器、Runner、Server 和日志放在同一条链路里，而不是让每个组件各自决定启动顺序。

## 服务入口与观测

应用进入运行态后，最常见的两个问题是对外提供服务和对内观察状态。Go-Spring 在这里提供的是接入点，而不是替代所有业务框架。

HTTP Server 基于标准库 `net/http`。应用可以继续使用 `http.DefaultServeMux`，也可以提供自定义 `*gs.HttpServeMux`，还可以把 Gin、gorilla/mux、chi 这类第三方路由器作为 `http.Handler` 交给 Go-Spring。Go-Spring 负责监听配置、Ready 和优雅关闭。

日志系统则负责结构化观测。它围绕日志级别、标签路由、Appender、Layout、Encoder、文件滚动、异步输出、上下文字段和动态刷新组织能力，让运行信息可检索、可治理。新的输出目标沿组件边界扩展，旧日志入口则通过适配接回同一条管线。

服务入口可以保持标准库模型，只把生命周期交给 Go-Spring。

```go
func init() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
}

func main() {
	gs.Run()
}
```

这段代码的语义是：路由仍由 `net/http` 处理，Go-Spring 负责应用级启动和退出。日志系统也是类似边界：业务代码表达日志事件，Go-Spring 日志能力负责输出结构、路由和刷新。

因此，服务入口与观测位于运行态，但它们不应该吞掉业务框架本身的职责。Go-Spring 负责把这些能力纳入统一生命周期。

## Starter 复用

当同一类组件在多个项目里反复接入时，单个项目里的注册代码就应该沉淀为 Starter。Starter 的主题不是“再封装一层 API”，而是稳定配置前缀、启用条件、默认 Bean 名、多实例规则和资源释放方式。

默认单实例和多实例可以共享一套组件约定。

```go
func init() {
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__")

	gs.Group("${spring.gorm.instances}", newClient, nil)
}
```

默认实例由 `spring.gorm.dsn` 触发，多实例来自 `spring.gorm.instances` 字典。`Provide`、`Module`、`Group`、`Condition`、`TagArg` 和 Destroy 回调仍然是 Go-Spring 原有能力，Starter 只是把这些能力组合成可复用包。

组件复用解决的是“跨项目接入如何保持一致”。如果某段初始化只服务于一个项目，直接写在应用里更清楚；如果配置命名、生命周期和多实例规则开始重复，Starter 才成为合适边界。

## 持续验证

能力越多，越需要测试把边界守住。Go-Spring 测试体系仍然基于 Go 原生 `go test`，但在需要验证容器装配时提供 `gs.RunTest`，在需要清晰失败输出时提供 `assert` 和 `require`，在需要隔离外部依赖时提供 `gs-mock`。

容器测试应该用于验证装配，而不是替代所有业务单测。

```go
func TestOrderFlow(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Property("app.env", "test")
	}).RunTest(t, func(s *struct {
		OrderSvc *OrderService `autowire:""`
		Env      string        `value:"${app.env}"`
	}) {
		assert.That(t, s.Env).Equal("test")
		assert.That(t, s.OrderSvc.Create(1001)).Nil()
	})
}
```

`RunTest` 创建测试 App，完成配置绑定和依赖注入，再执行断言。纯业务逻辑仍然应该留在普通单测里；外部系统或不稳定调用才用 Mock 隔离。

测试与 Mock 在能力地图里的位置，是持续验证配置、IoC、Starter 和运行时的组合是否仍然成立，并把外部系统、不稳定调用和难复现错误隔离出来。没有测试，生命周期模型只能依赖人工记忆；有了分层测试和清楚的 Mock 边界，框架能力才真正变成可维护的工程约束。

## 与 Wire 的边界

Go-Spring 和 Wire 都能减少手写装配代码，但它们面对的约束不同。Wire 把装配过程变成编译期代码生成，适合依赖关系稳定、希望尽量静态化的项目。Go-Spring 把装配放在启动期，换取配置驱动、条件注册、生命周期管理和 Starter 复用。

| 特性 | Go-Spring | Wire |
| --- | --- | --- |
| 处理时机 | 启动阶段 | 编译期代码生成 |
| 条件注册 | 支持 | 不支持 |
| 配置驱动 | 支持 | 不支持 |
| 生命周期管理 | 支持 | 不支持 |
| 运行路径 | 启动期反射，运行期普通调用 | 生成代码直接调用 |

如果系统需要极致静态约束，Wire 很合适。如果系统需要根据配置和环境裁剪 Bean 候选，需要统一启动关闭，或者希望把组件接入沉淀成 Starter，Go-Spring 的启动期模型会覆盖更多运行语义。

## 生命周期模型

把这些能力放回同一张图里，可以看到 Go-Spring 的边界。

| 阶段 | Go-Spring 能力 | 解决的问题 |
| --- | --- | --- |
| 启动前输入 | 配置系统 | 外部差异如何进入应用 |
| 启动期装配 | IoC 容器 | 对象如何创建、选择和组合 |
| 运行时调度 | App、Runner、Server | 应用如何启动、Ready 和退出 |
| 运行态能力 | 日志、HTTP Server | 服务入口和运行观测如何纳入生命周期 |
| 跨项目复用 | Starter | 组件接入约定如何复用 |
| 持续反馈 | 测试体系、gs-mock | 装配、行为和外部边界如何被验证 |

这也给出了使用 Go-Spring 的判断标准。如果项目只是少量函数和明确依赖，直接写 Go 代码更轻。如果项目需要多环境配置、条件装配、基础设施复用、统一启动关闭、结构化观测和可测试装配，那么把这些能力放进同一套生命周期模型，维护成本会更低。

Go-Spring 不是并排摆放的一组工具函数。它的能力地图围绕应用生命周期展开：配置提供输入，IoC 组织对象，运行时协调启动和退出，日志和 HTTP 接入运行态，Starter 复用组件规则，测试和 Mock 持续验证结果。理解这条链路，比记住单个 API 更重要。
